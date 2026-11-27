package service_test

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"goshop/services/inventory/internal/db/sqlc"
	"goshop/services/inventory/internal/repository"
	"goshop/shared/pkg/messaging"
)

var _ repository.InventoryRepository = (*mockInventoryRepo)(nil)

type fakeTx struct {
	pgx.Tx
	committed  bool
	rolledBack bool
	commitErr  error
}

func (f *fakeTx) Commit(context.Context) error {
	f.committed = true
	return f.commitErr
}

func (f *fakeTx) Rollback(context.Context) error {
	f.rolledBack = true
	return nil
}

type fakeTxBeginner struct {
	tx  *fakeTx
	err error
}

func (f *fakeTxBeginner) Begin(context.Context) (pgx.Tx, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.tx == nil {
		f.tx = &fakeTx{}
	}
	return f.tx, nil
}

type publishedTopic struct {
	routingKey string
	message    any
}

type fakeBroker struct {
	messaging.Broker
	topics []publishedTopic
	err    error
}

func (f *fakeBroker) PublishTopic(_ context.Context, routingKey string, message any) error {
	f.topics = append(f.topics, publishedTopic{routingKey: routingKey, message: message})
	return f.err
}

type mockInventoryRepo struct {
	getStockByUUIDFn               func(ctx context.Context, uuid uuid.UUID) (sqlc.InventoryStock, error)
	getStockByUUIDsFn              func(ctx context.Context, uuids []uuid.UUID) ([]sqlc.InventoryStock, error)
	getStockByProductIDFn          func(ctx context.Context, id int32) (sqlc.InventoryStock, error)
	getStockByProductIDsFn         func(ctx context.Context, ids []int32) ([]sqlc.InventoryStock, error)
	upsertStockMetadataFn          func(ctx context.Context, tx pgx.Tx, params sqlc.UpsertStockMetadataParams) (sqlc.InventoryStock, error)
	deductStockByUUIDFn            func(ctx context.Context, tx pgx.Tx, productUUID uuid.UUID, quantity int32) (sqlc.InventoryStock, error)
	deductStockByIDFn              func(ctx context.Context, tx pgx.Tx, productID int32, quantity int32) (sqlc.InventoryStock, error)
	restoreStockByUUIDFn           func(ctx context.Context, tx pgx.Tx, productUUID uuid.UUID, quantity int32) (sqlc.InventoryStock, error)
	restoreStockByIDFn             func(ctx context.Context, tx pgx.Tx, productID int32, quantity int32) (sqlc.InventoryStock, error)
	adjustStockByUUIDFn            func(ctx context.Context, tx pgx.Tx, productUUID uuid.UUID, quantityChange int32) (sqlc.InventoryStock, error)
	setStockQuantityFn             func(ctx context.Context, tx pgx.Tx, productUUID uuid.UUID, quantity int32) (sqlc.InventoryStock, error)
	createStockMovementFn          func(ctx context.Context, tx pgx.Tx, params sqlc.CreateStockMovementParams) (sqlc.StockMovement, error)
	getMovementByRefAndUUIDFn      func(ctx context.Context, refType, refID string, productUUID uuid.UUID) (sqlc.StockMovement, error)
	getMovementByRefAndProductIDFn func(ctx context.Context, refType, refID string, productID int32) (sqlc.StockMovement, error)
	listStockMovementsFn           func(ctx context.Context, params sqlc.ListStockMovementsParams) ([]sqlc.StockMovement, error)
	countStockMovementsFn          func(ctx context.Context, params sqlc.CountStockMovementsParams) (int64, error)
	listLowStockProductsFn         func(ctx context.Context, threshold, limit int32) ([]sqlc.ListLowStockProductsRow, error)
	countLowStockProductsFn        func(ctx context.Context, threshold int32) (int64, error)
}

func (m *mockInventoryRepo) GetStockByUUID(ctx context.Context, u uuid.UUID) (sqlc.InventoryStock, error) {
	if m.getStockByUUIDFn != nil {
		return m.getStockByUUIDFn(ctx, u)
	}
	return sqlc.InventoryStock{}, pgx.ErrNoRows
}

func (m *mockInventoryRepo) GetStockByUUIDs(ctx context.Context, uuids []uuid.UUID) ([]sqlc.InventoryStock, error) {
	if m.getStockByUUIDsFn != nil {
		return m.getStockByUUIDsFn(ctx, uuids)
	}
	return nil, nil
}

func (m *mockInventoryRepo) GetStockByProductID(ctx context.Context, id int32) (sqlc.InventoryStock, error) {
	if m.getStockByProductIDFn != nil {
		return m.getStockByProductIDFn(ctx, id)
	}
	return sqlc.InventoryStock{}, pgx.ErrNoRows
}

func (m *mockInventoryRepo) GetStockByProductIDs(ctx context.Context, ids []int32) ([]sqlc.InventoryStock, error) {
	if m.getStockByProductIDsFn != nil {
		return m.getStockByProductIDsFn(ctx, ids)
	}
	return nil, nil
}

func (m *mockInventoryRepo) UpsertStockMetadata(ctx context.Context, tx pgx.Tx, params sqlc.UpsertStockMetadataParams) (sqlc.InventoryStock, error) {
	if m.upsertStockMetadataFn != nil {
		return m.upsertStockMetadataFn(ctx, tx, params)
	}
	return sqlc.InventoryStock{}, nil
}

func (m *mockInventoryRepo) DeductStockByUUID(ctx context.Context, tx pgx.Tx, productUUID uuid.UUID, quantity int32) (sqlc.InventoryStock, error) {
	if m.deductStockByUUIDFn != nil {
		return m.deductStockByUUIDFn(ctx, tx, productUUID, quantity)
	}
	return sqlc.InventoryStock{}, nil
}

func (m *mockInventoryRepo) DeductStockByID(ctx context.Context, tx pgx.Tx, productID int32, quantity int32) (sqlc.InventoryStock, error) {
	if m.deductStockByIDFn != nil {
		return m.deductStockByIDFn(ctx, tx, productID, quantity)
	}
	return sqlc.InventoryStock{}, nil
}

func (m *mockInventoryRepo) RestoreStockByUUID(ctx context.Context, tx pgx.Tx, productUUID uuid.UUID, quantity int32) (sqlc.InventoryStock, error) {
	if m.restoreStockByUUIDFn != nil {
		return m.restoreStockByUUIDFn(ctx, tx, productUUID, quantity)
	}
	return sqlc.InventoryStock{}, nil
}

func (m *mockInventoryRepo) RestoreStockByID(ctx context.Context, tx pgx.Tx, productID int32, quantity int32) (sqlc.InventoryStock, error) {
	if m.restoreStockByIDFn != nil {
		return m.restoreStockByIDFn(ctx, tx, productID, quantity)
	}
	return sqlc.InventoryStock{}, nil
}

func (m *mockInventoryRepo) AdjustStockByUUID(ctx context.Context, tx pgx.Tx, productUUID uuid.UUID, quantityChange int32) (sqlc.InventoryStock, error) {
	if m.adjustStockByUUIDFn != nil {
		return m.adjustStockByUUIDFn(ctx, tx, productUUID, quantityChange)
	}
	return sqlc.InventoryStock{}, nil
}

func (m *mockInventoryRepo) SetStockQuantity(ctx context.Context, tx pgx.Tx, productUUID uuid.UUID, quantity int32) (sqlc.InventoryStock, error) {
	if m.setStockQuantityFn != nil {
		return m.setStockQuantityFn(ctx, tx, productUUID, quantity)
	}
	return sqlc.InventoryStock{}, nil
}

func (m *mockInventoryRepo) CreateStockMovement(ctx context.Context, tx pgx.Tx, params sqlc.CreateStockMovementParams) (sqlc.StockMovement, error) {
	if m.createStockMovementFn != nil {
		return m.createStockMovementFn(ctx, tx, params)
	}
	return sqlc.StockMovement{
		MovementID:     1,
		MovementUuid:   uuid.New(),
		ProductID:      params.ProductID,
		ProductUuid:    params.ProductUuid,
		ChangeType:     params.ChangeType,
		QuantityChange: params.QuantityChange,
		StockBefore:    params.StockBefore,
		StockAfter:     params.StockAfter,
		ReferenceID:    params.ReferenceID,
		ReferenceType:  params.ReferenceType,
	}, nil
}

func (m *mockInventoryRepo) GetMovementByRefAndProductUUID(ctx context.Context, refType, refID string, productUUID uuid.UUID) (sqlc.StockMovement, error) {
	if m.getMovementByRefAndUUIDFn != nil {
		return m.getMovementByRefAndUUIDFn(ctx, refType, refID, productUUID)
	}
	return sqlc.StockMovement{}, pgx.ErrNoRows
}

func (m *mockInventoryRepo) GetMovementByRefAndProductID(ctx context.Context, refType, refID string, productID int32) (sqlc.StockMovement, error) {
	if m.getMovementByRefAndProductIDFn != nil {
		return m.getMovementByRefAndProductIDFn(ctx, refType, refID, productID)
	}
	return sqlc.StockMovement{}, pgx.ErrNoRows
}

func (m *mockInventoryRepo) ListStockMovements(ctx context.Context, params sqlc.ListStockMovementsParams) ([]sqlc.StockMovement, error) {
	if m.listStockMovementsFn != nil {
		return m.listStockMovementsFn(ctx, params)
	}
	return nil, nil
}

func (m *mockInventoryRepo) CountStockMovements(ctx context.Context, params sqlc.CountStockMovementsParams) (int64, error) {
	if m.countStockMovementsFn != nil {
		return m.countStockMovementsFn(ctx, params)
	}
	return 0, nil
}

func (m *mockInventoryRepo) ListLowStockProducts(ctx context.Context, threshold, limit int32) ([]sqlc.ListLowStockProductsRow, error) {
	if m.listLowStockProductsFn != nil {
		return m.listLowStockProductsFn(ctx, threshold, limit)
	}
	return nil, nil
}

func (m *mockInventoryRepo) CountLowStockProducts(ctx context.Context, threshold int32) (int64, error) {
	if m.countLowStockProductsFn != nil {
		return m.countLowStockProductsFn(ctx, threshold)
	}
	return 0, nil
}
