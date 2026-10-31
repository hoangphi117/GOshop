package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"goshop/contracts/events"
	"goshop/pkg/logger"
	"goshop/pkg/messaging"
	"goshop/pkg/utils"
	"goshop/services/inventory/internal/db/sqlc"
	"goshop/services/inventory/internal/dto"
	"goshop/services/inventory/internal/repository"
)

type InventoryService interface {
	LockAndDeductStockWithRef(ctx context.Context, items []dto.DeductItem, refID string, refType string, actorID *int32, actorUUID *string, note *string) (map[string]dto.ItemStockDTO, error)
	RestoreStockWithRef(ctx context.Context, items []dto.RestoreItem, refID string, refType string, actorID *int32, actorUUID *string, note *string) error
	GetStock(ctx context.Context, productUUIDs []string, productIDs []int32) ([]dto.ItemStockDTO, error)
	ListLowStockProducts(ctx context.Context, threshold, limit int32) ([]dto.ItemStockDTO, int64, error)
	AdjustStock(ctx context.Context, actorUUID *uuid.UUID, actorName *string, input dto.AdjustStockInput) (*dto.StockMovementDTO, error)
	ListStockMovements(ctx context.Context, params dto.GetStockMovementsParam) ([]dto.StockMovementDTO, int64, error)
}

type inventoryService struct {
	repo     repository.InventoryRepository
	dbPool   *pgxpool.Pool
	rabbitMQ messaging.Broker
}

func NewInventoryService(repo repository.InventoryRepository, dbPool *pgxpool.Pool, rmq ...messaging.Broker) InventoryService {
	var r messaging.Broker
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

	// 1. Idempotency check: if reference already recorded, skip deduction without taking transaction
	if refID != "" && refType != "" {
		allAlreadyDeducted := true
		for _, item := range sortedItems {
			parsedUUID, err := uuid.Parse(item.ProductUUID)
			if err != nil {
				return nil, utils.NewError(utils.ErrCodeBadRequest, "Invalid product UUID: "+item.ProductUUID)
			}
			// Tombstone check: if cancellation/release arrived before reserve
			tombstone, tErr := s.repo.GetMovementByRefAndProductUUID(ctx, "order_cancelled", refID, parsedUUID)
			if tErr == nil && tombstone.MovementID > 0 {
				logger.Log.Warn().Str("ref_id", refID).Str("product_uuid", item.ProductUUID).Msg("Reservation rejected: cancellation tombstone found")
				return nil, utils.NewError(utils.ErrCodeConflict, "Reservation rejected: order cancelled prior to stock deduction (tombstone hit)")
			}

			existingMovement, err := s.repo.GetMovementByRefAndProductUUID(ctx, refType, refID, parsedUUID)
			if err == nil && existingMovement.MovementID > 0 {
				stockRow, sErr := s.repo.GetStockByUUID(ctx, parsedUUID)
				if sErr == nil {
					pID := int32(0)
					if stockRow.ProductID != nil {
						pID = *stockRow.ProductID
					}
					resultMap[item.ProductUUID] = dto.ItemStockDTO{
						ProductID:         pID,
						ProductUUID:       stockRow.ProductUuid.String(),
						Quantity:          stockRow.Quantity,
						ReservedQuantity:  stockRow.ReservedQuantity,
						AvailableQuantity: stockRow.Quantity - stockRow.ReservedQuantity,
					}
				}
			} else {
				allAlreadyDeducted = false
				break
			}
		}
		if allAlreadyDeducted && len(resultMap) == len(sortedItems) {
			logger.Log.Info().Msgf("Idempotent stock deduction hit for ref %s", refID)
			return resultMap, nil
		}
		// Reset resultMap if not completely idempotent
		resultMap = make(map[string]dto.ItemStockDTO, len(sortedItems))
	}

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

		// Atomic decrement directly on tx
		updatedStockRow, err := s.repo.DeductStockByUUID(ctx, tx, parsedUUID, item.Quantity)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("insufficient stock for product: %s", item.ProductUUID))
			}
			return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to deduct stock for product: "+item.ProductUUID, err)
		}

		// Record stock movement on tx
		_, err = s.repo.CreateStockMovement(ctx, tx, sqlc.CreateStockMovementParams{
			ProductID:      updatedStockRow.ProductID,
			ProductUuid:    parsedUUID,
			ChangeType:     "ORDER_PLACED",
			QuantityChange: -item.Quantity,
			StockBefore:    updatedStockRow.Quantity + item.Quantity,
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
			Quantity:          updatedStockRow.Quantity,
			ReservedQuantity:  updatedStockRow.ReservedQuantity,
			AvailableQuantity: updatedStockRow.Quantity - updatedStockRow.ReservedQuantity,
		}

		// Low stock alert via RabbitMQ. The catalog owns the product name, so
		// the alert carries identifiers and the notification resolves them.
		if s.rabbitMQ != nil && updatedStockRow.Quantity <= 5 {
			_ = s.rabbitMQ.PublishTopic(ctx, events.RoutingKeyInventoryStockLow, map[string]any{
				"product_id":   pID,
				"product_uuid": updatedStockRow.ProductUuid.String(),
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

		// Verify prior deduction movement exists before restoring to prevent phantom stock inflation
		if (refType == "order" || refType == "order_cancelled" || refType == "order_cancel") && refID != "" {
			deducted, dErr := s.repo.GetMovementByRefAndProductUUID(ctx, "order", refID, parsedUUID)
			if dErr != nil || deducted.MovementID == 0 {
				logger.Log.Info().Msgf("No prior deduction found for ref %s product %s, recording cancellation tombstone", refID, pUUIDStr)
				tombType := "order_cancelled"
				tombNote := "Cancellation tombstone: release before reserve"
				_, _ = s.repo.CreateStockMovement(ctx, tx, sqlc.CreateStockMovementParams{
					ProductID:      nil,
					ProductUuid:    parsedUUID,
					ChangeType:     "ORDER_CANCELLED",
					QuantityChange: 0,
					StockBefore:    0,
					StockAfter:     0,
					ReferenceID:    defaultRefID,
					ReferenceType:  &tombType,
					ActorID:        actorID,
					ActorUuid:      toPgUUID(parsedActorUUID),
					ActorName:      nil,
					Note:           &tombNote,
				})
				continue
			}
		}

		updatedStock, err := s.repo.RestoreStockByUUID(ctx, tx, parsedUUID, qty)
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to restore stock for product: "+pUUIDStr, err)
		}

		_, err = s.repo.CreateStockMovement(ctx, tx, sqlc.CreateStockMovementParams{
			ProductID:      updatedStock.ProductID,
			ProductUuid:    parsedUUID,
			ChangeType:     changeType,
			QuantityChange: qty,
			StockBefore:    updatedStock.Quantity - qty,
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

		// Verify prior deduction movement exists before restoring to prevent phantom stock inflation
		if (refType == "order" || refType == "order_cancelled" || refType == "order_cancel") && refID != "" {
			deducted, dErr := s.repo.GetMovementByRefAndProductID(ctx, "order", refID, pID)
			if dErr != nil || deducted.MovementID == 0 {
				logger.Log.Info().Msgf("No prior deduction found for ref %s product ID %d, recording cancellation tombstone", refID, pID)
				tombType := "order_cancelled"
				tombNote := "Cancellation tombstone: release before reserve"
				_, _ = s.repo.CreateStockMovement(ctx, tx, sqlc.CreateStockMovementParams{
					ProductID:      &pID,
					ProductUuid:    uuid.Nil,
					ChangeType:     "ORDER_CANCELLED",
					QuantityChange: 0,
					StockBefore:    0,
					StockAfter:     0,
					ReferenceID:    defaultRefID,
					ReferenceType:  &tombType,
					ActorID:        actorID,
					ActorUuid:      toPgUUID(parsedActorUUID),
					ActorName:      nil,
					Note:           &tombNote,
				})
				continue
			}
		}

		updatedStock, err := s.repo.RestoreStockByID(ctx, tx, pID, qty)
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, fmt.Sprintf("Failed to restore stock for product ID: %d", pID), err)
		}

		_, err = s.repo.CreateStockMovement(ctx, tx, sqlc.CreateStockMovementParams{
			ProductID:      &pID,
			ProductUuid:    updatedStock.ProductUuid,
			ChangeType:     changeType,
			QuantityChange: qty,
			StockBefore:    updatedStock.Quantity - qty,
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
				Quantity:          r.Quantity,
				ReservedQuantity:  r.ReservedQuantity,
				AvailableQuantity: r.Quantity - r.ReservedQuantity,
			})
		}
	}

	return result, nil
}

// ListLowStockProducts returns the products with the lowest available stock
// (available = quantity - reserved) at or below the threshold, plus the total
// number of products matching the threshold. Consumed by analytics alerts.
func (s *inventoryService) ListLowStockProducts(ctx context.Context, threshold, limit int32) ([]dto.ItemStockDTO, int64, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.repo.ListLowStockProducts(ctx, threshold, limit)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to list low stock products", err)
	}
	total, err := s.repo.CountLowStockProducts(ctx, threshold)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to count low stock products", err)
	}

	items := make([]dto.ItemStockDTO, 0, len(rows))
	for _, r := range rows {
		pID := int32(0)
		if r.ProductID != nil {
			pID = *r.ProductID
		}
		items = append(items, dto.ItemStockDTO{
			ProductID:         pID,
			ProductUUID:       r.ProductUuid.String(),
			Quantity:          0,
			ReservedQuantity:  0,
			AvailableQuantity: r.AvailableQuantity,
		})
	}
	return items, total, nil
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

	// Idempotency: a caller-supplied reference means a retry must not apply the
	// delta twice. Return the movement already recorded for that reference.
	// Only INITIAL_STOCK honours it; other change types keep the manual path.
	refID := ""
	if input.ChangeType == "INITIAL_STOCK" && input.ReferenceID != nil && *input.ReferenceID != "" {
		refID = *input.ReferenceID
		if existing, gErr := s.repo.GetMovementByRefAndProductUUID(ctx, "INITIAL_STOCK", refID, productUUID); gErr == nil && existing.MovementID > 0 {
			logger.Log.Info().Str("reference_id", refID).Str("product_uuid", productUUID.String()).Msg("Idempotent stock adjustment hit")
			return s.movementToDTO(ctx, &existing), nil
		}
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
				ProductID:   input.ProductID,
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
	// Absolute-set path: one UPDATE decides the final value inside this
	// transaction, so two concurrent admins cannot lose each other's change.
	if input.TargetQuantity != nil {
		updatedRow, aErr := s.repo.SetStockQuantity(ctx, tx, productUUID, *input.TargetQuantity)
		if aErr != nil {
			var pgErr *pgconn.PgError
			if errors.As(aErr, &pgErr) && pgErr.Code == "23514" {
				return nil, utils.NewError(utils.ErrCodeBadRequest, "target quantity cannot be negative")
			}
			return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to set stock", aErr)
		}
		change := updatedRow.Quantity - currentStock
		if change == 0 {
			return s.movementToDTO(ctx, &sqlc.StockMovement{
				ProductID:     stockRow.ProductID,
				ProductUuid:   productUUID,
				ChangeType:    input.ChangeType,
				ReferenceType: ptr("manual"),
				StockBefore:   currentStock,
				StockAfter:    currentStock,
			}), nil
		}
		return s.recordAdjustment(ctx, tx, stockRow, productUUID, currentStock, updatedRow.Quantity, actorUUID, actorName, input, refID)
	}

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
	return s.recordAdjustment(ctx, tx, stockRow, productUUID, currentStock, updatedRow.Quantity, actorUUID, actorName, input, refID)
}

// ptr is a local helper so literal pointers stay readable inline.
func ptr[T any](v T) *T { return &v }

// recordAdjustment writes the movement row, commits, and emits the low-stock
// alert. Shared by the delta and absolute-set paths so both produce identical
// side effects.
func (s *inventoryService) recordAdjustment(
	ctx context.Context,
	tx pgx.Tx,
	stockRow sqlc.InventoryStock,
	productUUID uuid.UUID,
	currentStock int32,
	newStock int32,
	actorUUID *uuid.UUID,
	actorName *string,
	input dto.AdjustStockInput,
	refID string,
) (*dto.StockMovementDTO, error) {
	refType := "manual"
	var referenceID *string
	if refID != "" {
		refType = "INITIAL_STOCK"
		referenceID = &refID
	}
	movement, err := s.repo.CreateStockMovement(ctx, tx, sqlc.CreateStockMovementParams{
		ProductID:      stockRow.ProductID,
		ProductUuid:    productUUID,
		ChangeType:     input.ChangeType,
		QuantityChange: newStock - currentStock,
		StockBefore:    currentStock,
		StockAfter:     newStock,
		ReferenceID:    referenceID,
		ReferenceType:  &refType,
		ActorID:        nil,
		ActorUuid:      toPgUUID(actorUUID),
		ActorName:      actorName,
		Note:           input.Note,
	})
	if err != nil {
		// Concurrent retry already recorded this reference: treat as success.
		var pgErr *pgconn.PgError
		if refID != "" && errors.As(err, &pgErr) && pgErr.Code == "23505" {
			logger.Log.Info().Str("reference_id", refID).Msg("Stock adjustment race lost; returning existing movement")
			if existing, gErr := s.repo.GetMovementByRefAndProductUUID(ctx, refType, refID, productUUID); gErr == nil && existing.MovementID > 0 {
				return s.movementToDTO(ctx, &existing), nil
			}
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to record adjustment movement", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to commit adjustment", err)
	}

	// Trigger low stock alert if inventory drops to or below threshold.
	// The catalog owns the product name; the alert carries identifiers.
	if s.rabbitMQ != nil && newStock <= 5 {
		pID := int32(0)
		if stockRow.ProductID != nil {
			pID = *stockRow.ProductID
		}
		_ = s.rabbitMQ.PublishTopic(ctx, events.RoutingKeyInventoryStockLow, map[string]any{
			"product_id":   pID,
			"product_uuid": productUUID.String(),
			"stock":        newStock,
		})
	}

	return s.movementToDTO(ctx, &movement), nil
}

// movementToDTO builds the response from a recorded movement. The movement row
// is the whole contract: product name/SKU/image belong to the catalog.
func (s *inventoryService) movementToDTO(_ context.Context, m *sqlc.StockMovement) *dto.StockMovementDTO {
	return &dto.StockMovementDTO{
		MovementID:     m.MovementID,
		MovementUUID:   m.MovementUuid.String(),
		ProductID:      m.ProductID,
		ProductUUID:    m.ProductUuid.String(),
		ChangeType:     m.ChangeType,
		QuantityChange: m.QuantityChange,
		StockBefore:    m.StockBefore,
		StockAfter:     m.StockAfter,
		ReferenceID:    m.ReferenceID,
		ReferenceType:  m.ReferenceType,
		ActorID:        m.ActorID,
		ActorUUID:      fromPgUUID(m.ActorUuid),
		ActorName:      m.ActorName,
		Note:           m.Note,
		CreatedAt:      m.CreatedAt,
	}
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
		dtos[i] = dto.StockMovementDTO{
			MovementID:     r.MovementID,
			MovementUUID:   r.MovementUuid.String(),
			ProductID:      r.ProductID,
			ProductUUID:    r.ProductUuid.String(),
			ChangeType:     r.ChangeType,
			QuantityChange: r.QuantityChange,
			StockBefore:    r.StockBefore,
			StockAfter:     r.StockAfter,
			ReferenceID:    r.ReferenceID,
			ReferenceType:  r.ReferenceType,
			ActorID:        r.ActorID,
			ActorUUID:      fromPgUUID(r.ActorUuid),
			ActorName:      r.ActorName,
			Note:           r.Note,
			CreatedAt:      r.CreatedAt,
		}
	}

	return dtos, count, nil
}
