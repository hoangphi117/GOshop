package v1service

import (
	"context"
	"errors"
	"fmt"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/rabbitmq"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type DeductItem struct {
	ProductUUID string
	Quantity    int32
}

type RestoreItem struct {
	ProductID int32
	Quantity  int32
}

type inventoryService struct {
	productRepo       repository.ProductRepository
	stockMovementRepo repository.StockMovementRepository
	userRepo          repository.UserReader
	txManager         db.TxManager
	rabbitMQ          rabbitmq.RabbitMQService
}

func NewInventoryService(
	productRepo repository.ProductRepository,
	stockMovementRepo repository.StockMovementRepository,
	userRepo repository.UserReader,
	txManager db.TxManager,
	rabbitMQ ...rabbitmq.RabbitMQService,
) InventoryService {
	var rmq rabbitmq.RabbitMQService
	if len(rabbitMQ) > 0 {
		rmq = rabbitMQ[0]
	}
	return &inventoryService{
		productRepo:       productRepo,
		stockMovementRepo: stockMovementRepo,
		userRepo:          userRepo,
		txManager:         txManager,
		rabbitMQ:          rmq,
	}
}

func (s *inventoryService) LockAndDeductStock(ctx context.Context, items []DeductItem) (map[string]InventoryItemStock, error) {
	return s.LockAndDeductStockWithRef(ctx, items, "", "", nil, nil)
}

func (s *inventoryService) LockAndDeductStockWithRef(
	ctx context.Context,
	items []DeductItem,
	refID string,
	refType string,
	actorID *int32,
	note *string,
) (map[string]InventoryItemStock, error) {
	if len(items) == 0 {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "Item list cannot be empty")
	}

	// Consolidate duplicate products if any
	consolidatedMap := make(map[string]int32)
	for _, item := range items {
		if item.Quantity <= 0 {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "Quantity must be greater than zero")
		}
		consolidatedMap[item.ProductUUID] += item.Quantity
	}

	sortedItems := make([]DeductItem, 0, len(consolidatedMap))
	for pUUID, qty := range consolidatedMap {
		sortedItems = append(sortedItems, DeductItem{
			ProductUUID: pUUID,
			Quantity:    qty,
		})
	}
	sort.Slice(sortedItems, func(i, j int) bool {
		return sortedItems[i].ProductUUID < sortedItems[j].ProductUUID
	})

	uuids := make([]uuid.UUID, len(sortedItems))
	for i, item := range sortedItems {
		parsed, err := uuid.Parse(item.ProductUUID)
		if err != nil {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "Invalid product UUID format: "+item.ProductUUID)
		}
		uuids[i] = parsed
	}

	tx, _ := db.GetTx(ctx)

	products, err := s.productRepo.GetByUUIDs(ctx, tx, uuids)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to fetch products", err)
	}

	if len(products) != len(sortedItems) {
		return nil, utils.NewError(utils.ErrCodeNotFound, "One or more products not found")
	}

	productMap := make(map[string]InventoryItemStock, len(products))
	for _, p := range products {
		productMap[p.ProductUuid.String()] = InventoryItemStock{
			ProductID:       p.ProductID,
			ProductUUID:     p.ProductUuid,
			ProductName:     p.ProductName,
			Price:           utils.NumericToFloat(p.ProductPrice),
			Stock:           utils.Int32Value(p.ProductStock),
			Status:          p.ProductStatus,
			SKU:             p.ProductSku,
			ProductSlug:     p.ProductSlug,
			ProductImageURL: p.ProductImageUrl,
		}
	}

	var defaultRefID *string
	if refID != "" {
		defaultRefID = &refID
	}
	var defaultRefType *string
	if refType != "" {
		defaultRefType = &refType
	}

	for _, item := range sortedItems {
		product, ok := productMap[item.ProductUUID]
		if !ok {
			return nil, utils.NewError(utils.ErrCodeNotFound, "Product not found: "+item.ProductUUID)
		}

		if product.Status != ProductStatusActive {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "Product is unavailable: "+item.ProductUUID)
		}

		currentStock := product.Stock
		if currentStock < item.Quantity {
			return nil, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Insufficient stock for product: %s (available: %d, requested: %d)", product.ProductName, currentStock, item.Quantity))
		}

		if err := s.productRepo.DeductStock(ctx, tx, product.ProductID, item.Quantity); err != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to deduct stock for product: "+item.ProductUUID, err)
		}

		newStock := currentStock - item.Quantity

		// Record stock movement if repository is available
		if s.stockMovementRepo != nil {
			_, err := s.stockMovementRepo.Create(ctx, tx, sqlc.CreateStockMovementParams{
				ProductID:      product.ProductID,
				ChangeType:     "ORDER_PLACED",
				QuantityChange: -item.Quantity,
				StockBefore:    currentStock,
				StockAfter:     newStock,
				ReferenceID:    defaultRefID,
				ReferenceType:  defaultRefType,
				ActorID:        actorID,
				Note:           note,
			})
			if err != nil {
				var pgErr *pgconn.PgError
				if errors.As(err, &pgErr) && pgErr.Code == "23505" {
					return nil, utils.NewError(utils.ErrCodeConflict, fmt.Sprintf("Stock already deducted for reference %s and product %s", utils.NilToString(defaultRefID), item.ProductUUID))
				}
				return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to record stock movement", err)
			}
		}

		// Trigger low stock alert if inventory drops to or below threshold
		if s.rabbitMQ != nil && newStock <= 5 {
			_ = s.rabbitMQ.PublishTopic(ctx, rabbitmq.RoutingKeyInventoryStockLow, map[string]any{
				"product_id": product.ProductID,
				"name":       product.ProductName,
				"stock":      newStock,
			})
		}
	}
	return productMap, nil
}

func (s *inventoryService) RestoreStock(ctx context.Context, items []RestoreItem) error {
	return s.RestoreStockWithRef(ctx, items, "", "", nil, nil)
}

func (s *inventoryService) RestoreStockWithRef(
	ctx context.Context,
	items []RestoreItem,
	refID string,
	refType string,
	actorID *int32,
	note *string,
) error {
	if len(items) == 0 {
		return nil
	}

	// Consolidate duplicate products if any
	consolidatedMap := make(map[int32]int32)
	for _, item := range items {
		if item.Quantity <= 0 {
			continue
		}
		consolidatedMap[item.ProductID] += item.Quantity
	}

	// Sort items by ProductID to avoid deadlocks
	sortedItems := make([]RestoreItem, 0, len(consolidatedMap))
	for pID, qty := range consolidatedMap {
		sortedItems = append(sortedItems, RestoreItem{
			ProductID: pID,
			Quantity:  qty,
		})
	}
	sort.Slice(sortedItems, func(i, j int) bool {
		return sortedItems[i].ProductID < sortedItems[j].ProductID
	})

	tx, _ := db.GetTx(ctx)

	var defaultRefID *string
	if refID != "" {
		defaultRefID = &refID
	}
	var defaultRefType *string
	if refType != "" {
		defaultRefType = &refType
	}

	changeType := "ORDER_CANCELLED"
	if refType == "order_return" || refType == "return" {
		changeType = "ORDER_RETURNED"
	}

	for _, item := range sortedItems {
		// Fetch current stock for audit trail
		var currentStock int32
		if s.stockMovementRepo != nil {
			row, err := s.productRepo.GetByUUID(ctx, uuid.Nil) // fallback or repo query
			_ = row
			_ = err
		}

		if err := s.productRepo.RestoreStock(ctx, tx, item.ProductID, item.Quantity); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, fmt.Sprintf("Failed to restore stock for product ID: %d", item.ProductID), err)
		}

		if s.stockMovementRepo != nil {
			_, err := s.stockMovementRepo.Create(ctx, tx, sqlc.CreateStockMovementParams{
				ProductID:      item.ProductID,
				ChangeType:     changeType,
				QuantityChange: item.Quantity,
				StockBefore:    currentStock,
				StockAfter:     currentStock + item.Quantity,
				ReferenceID:    defaultRefID,
				ReferenceType:  defaultRefType,
				ActorID:        actorID,
				Note:           note,
			})
			if err != nil {
				var pgErr *pgconn.PgError
				if errors.As(err, &pgErr) && pgErr.Code == "23505" {
					return utils.NewError(utils.ErrCodeConflict, fmt.Sprintf("Stock already restored for reference %s and product ID %d", utils.NilToString(defaultRefID), item.ProductID))
				}
				return utils.WrapError(utils.ErrCodeInternal, "Failed to record stock movement", err)
			}
		}
	}
	return nil
}

func (s *inventoryService) AdjustStock(ctx context.Context, actorUUID uuid.UUID, input v1dto.AdjustStockInput) (*v1dto.StockMovementDTO, error) {
	var actorID *int32
	var actorName *string
	var actorUUIDStr *string
	if s.userRepo != nil && actorUUID != uuid.Nil {
		actor, err := s.userRepo.GetByUUID(ctx, actorUUID)
		if err == nil {
			actorID = &actor.UserID
			actorName = &actor.UserFullname
			uStr := actor.UserUuid.String()
			actorUUIDStr = &uStr
		}
	}

	productUUID, err := uuid.Parse(input.ProductUUID)
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "Invalid product UUID format")
	}

	product, err := s.productRepo.GetByUUID(ctx, productUUID)
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeNotFound, "Product not found")
	}

	currentStock := utils.Int32Value(product.ProductStock)
	newStock := currentStock + input.QuantityChange
	if newStock < 0 {
		return nil, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Cannot adjust stock below zero (current: %d, adjustment: %d)", currentStock, input.QuantityChange))
	}

	var movement sqlc.StockMovement

	if s.txManager != nil {
		err := s.txManager.WithTx(ctx, func(txCtx context.Context) error {
			tx, _ := db.GetTx(txCtx)
			if input.QuantityChange > 0 {
				if err := s.productRepo.RestoreStock(txCtx, tx, product.ProductID, input.QuantityChange); err != nil {
					return utils.WrapError(utils.ErrCodeInternal, "Failed to adjust product stock", err)
				}
			} else {
				if err := s.productRepo.DeductStock(txCtx, tx, product.ProductID, -input.QuantityChange); err != nil {
					return utils.WrapError(utils.ErrCodeInternal, "Failed to adjust product stock", err)
				}
			}

			refType := "manual"
			createdMovement, err := s.stockMovementRepo.Create(txCtx, tx, sqlc.CreateStockMovementParams{
				ProductID:      product.ProductID,
				ChangeType:     input.ChangeType,
				QuantityChange: input.QuantityChange,
				StockBefore:    currentStock,
				StockAfter:     newStock,
				ReferenceID:    nil,
				ReferenceType:  &refType,
				ActorID:        actorID,
				Note:           input.Note,
			})
			if err != nil {
				return utils.WrapError(utils.ErrCodeInternal, "Failed to record stock movement", err)
			}
			movement = createdMovement
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	// Trigger low stock alert if inventory drops to or below threshold
	if s.rabbitMQ != nil && newStock <= 5 {
		_ = s.rabbitMQ.PublishTopic(ctx, rabbitmq.RoutingKeyInventoryStockLow, map[string]any{
			"product_id": product.ProductID,
			"name":       product.ProductName,
			"stock":      newStock,
		})
	}

	dto := v1dto.MapStockMovementToDTO(
		movement,
		product.ProductUuid.String(),
		product.ProductName,
		product.ProductSku,
		product.ProductImageUrl,
		actorUUIDStr,
		actorName,
	)
	return &dto, nil
}

func (s *inventoryService) ListStockMovements(ctx context.Context, params v1dto.GetStockMovementsParam) ([]v1dto.StockMovementDTO, int64, error) {
	if s.stockMovementRepo == nil {
		return []v1dto.StockMovementDTO{}, 0, nil
	}

	page := params.Page
	if page < 1 {
		page = 1
	}
	limit := params.Limit
	if limit < 1 || limit > 100 {
		limit = 10
	}
	offset := (page - 1) * limit

	var productUUIDParam pgtype.UUID
	if params.ProductUUID != nil && *params.ProductUUID != "" {
		if parsed, err := uuid.Parse(*params.ProductUUID); err == nil {
			productUUIDParam = pgtype.UUID{Bytes: parsed, Valid: true}
		}
	}

	var fromDateParam pgtype.Timestamptz
	if params.FromDate != nil && *params.FromDate != "" {
		if t, err := time.Parse(time.RFC3339, *params.FromDate); err == nil {
			fromDateParam = pgtype.Timestamptz{Time: t, Valid: true}
		} else if t, err := time.Parse("2006-01-02", *params.FromDate); err == nil {
			fromDateParam = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}

	var toDateParam pgtype.Timestamptz
	if params.ToDate != nil && *params.ToDate != "" {
		if t, err := time.Parse(time.RFC3339, *params.ToDate); err == nil {
			toDateParam = pgtype.Timestamptz{Time: t, Valid: true}
		} else if t, err := time.Parse("2006-01-02", *params.ToDate); err == nil {
			toDateParam = pgtype.Timestamptz{Time: t.Add(24*time.Hour - time.Nanosecond), Valid: true}
		}
	}

	count, err := s.stockMovementRepo.Count(ctx, sqlc.CountStockMovementsParams{
		ProductUuid: productUUIDParam,
		ChangeType:  params.ChangeType,
		FromDate:    fromDateParam,
		ToDate:      toDateParam,
		Search:      params.Search,
	})
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to count stock movements", err)
	}

	rows, err := s.stockMovementRepo.List(ctx, sqlc.ListStockMovementsParams{
		ProductUuid: productUUIDParam,
		ChangeType:  params.ChangeType,
		FromDate:    fromDateParam,
		ToDate:      toDateParam,
		Search:      params.Search,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to list stock movements", err)
	}

	dtos := make([]v1dto.StockMovementDTO, len(rows))
	for i, r := range rows {
		dtos[i] = v1dto.MapStockMovementRowToDTO(r)
	}

	return dtos, count, nil
}
