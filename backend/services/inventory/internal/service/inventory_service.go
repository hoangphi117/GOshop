package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"goshop/internal/utils"
	"goshop/pkg/logger"
	"goshop/pkg/rabbitmq"
	"goshop/services/inventory/internal/db/sqlc"
	"goshop/services/inventory/internal/dto"
	"goshop/services/inventory/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type InventoryService interface {
	LockAndDeductStockWithRef(ctx context.Context, items []dto.DeductItem, refID string, refType string, actorID *int32, actorUUID *string, note *string) (map[string]dto.ItemStockDTO, error)
	RestoreStockWithRef(ctx context.Context, items []dto.RestoreItem, refID string, refType string, actorID *int32, actorUUID *string, note *string) error
	GetStock(ctx context.Context, productUUIDs []string, productIDs []int32) ([]dto.ItemStockDTO, error)
	AdjustStock(ctx context.Context, actorUUID *uuid.UUID, actorName *string, input dto.AdjustStockInput) (*dto.StockMovementDTO, error)
	ListStockMovements(ctx context.Context, params dto.GetStockMovementsParam) ([]dto.StockMovementDTO, int64, error)
}

type inventoryService struct {
	repo     repository.InventoryRepository
	dbPool   *pgxpool.Pool
	rabbitMQ rabbitmq.RabbitMQService
}

func NewInventoryService(repo repository.InventoryRepository, dbPool *pgxpool.Pool, rmq ...rabbitmq.RabbitMQService) InventoryService {
	var r rabbitmq.RabbitMQService
	if len(rmq) > 0 {
		r = rmq[0]
	}
	return &inventoryService{
		repo:     repo,
		dbPool:   dbPool,
		rabbitMQ: r,
	}
}

func toPgUUID(u *uuid.UUID) pgtype.UUID {
	if u == nil || *u == uuid.Nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: *u, Valid: true}
}

func fromPgUUID(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	parsed, err := uuid.FromBytes(u.Bytes[:])
	if err != nil {
		return nil
	}
	str := parsed.String()
	return &str
}

func (s *inventoryService) LockAndDeductStockWithRef(
	ctx context.Context,
	items []dto.DeductItem,
	refID string,
	refType string,
	actorID *int32,
	actorUUID *string,
	note *string,
) (map[string]dto.ItemStockDTO, error) {
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

	sortedItems := make([]dto.DeductItem, 0, len(consolidatedMap))
	for pUUID, qty := range consolidatedMap {
		sortedItems = append(sortedItems, dto.DeductItem{
			ProductUUID: pUUID,
			Quantity:    qty,
		})
	}
	// Sort by ProductUUID to prevent deadlocks
	sort.Slice(sortedItems, func(i, j int) bool {
		return sortedItems[i].ProductUUID < sortedItems[j].ProductUUID
	})

	var defaultRefID *string
	if refID != "" {
		defaultRefID = &refID
	}
	var defaultRefType *string
	if refType != "" {
		defaultRefType = &refType
	}

	var parsedActorUUID *uuid.UUID
	if actorUUID != nil && *actorUUID != "" {
		if u, err := uuid.Parse(*actorUUID); err == nil {
			parsedActorUUID = &u
		}
	}

	resultMap := make(map[string]dto.ItemStockDTO, len(sortedItems))

	tx, err := s.dbPool.Begin(ctx)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to start database transaction", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	for _, item := range sortedItems {
		parsedUUID, err := uuid.Parse(item.ProductUUID)
		if err != nil {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "Invalid product UUID: "+item.ProductUUID)
		}

		// Idempotency check: if reference already recorded, skip deduction
		if refID != "" && refType != "" {
			existingMovement, err := s.repo.GetMovementByRefAndProductUUID(ctx, refType, refID, parsedUUID)
			if err == nil && existingMovement.MovementID > 0 {
				logger.Log.Info().Msgf("Idempotent stock deduction hit for ref %s product %s", refID, item.ProductUUID)
				stockRow, err := s.repo.GetStockByUUID(ctx, parsedUUID)
				if err == nil {
					pID := int32(0)
					if stockRow.ProductID != nil {
						pID = *stockRow.ProductID
					}
					resultMap[item.ProductUUID] = dto.ItemStockDTO{
						ProductID:         pID,
						ProductUUID:       stockRow.ProductUuid.String(),
						ProductName:       stockRow.ProductName,
						ProductSKU:        stockRow.ProductSku,
						ProductImageURL:   stockRow.ProductImageUrl,
						Quantity:          stockRow.Quantity,
						ReservedQuantity:  stockRow.ReservedQuantity,
						AvailableQuantity: stockRow.Quantity - stockRow.ReservedQuantity,
					}
				}
				continue
			}
		}

		// Check current stock first to capture stockBefore
		currentStockRow, err := s.repo.GetStockByUUID(ctx, parsedUUID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, utils.NewError(utils.ErrCodeNotFound, "Product stock not found: "+item.ProductUUID)
			}
			return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to check product stock", err)
		}

		if currentStockRow.Quantity < item.Quantity {
			return nil, utils.NewError(
				utils.ErrCodeBadRequest,
				fmt.Sprintf("Insufficient stock for product %s (available: %d, requested: %d)",
					currentStockRow.ProductName, currentStockRow.Quantity, item.Quantity),
			)
		}

		// Atomic decrement
		updatedStockRow, err := s.repo.DeductStockByUUID(ctx, tx, parsedUUID, item.Quantity)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, utils.NewError(utils.ErrCodeBadRequest, "Stock was modified concurrently, insufficient stock for product: "+item.ProductUUID)
			}
			return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to deduct stock for product: "+item.ProductUUID, err)
		}

		// Record stock movement
		_, err = s.repo.CreateStockMovement(ctx, tx, sqlc.CreateStockMovementParams{
			ProductID:      currentStockRow.ProductID,
			ProductUuid:    parsedUUID,
			ChangeType:     "ORDER_PLACED",
			QuantityChange: -item.Quantity,
			StockBefore:    currentStockRow.Quantity,
			StockAfter:     updatedStockRow.Quantity,
			ReferenceID:    defaultRefID,
			ReferenceType:  defaultRefType,
			ActorID:        actorID,
			ActorUuid:      toPgUUID(parsedActorUUID),
			ActorName:      nil,
			Note:           note,
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return nil, utils.NewError(utils.ErrCodeConflict, fmt.Sprintf("Stock already deducted for reference %s and product %s", refID, item.ProductUUID))
			}
			return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to record stock movement", err)
		}

		pID := int32(0)
		if updatedStockRow.ProductID != nil {
			pID = *updatedStockRow.ProductID
		}
		resultMap[item.ProductUUID] = dto.ItemStockDTO{
			ProductID:         pID,
			ProductUUID:       updatedStockRow.ProductUuid.String(),
			ProductName:       updatedStockRow.ProductName,
			ProductSKU:        updatedStockRow.ProductSku,
			ProductImageURL:   updatedStockRow.ProductImageUrl,
			Quantity:          updatedStockRow.Quantity,
			ReservedQuantity:  updatedStockRow.ReservedQuantity,
			AvailableQuantity: updatedStockRow.Quantity - updatedStockRow.ReservedQuantity,
		}

		// Low stock alert via RabbitMQ
		if s.rabbitMQ != nil && updatedStockRow.Quantity <= 5 {
			_ = s.rabbitMQ.PublishTopic(ctx, rabbitmq.RoutingKeyInventoryStockLow, map[string]any{
				"product_id":   pID,
				"product_uuid": updatedStockRow.ProductUuid.String(),
				"name":         updatedStockRow.ProductName,
				"stock":        updatedStockRow.Quantity,
			})
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to commit stock deduction", err)
	}

	return resultMap, nil
}

func (s *inventoryService) RestoreStockWithRef(
	ctx context.Context,
	items []dto.RestoreItem,
	refID string,
	refType string,
	actorID *int32,
	actorUUID *string,
	note *string,
) error {
	if len(items) == 0 {
		return nil
	}

	// Consolidate duplicates
	consolidatedUUIDMap := make(map[string]int32)
	consolidatedIDMap := make(map[int32]int32)

	for _, item := range items {
		if item.Quantity <= 0 {
			continue
		}
		if item.ProductUUID != "" {
			consolidatedUUIDMap[item.ProductUUID] += item.Quantity
		} else if item.ProductID > 0 {
			consolidatedIDMap[item.ProductID] += item.Quantity
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

	var parsedActorUUID *uuid.UUID
	if actorUUID != nil && *actorUUID != "" {
		if u, err := uuid.Parse(*actorUUID); err == nil {
			parsedActorUUID = &u
		}
	}

	changeType := "ORDER_CANCELLED"
	if refType == "order_return" || refType == "return" {
		changeType = "ORDER_RETURNED"
	}

	tx, err := s.dbPool.Begin(ctx)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to begin transaction", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// 1. Process items with ProductUUID
	for pUUIDStr, qty := range consolidatedUUIDMap {
		parsedUUID, err := uuid.Parse(pUUIDStr)
		if err != nil {
			continue
		}

		// Idempotency check
		if refID != "" && refType != "" {
			existing, err := s.repo.GetMovementByRefAndProductUUID(ctx, refType, refID, parsedUUID)
			if err == nil && existing.MovementID > 0 {
				logger.Log.Info().Msgf("Idempotent restore hit for ref %s product %s", refID, pUUIDStr)
				continue
			}
		}

		currentStock, err := s.repo.GetStockByUUID(ctx, parsedUUID)
		if err != nil {
			logger.Log.Warn().Err(err).Msgf("Could not find product %s to restore stock", pUUIDStr)
			continue
		}

		updatedStock, err := s.repo.RestoreStockByUUID(ctx, tx, parsedUUID, qty)
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to restore stock for product: "+pUUIDStr, err)
		}

		_, err = s.repo.CreateStockMovement(ctx, tx, sqlc.CreateStockMovementParams{
			ProductID:      currentStock.ProductID,
			ProductUuid:    parsedUUID,
			ChangeType:     changeType,
			QuantityChange: qty,
			StockBefore:    currentStock.Quantity,
			StockAfter:     updatedStock.Quantity,
			ReferenceID:    defaultRefID,
			ReferenceType:  defaultRefType,
			ActorID:        actorID,
			ActorUuid:      toPgUUID(parsedActorUUID),
			ActorName:      nil,
			Note:           note,
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				// Idempotency constraint hit
				continue
			}
			return utils.WrapError(utils.ErrCodeInternal, "Failed to record restoration movement", err)
		}
	}

	// 2. Process items with ProductID
	for pID, qty := range consolidatedIDMap {
		// Idempotency check
		if refID != "" && refType != "" {
			existing, err := s.repo.GetMovementByRefAndProductID(ctx, refType, refID, pID)
			if err == nil && existing.MovementID > 0 {
				continue
			}
		}

		currentStock, err := s.repo.GetStockByProductID(ctx, pID)
		if err != nil {
			logger.Log.Warn().Err(err).Msgf("Could not find product ID %d to restore stock", pID)
			continue
		}

		updatedStock, err := s.repo.RestoreStockByID(ctx, tx, pID, qty)
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, fmt.Sprintf("Failed to restore stock for product ID: %d", pID), err)
		}

		_, err = s.repo.CreateStockMovement(ctx, tx, sqlc.CreateStockMovementParams{
			ProductID:      &pID,
			ProductUuid:    currentStock.ProductUuid,
			ChangeType:     changeType,
			QuantityChange: qty,
			StockBefore:    currentStock.Quantity,
			StockAfter:     updatedStock.Quantity,
			ReferenceID:    defaultRefID,
			ReferenceType:  defaultRefType,
			ActorID:        actorID,
			ActorUuid:      toPgUUID(parsedActorUUID),
			ActorName:      nil,
			Note:           note,
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				continue
			}
			return utils.WrapError(utils.ErrCodeInternal, "Failed to record restoration movement", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to commit stock restoration", err)
	}

	return nil
}

func (s *inventoryService) GetStock(ctx context.Context, productUUIDs []string, productIDs []int32) ([]dto.ItemStockDTO, error) {
	result := make([]dto.ItemStockDTO, 0)
	visited := make(map[string]bool)

	if len(productUUIDs) > 0 {
		uuids := make([]uuid.UUID, 0, len(productUUIDs))
		for _, uStr := range productUUIDs {
			if u, err := uuid.Parse(uStr); err == nil {
				uuids = append(uuids, u)
			}
		}
		if len(uuids) > 0 {
			rows, err := s.repo.GetStockByUUIDs(ctx, uuids)
			if err != nil {
				return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to get stock by UUIDs", err)
			}
			for _, r := range rows {
				pID := int32(0)
				if r.ProductID != nil {
					pID = *r.ProductID
				}
				visited[r.ProductUuid.String()] = true
				result = append(result, dto.ItemStockDTO{
					ProductID:         pID,
					ProductUUID:       r.ProductUuid.String(),
					ProductName:       r.ProductName,
					ProductSKU:        r.ProductSku,
					ProductImageURL:   r.ProductImageUrl,
					Quantity:          r.Quantity,
					ReservedQuantity:  r.ReservedQuantity,
					AvailableQuantity: r.Quantity - r.ReservedQuantity,
				})
			}
		}
	}

	if len(productIDs) > 0 {
		rows, err := s.repo.GetStockByProductIDs(ctx, productIDs)
		if err != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to get stock by product IDs", err)
		}
		for _, r := range rows {
			if visited[r.ProductUuid.String()] {
				continue
			}
			pID := int32(0)
			if r.ProductID != nil {
				pID = *r.ProductID
			}
			result = append(result, dto.ItemStockDTO{
				ProductID:         pID,
				ProductUUID:       r.ProductUuid.String(),
				ProductName:       r.ProductName,
				ProductSKU:        r.ProductSku,
				ProductImageURL:   r.ProductImageUrl,
				Quantity:          r.Quantity,
				ReservedQuantity:  r.ReservedQuantity,
				AvailableQuantity: r.Quantity - r.ReservedQuantity,
			})
		}
	}

	return result, nil
}

func (s *inventoryService) AdjustStock(
	ctx context.Context,
	actorUUID *uuid.UUID,
	actorName *string,
	input dto.AdjustStockInput,
) (*dto.StockMovementDTO, error) {
	productUUID, err := uuid.Parse(input.ProductUUID)
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "Invalid product UUID format")
	}

	tx, err := s.dbPool.Begin(ctx)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to start transaction", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	stockRow, err := s.repo.GetStockByUUID(ctx, productUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			createdRow, createErr := s.repo.UpsertStockMetadata(ctx, tx, sqlc.UpsertStockMetadataParams{
				ProductUuid: productUUID,
				Quantity:    0,
			})
			if createErr != nil {
				return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to initialize product stock row", createErr)
			}
			stockRow = createdRow
		} else {
			return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to fetch stock", err)
		}
	}

	currentStock := stockRow.Quantity
	newStock := currentStock + input.QuantityChange
	if newStock < 0 {
		return nil, utils.NewError(
			utils.ErrCodeBadRequest,
			fmt.Sprintf("Cannot adjust stock below zero (current: %d, adjustment: %d)", currentStock, input.QuantityChange),
		)
	}

	updatedRow, err := s.repo.AdjustStockByUUID(ctx, tx, productUUID, input.QuantityChange)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to adjust stock", err)
	}

	refType := "manual"
	movement, err := s.repo.CreateStockMovement(ctx, tx, sqlc.CreateStockMovementParams{
		ProductID:      stockRow.ProductID,
		ProductUuid:    productUUID,
		ChangeType:     input.ChangeType,
		QuantityChange: input.QuantityChange,
		StockBefore:    currentStock,
		StockAfter:     updatedRow.Quantity,
		ReferenceID:    nil,
		ReferenceType:  &refType,
		ActorID:        nil,
		ActorUuid:      toPgUUID(actorUUID),
		ActorName:      actorName,
		Note:           input.Note,
	})
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to record adjustment movement", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to commit adjustment", err)
	}

	// Trigger low stock alert if inventory drops to or below threshold
	if s.rabbitMQ != nil && updatedRow.Quantity <= 5 {
		pID := int32(0)
		if updatedRow.ProductID != nil {
			pID = *updatedRow.ProductID
		}
		_ = s.rabbitMQ.PublishTopic(ctx, rabbitmq.RoutingKeyInventoryStockLow, map[string]any{
			"product_id":   pID,
			"product_uuid": updatedRow.ProductUuid.String(),
			"name":         updatedRow.ProductName,
			"stock":        updatedRow.Quantity,
		})
	}

	actorUUIDStr := fromPgUUID(movement.ActorUuid)

	return &dto.StockMovementDTO{
		MovementID:      movement.MovementID,
		MovementUUID:    movement.MovementUuid.String(),
		ProductID:       movement.ProductID,
		ProductUUID:     movement.ProductUuid.String(),
		ProductName:     stockRow.ProductName,
		ProductSKU:      &stockRow.ProductSku,
		ProductImageURL: &stockRow.ProductImageUrl,
		ChangeType:      movement.ChangeType,
		QuantityChange:  movement.QuantityChange,
		StockBefore:     movement.StockBefore,
		StockAfter:      movement.StockAfter,
		ReferenceID:     movement.ReferenceID,
		ReferenceType:   movement.ReferenceType,
		ActorID:         movement.ActorID,
		ActorUUID:       actorUUIDStr,
		ActorName:       movement.ActorName,
		Note:            movement.Note,
		CreatedAt:       movement.CreatedAt,
	}, nil
}

func (s *inventoryService) ListStockMovements(ctx context.Context, params dto.GetStockMovementsParam) ([]dto.StockMovementDTO, int64, error) {
	page := params.Page
	if page < 1 {
		page = 1
	}
	limit := params.Limit
	if limit < 1 || limit > 100 {
		limit = 10
	}
	offset := int32((page - 1) * limit)

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

	count, err := s.repo.CountStockMovements(ctx, sqlc.CountStockMovementsParams{
		ProductUuid: productUUIDParam,
		ChangeType:  params.ChangeType,
		FromDate:    fromDateParam,
		ToDate:      toDateParam,
		Search:      params.Search,
	})
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to count stock movements", err)
	}

	rows, err := s.repo.ListStockMovements(ctx, sqlc.ListStockMovementsParams{
		ProductUuid: productUUIDParam,
		ChangeType:  params.ChangeType,
		FromDate:    fromDateParam,
		ToDate:      toDateParam,
		Search:      params.Search,
		Limit:       int32(limit),
		Offset:      offset,
	})
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to list stock movements", err)
	}

	dtos := make([]dto.StockMovementDTO, len(rows))
	for i, r := range rows {
		actorUUIDStr := fromPgUUID(r.ActorUuid)
		var sku *string
		if r.ProductSku != "" {
			sku = &r.ProductSku
		}
		var img *string
		if r.ProductImageUrl != "" {
			img = &r.ProductImageUrl
		}

		dtos[i] = dto.StockMovementDTO{
			MovementID:      r.MovementID,
			MovementUUID:    r.MovementUuid.String(),
			ProductID:       r.ProductID,
			ProductUUID:     r.ProductUuid.String(),
			ProductName:     r.ProductName,
			ProductSKU:      sku,
			ProductImageURL: img,
			ChangeType:      r.ChangeType,
			QuantityChange:  r.QuantityChange,
			StockBefore:     r.StockBefore,
			StockAfter:      r.StockAfter,
			ReferenceID:     r.ReferenceID,
			ReferenceType:   r.ReferenceType,
			ActorID:         r.ActorID,
			ActorUUID:       actorUUIDStr,
			ActorName:       r.ActorName,
			Note:            r.Note,
			CreatedAt:       r.CreatedAt,
		}
	}

	return dtos, count, nil
}
