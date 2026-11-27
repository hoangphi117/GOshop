package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goshop/contracts/events"
	"goshop/services/inventory/internal/db/sqlc"
	"goshop/services/inventory/internal/dto"
	"goshop/services/inventory/internal/service"
	"goshop/shared/pkg/utils"
)

func assertAppErrCode(t *testing.T, err error, want utils.ErrorCode) {
	t.Helper()
	var appErr *utils.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, want, appErr.Code)
}

func TestLockAndDeductStock_SuccessReducesStockAndPublishesLowStock(t *testing.T) {
	p1 := uuid.New()
	pID := int32(42)
	tx := &fakeTx{}
	db := &fakeTxBeginner{tx: tx}
	broker := &fakeBroker{}
	var recorded []sqlc.CreateStockMovementParams

	repo := &mockInventoryRepo{
		deductStockByUUIDFn: func(_ context.Context, _ pgx.Tx, productUUID uuid.UUID, qty int32) (sqlc.InventoryStock, error) {
			assert.Equal(t, p1, productUUID)
			assert.Equal(t, int32(5), qty) // consolidated from 2 + 3
			return sqlc.InventoryStock{
				ProductID:        &pID,
				ProductUuid:      p1,
				Quantity:         4, // <= 5 triggers low stock alert
				ReservedQuantity: 1,
			}, nil
		},
		createStockMovementFn: func(_ context.Context, _ pgx.Tx, params sqlc.CreateStockMovementParams) (sqlc.StockMovement, error) {
			recorded = append(recorded, params)
			return sqlc.StockMovement{MovementID: 10}, nil
		},
	}

	svc := service.NewInventoryService(repo, db, broker)
	res, err := svc.LockAndDeductStockWithRef(
		context.Background(),
		[]dto.DeductItem{
			{ProductUUID: p1.String(), Quantity: 2},
			{ProductUUID: p1.String(), Quantity: 3},
		},
		"order-123",
		"order",
		nil,
		nil,
		nil,
	)

	require.NoError(t, err)
	assert.True(t, tx.committed)
	require.Contains(t, res, p1.String())
	assert.Equal(t, int32(4), res[p1.String()].Quantity)
	assert.Equal(t, int32(3), res[p1.String()].AvailableQuantity)

	require.Len(t, recorded, 1)
	assert.Equal(t, "ORDER_PLACED", recorded[0].ChangeType)
	assert.Equal(t, int32(-5), recorded[0].QuantityChange)
	assert.Equal(t, int32(9), recorded[0].StockBefore)
	assert.Equal(t, int32(4), recorded[0].StockAfter)

	require.Len(t, broker.topics, 1)
	assert.Equal(t, events.RoutingKeyInventoryStockLow, broker.topics[0].routingKey)
}

func TestLockAndDeductStock_ValidationAndInsufficientStock(t *testing.T) {
	p1 := uuid.New()
	tx := &fakeTx{}
	db := &fakeTxBeginner{tx: tx}

	svc := service.NewInventoryService(&mockInventoryRepo{
		deductStockByUUIDFn: func(context.Context, pgx.Tx, uuid.UUID, int32) (sqlc.InventoryStock, error) {
			return sqlc.InventoryStock{}, pgx.ErrNoRows
		},
	}, db)

	t.Run("empty items", func(t *testing.T) {
		_, err := svc.LockAndDeductStockWithRef(context.Background(), nil, "ref", "order", nil, nil, nil)
		assertAppErrCode(t, err, utils.ErrCodeBadRequest)
	})

	t.Run("non-positive quantity", func(t *testing.T) {
		_, err := svc.LockAndDeductStockWithRef(context.Background(), []dto.DeductItem{{ProductUUID: p1.String(), Quantity: 0}}, "ref", "order", nil, nil, nil)
		assertAppErrCode(t, err, utils.ErrCodeBadRequest)
	})

	t.Run("insufficient stock", func(t *testing.T) {
		_, err := svc.LockAndDeductStockWithRef(context.Background(), []dto.DeductItem{{ProductUUID: p1.String(), Quantity: 5}}, "ref", "order", nil, nil, nil)
		assertAppErrCode(t, err, utils.ErrCodeBadRequest)
		assert.False(t, tx.committed)
		assert.True(t, tx.rolledBack)
	})
}

func TestLockAndDeductStock_IdempotentWithSameRef(t *testing.T) {
	p1 := uuid.New()
	pID := int32(7)
	db := &fakeTxBeginner{}
	deductCalled := false

	repo := &mockInventoryRepo{
		getMovementByRefAndUUIDFn: func(_ context.Context, refType, refID string, productUUID uuid.UUID) (sqlc.StockMovement, error) {
			if refType == "order" && refID == "ord-1" && productUUID == p1 {
				return sqlc.StockMovement{MovementID: 99}, nil
			}
			return sqlc.StockMovement{}, pgx.ErrNoRows
		},
		getStockByUUIDFn: func(_ context.Context, u uuid.UUID) (sqlc.InventoryStock, error) {
			return sqlc.InventoryStock{ProductID: &pID, ProductUuid: u, Quantity: 20, ReservedQuantity: 2}, nil
		},
		deductStockByUUIDFn: func(context.Context, pgx.Tx, uuid.UUID, int32) (sqlc.InventoryStock, error) {
			deductCalled = true
			return sqlc.InventoryStock{}, nil
		},
	}

	svc := service.NewInventoryService(repo, db)
	res, err := svc.LockAndDeductStockWithRef(context.Background(), []dto.DeductItem{{ProductUUID: p1.String(), Quantity: 3}}, "ord-1", "order", nil, nil, nil)

	require.NoError(t, err)
	assert.False(t, deductCalled)
	assert.Nil(t, db.tx) // no transaction started
	assert.Equal(t, int32(18), res[p1.String()].AvailableQuantity)
}

func TestLockAndDeductStock_TombstoneRejectsReservation(t *testing.T) {
	p1 := uuid.New()
	db := &fakeTxBeginner{}

	repo := &mockInventoryRepo{
		getMovementByRefAndUUIDFn: func(_ context.Context, refType, refID string, _ uuid.UUID) (sqlc.StockMovement, error) {
			if refType == "order_cancelled" && refID == "ord-dead" {
				return sqlc.StockMovement{MovementID: 55}, nil
			}
			return sqlc.StockMovement{}, pgx.ErrNoRows
		},
	}

	svc := service.NewInventoryService(repo, db)
	_, err := svc.LockAndDeductStockWithRef(context.Background(), []dto.DeductItem{{ProductUUID: p1.String(), Quantity: 1}}, "ord-dead", "order", nil, nil, nil)

	assertAppErrCode(t, err, utils.ErrCodeConflict)
	assert.Nil(t, db.tx)
}

func TestRestoreStockWithRef_RevertsDeductionAndHandlesTombstone(t *testing.T) {
	p1 := uuid.New()
	pID := int32(11)

	t.Run("restores when prior deduction exists", func(t *testing.T) {
		tx := &fakeTx{}
		var restoredQty int32
		var createdChangeType string

		repo := &mockInventoryRepo{
			getMovementByRefAndUUIDFn: func(_ context.Context, refType, refID string, _ uuid.UUID) (sqlc.StockMovement, error) {
				if refType == "order" && refID == "ord-1" {
					return sqlc.StockMovement{MovementID: 12}, nil
				}
				return sqlc.StockMovement{}, pgx.ErrNoRows
			},
			restoreStockByUUIDFn: func(_ context.Context, _ pgx.Tx, _ uuid.UUID, qty int32) (sqlc.InventoryStock, error) {
				restoredQty = qty
				return sqlc.InventoryStock{ProductID: &pID, ProductUuid: p1, Quantity: 15}, nil
			},
			createStockMovementFn: func(_ context.Context, _ pgx.Tx, params sqlc.CreateStockMovementParams) (sqlc.StockMovement, error) {
				createdChangeType = params.ChangeType
				return sqlc.StockMovement{MovementID: 13}, nil
			},
		}

		svc := service.NewInventoryService(repo, &fakeTxBeginner{tx: tx})
		err := svc.RestoreStockWithRef(context.Background(), []dto.RestoreItem{{ProductUUID: p1.String(), Quantity: 3}}, "ord-1", "order_cancel", nil, nil, nil)

		require.NoError(t, err)
		assert.True(t, tx.committed)
		assert.Equal(t, int32(3), restoredQty)
		assert.Equal(t, "ORDER_CANCELLED", createdChangeType)
	})

	t.Run("writes tombstone when no prior deduction exists", func(t *testing.T) {
		tx := &fakeTx{}
		restoreCalled := false
		var recordedParams sqlc.CreateStockMovementParams

		repo := &mockInventoryRepo{
			restoreStockByUUIDFn: func(context.Context, pgx.Tx, uuid.UUID, int32) (sqlc.InventoryStock, error) {
				restoreCalled = true
				return sqlc.InventoryStock{}, nil
			},
			createStockMovementFn: func(_ context.Context, _ pgx.Tx, params sqlc.CreateStockMovementParams) (sqlc.StockMovement, error) {
				recordedParams = params
				return sqlc.StockMovement{MovementID: 14}, nil
			},
		}

		svc := service.NewInventoryService(repo, &fakeTxBeginner{tx: tx})
		err := svc.RestoreStockWithRef(context.Background(), []dto.RestoreItem{{ProductUUID: p1.String(), Quantity: 2}}, "ord-missing", "order_cancel", nil, nil, nil)

		require.NoError(t, err)
		assert.False(t, restoreCalled)
		assert.True(t, tx.committed)
		assert.Equal(t, int32(0), recordedParams.QuantityChange)
		require.NotNil(t, recordedParams.ReferenceType)
		assert.Equal(t, "order_cancelled", *recordedParams.ReferenceType)
	})
}

func TestGetStock_MergesAndDeduplicatesUUIDsAndIDs(t *testing.T) {
	p1 := uuid.New()
	p2 := uuid.New()
	id1, id2 := int32(1), int32(2)

	repo := &mockInventoryRepo{
		getStockByUUIDsFn: func(_ context.Context, uuids []uuid.UUID) ([]sqlc.InventoryStock, error) {
			require.Len(t, uuids, 1)
			return []sqlc.InventoryStock{
				{ProductID: &id1, ProductUuid: p1, Quantity: 10, ReservedQuantity: 2},
			}, nil
		},
		getStockByProductIDsFn: func(_ context.Context, ids []int32) ([]sqlc.InventoryStock, error) {
			return []sqlc.InventoryStock{
				{ProductID: &id1, ProductUuid: p1, Quantity: 10, ReservedQuantity: 2}, // duplicate of p1
				{ProductID: &id2, ProductUuid: p2, Quantity: 5, ReservedQuantity: 0},
			}, nil
		},
	}

	svc := service.NewInventoryService(repo, &fakeTxBeginner{})
	items, err := svc.GetStock(context.Background(), []string{p1.String(), "not-a-uuid"}, []int32{1, 2})

	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, int32(8), items[0].AvailableQuantity)
	assert.Equal(t, int32(5), items[1].AvailableQuantity)
}

func TestListLowStockProducts_DefaultLimitAndError(t *testing.T) {
	p1 := uuid.New()
	id1 := int32(1)
	var capturedLimit int32

	repo := &mockInventoryRepo{
		listLowStockProductsFn: func(_ context.Context, threshold, limit int32) ([]sqlc.ListLowStockProductsRow, error) {
			capturedLimit = limit
			return []sqlc.ListLowStockProductsRow{
				{ProductID: &id1, ProductUuid: p1, AvailableQuantity: 3},
			}, nil
		},
		countLowStockProductsFn: func(context.Context, int32) (int64, error) {
			return 1, nil
		},
	}

	svc := service.NewInventoryService(repo, &fakeTxBeginner{})
	items, total, err := svc.ListLowStockProducts(context.Background(), 5, 0)

	require.NoError(t, err)
	assert.Equal(t, int32(10), capturedLimit)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, int32(3), items[0].AvailableQuantity)

	svcErr := service.NewInventoryService(&mockInventoryRepo{
		listLowStockProductsFn: func(context.Context, int32, int32) ([]sqlc.ListLowStockProductsRow, error) {
			return nil, errors.New("db down")
		},
	}, &fakeTxBeginner{})
	_, _, err = svcErr.ListLowStockProducts(context.Background(), 5, 10)
	assertAppErrCode(t, err, utils.ErrCodeInternal)
}

func TestAdjustStock_DeltaTargetAndValidation(t *testing.T) {
	p1 := uuid.New()
	pID := int32(9)
	actor := uuid.New()
	actorName := "Admin"

	t.Run("rejects adjustment below zero", func(t *testing.T) {
		tx := &fakeTx{}
		repo := &mockInventoryRepo{
			getStockByUUIDFn: func(context.Context, uuid.UUID) (sqlc.InventoryStock, error) {
				return sqlc.InventoryStock{ProductID: &pID, ProductUuid: p1, Quantity: 3}, nil
			},
		}
		svc := service.NewInventoryService(repo, &fakeTxBeginner{tx: tx})
		_, err := svc.AdjustStock(context.Background(), &actor, &actorName, dto.AdjustStockInput{
			ProductUUID:    p1.String(),
			QuantityChange: -5,
			ChangeType:     "MANUAL_ADJUSTMENT",
		})
		assertAppErrCode(t, err, utils.ErrCodeBadRequest)
		assert.False(t, tx.committed)
	})

	t.Run("target quantity sets absolute stock and records movement", func(t *testing.T) {
		tx := &fakeTx{}
		target := int32(3)
		broker := &fakeBroker{}
		repo := &mockInventoryRepo{
			getStockByUUIDFn: func(context.Context, uuid.UUID) (sqlc.InventoryStock, error) {
				return sqlc.InventoryStock{ProductID: &pID, ProductUuid: p1, Quantity: 10}, nil
			},
			setStockQuantityFn: func(_ context.Context, _ pgx.Tx, _ uuid.UUID, q int32) (sqlc.InventoryStock, error) {
				return sqlc.InventoryStock{ProductID: &pID, ProductUuid: p1, Quantity: q}, nil
			},
		}
		svc := service.NewInventoryService(repo, &fakeTxBeginner{tx: tx}, broker)
		mv, err := svc.AdjustStock(context.Background(), &actor, &actorName, dto.AdjustStockInput{
			ProductUUID:    p1.String(),
			ChangeType:     "MANUAL_ADJUSTMENT",
			TargetQuantity: &target,
		})
		require.NoError(t, err)
		assert.True(t, tx.committed)
		assert.Equal(t, int32(-7), mv.QuantityChange)
		assert.Equal(t, int32(3), mv.StockAfter)
		require.Len(t, broker.topics, 1) // 3 <= 5 triggers low stock alert
	})

	t.Run("initial stock reference is idempotent", func(t *testing.T) {
		ref := "init-prod-1"
		db := &fakeTxBeginner{}
		repo := &mockInventoryRepo{
			getMovementByRefAndUUIDFn: func(_ context.Context, refType, refID string, _ uuid.UUID) (sqlc.StockMovement, error) {
				if refType == "INITIAL_STOCK" && refID == ref {
					return sqlc.StockMovement{MovementID: 77, ProductUuid: p1, StockAfter: 50}, nil
				}
				return sqlc.StockMovement{}, pgx.ErrNoRows
			},
		}
		svc := service.NewInventoryService(repo, db)
		mv, err := svc.AdjustStock(context.Background(), &actor, &actorName, dto.AdjustStockInput{
			ProductUUID:    p1.String(),
			QuantityChange: 50,
			ChangeType:     "INITIAL_STOCK",
			ReferenceID:    &ref,
		})
		require.NoError(t, err)
		assert.Nil(t, db.tx)
		assert.Equal(t, int64(77), mv.MovementID)
	})
}

func TestListStockMovements_PaginationAndDateFilters(t *testing.T) {
	p1 := uuid.New()
	from := "2026-11-01"
	to := "2026-11-10"
	var gotParams sqlc.ListStockMovementsParams

	repo := &mockInventoryRepo{
		countStockMovementsFn: func(context.Context, sqlc.CountStockMovementsParams) (int64, error) {
			return 25, nil
		},
		listStockMovementsFn: func(_ context.Context, params sqlc.ListStockMovementsParams) ([]sqlc.StockMovement, error) {
			gotParams = params
			return []sqlc.StockMovement{
				{MovementID: 1, MovementUuid: uuid.New(), ProductUuid: p1, ChangeType: "RESTOCK", CreatedAt: time.Now()},
			}, nil
		},
	}

	svc := service.NewInventoryService(repo, &fakeTxBeginner{})
	items, total, err := svc.ListStockMovements(context.Background(), dto.GetStockMovementsParam{
		Page:        2,
		Limit:       200, // clamped to 10
		ProductUUID: ptrStr(p1.String()),
		FromDate:    &from,
		ToDate:      &to,
	})

	require.NoError(t, err)
	assert.Equal(t, int64(25), total)
	require.Len(t, items, 1)
	assert.Equal(t, int32(10), gotParams.Limit)
	assert.Equal(t, int32(10), gotParams.Offset)
	assert.True(t, gotParams.ProductUuid.Valid)
	assert.True(t, gotParams.FromDate.Valid)
	assert.True(t, gotParams.ToDate.Valid)
}

func ptrStr(s string) *string { return &s }
