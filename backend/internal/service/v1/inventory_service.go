package v1service

import (
	"context"
	"fmt"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"sort"
	"time"

	"github.com/google/uuid"
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
	userRepo          repository.UserRepository
	txManager         db.TxManager
}

func NewInventoryService(
	productRepo repository.ProductRepository,
	stockMovementRepo repository.StockMovementRepository,
	userRepo repository.UserRepository,
	txManager db.TxManager,
) InventoryService {
	return &inventoryService{
		productRepo:       productRepo,
		stockMovementRepo: stockMovementRepo,
		userRepo:          userRepo,
		txManager:         txManager,
	}
}

func (s *inventoryService) LockAndDeductStock(ctx context.Context, items []DeductItem) (map[string]sqlc.GetByUUIDsForUpdateRow, error) {
	return s.LockAndDeductStockWithRef(ctx, items, "", "", nil, nil)
}

func (s *inventoryService) LockAndDeductStockWithRef(
	ctx context.Context,
	items []DeductItem,
	refID string,
	refType string,
	actorID *int32,
	note *string,
) (map[string]sqlc.GetByUUIDsForUpdateRow, error) {
	if len(items) == 0 {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "Item list cannot be empty")
	}

	sortedItems := make([]DeductItem, len(items))
	copy(sortedItems, items)
	sort.Slice(sortedItems, func(i, j int) bool {
		return sortedItems[i].ProductUUID < sortedItems[j].ProductUUID
	})

	uuids := make([]uuid.UUID, len(sortedItems))
	for i, item := range sortedItems {
		if item.Quantity <= 0 {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "Quantity must be greater than zero")
		}
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

	if len(products) != len(items) {
		return nil, utils.NewError(utils.ErrCodeNotFound, "One or more products not found")
	}

	productMap := make(map[string]sqlc.GetByUUIDsForUpdateRow, len(products))
	for _, p := range products {
		productMap[p.ProductUuid.String()] = p
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

		if product.ProductStatus != ProductStatusActive {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "Product is unavailable: "+item.ProductUUID)
		}

		currentStock := utils.Int32Value(product.ProductStock)
		if currentStock < item.Quantity {
			return nil, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Insufficient stock for product: %s (available: %d, requested: %d)", product.ProductName, currentStock, item.Quantity))
		}

		if err := s.productRepo.DeductStock(ctx, tx, product.ProductID, item.Quantity); err != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to deduct stock for product: "+item.ProductUUID, err)
		}

		// Record stock movement if repository is available
		if s.stockMovementRepo != nil {
			_, _ = s.stockMovementRepo.Create(ctx, tx, sqlc.CreateStockMovementParams{
				ProductID:      product.ProductID,
				ChangeType:     "ORDER_PLACED",
				QuantityChange: -item.Quantity,
				StockBefore:    currentStock,
				StockAfter:     currentStock - item.Quantity,
				ReferenceID:    defaultRefID,
				ReferenceType:  defaultRefType,
				ActorID:        actorID,
				Note:           note,
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
	// Sort items by ProductID to avoid deadlocks
	sortedItems := make([]RestoreItem, len(items))
	copy(sortedItems, items)
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
		if item.Quantity <= 0 {
			continue
		}

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
			_, _ = s.stockMovementRepo.Create(ctx, tx, sqlc.CreateStockMovementParams{
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
		}
	}
	return nil
}

func (s *inventoryService) AdjustStock(ctx context.Context, actorUUID uuid.UUID, input v1dto.AdjustStockInput) (sqlc.StockMovement, error) {
	var actorID *int32
	if s.userRepo != nil && actorUUID != uuid.Nil {
		actor, err := s.userRepo.GetByUUID(ctx, actorUUID)
		if err == nil {
			actorID = &actor.UserID
		}
	}

	productUUID, err := uuid.Parse(input.ProductUUID)
	if err != nil {
		return sqlc.StockMovement{}, utils.NewError(utils.ErrCodeBadRequest, "Invalid product UUID format")
	}

	product, err := s.productRepo.GetByUUID(ctx, productUUID)
	if err != nil {
		return sqlc.StockMovement{}, utils.NewError(utils.ErrCodeNotFound, "Product not found")
	}

	currentStock := utils.Int32Value(product.ProductStock)
	newStock := currentStock + input.QuantityChange
	if newStock < 0 {
		return sqlc.StockMovement{}, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Cannot adjust stock below zero (current: %d, adjustment: %d)", currentStock, input.QuantityChange))
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
			return sqlc.StockMovement{}, err
		}
	}

	return movement, nil
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
