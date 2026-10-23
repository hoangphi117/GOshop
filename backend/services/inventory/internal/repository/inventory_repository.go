package repository

import (
	"context"
	"goshop/services/inventory/internal/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type InventoryRepository interface {
	GetStockByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.InventoryStock, error)
	GetStockByUUIDs(ctx context.Context, uuids []uuid.UUID) ([]sqlc.InventoryStock, error)
	GetStockByProductID(ctx context.Context, id int32) (sqlc.InventoryStock, error)
	GetStockByProductIDs(ctx context.Context, ids []int32) ([]sqlc.InventoryStock, error)
	UpsertStockMetadata(ctx context.Context, tx pgx.Tx, params sqlc.UpsertStockMetadataParams) (sqlc.InventoryStock, error)
	DeductStockByUUID(ctx context.Context, tx pgx.Tx, productUUID uuid.UUID, quantity int32) (sqlc.InventoryStock, error)
	DeductStockByID(ctx context.Context, tx pgx.Tx, productID int32, quantity int32) (sqlc.InventoryStock, error)
	RestoreStockByUUID(ctx context.Context, tx pgx.Tx, productUUID uuid.UUID, quantity int32) (sqlc.InventoryStock, error)
	RestoreStockByID(ctx context.Context, tx pgx.Tx, productID int32, quantity int32) (sqlc.InventoryStock, error)
	AdjustStockByUUID(ctx context.Context, tx pgx.Tx, productUUID uuid.UUID, quantityChange int32) (sqlc.InventoryStock, error)
	SetStockQuantity(ctx context.Context, tx pgx.Tx, productUUID uuid.UUID, quantity int32) (sqlc.InventoryStock, error)
	CreateStockMovement(ctx context.Context, tx pgx.Tx, params sqlc.CreateStockMovementParams) (sqlc.StockMovement, error)
	GetMovementByRefAndProductUUID(ctx context.Context, refType, refID string, productUUID uuid.UUID) (sqlc.StockMovement, error)
	GetMovementByRefAndProductID(ctx context.Context, refType, refID string, productID int32) (sqlc.StockMovement, error)
	ListStockMovements(ctx context.Context, params sqlc.ListStockMovementsParams) ([]sqlc.StockMovement, error)
	CountStockMovements(ctx context.Context, params sqlc.CountStockMovementsParams) (int64, error)
}

type SqlInventoryRepository struct {
	q *sqlc.Queries
}

func NewSqlInventoryRepository(q *sqlc.Queries) *SqlInventoryRepository {
	return &SqlInventoryRepository{q: q}
}

func (r *SqlInventoryRepository) querier(tx pgx.Tx) *sqlc.Queries {
	if tx != nil {
		return r.q.WithTx(tx)
	}
	return r.q
}

func (r *SqlInventoryRepository) GetStockByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.InventoryStock, error) {
	return r.q.GetStockByUUID(ctx, uuid)
}

func (r *SqlInventoryRepository) GetStockByUUIDs(ctx context.Context, uuids []uuid.UUID) ([]sqlc.InventoryStock, error) {
	return r.q.GetStockByUUIDs(ctx, uuids)
}

func (r *SqlInventoryRepository) GetStockByProductID(ctx context.Context, id int32) (sqlc.InventoryStock, error) {
	return r.q.GetStockByProductID(ctx, &id)
}

func (r *SqlInventoryRepository) GetStockByProductIDs(ctx context.Context, ids []int32) ([]sqlc.InventoryStock, error) {
	return r.q.GetStockByProductIDs(ctx, ids)
}

func (r *SqlInventoryRepository) UpsertStockMetadata(ctx context.Context, tx pgx.Tx, params sqlc.UpsertStockMetadataParams) (sqlc.InventoryStock, error) {
	return r.querier(tx).UpsertStockMetadata(ctx, params)
}

func (r *SqlInventoryRepository) DeductStockByUUID(ctx context.Context, tx pgx.Tx, productUUID uuid.UUID, quantity int32) (sqlc.InventoryStock, error) {
	return r.querier(tx).DeductStockByUUID(ctx, sqlc.DeductStockByUUIDParams{
		ProductUuid: productUUID,
		Quantity:    quantity,
	})
}

func (r *SqlInventoryRepository) DeductStockByID(ctx context.Context, tx pgx.Tx, productID int32, quantity int32) (sqlc.InventoryStock, error) {
	return r.querier(tx).DeductStockByID(ctx, sqlc.DeductStockByIDParams{
		ProductID: &productID,
		Quantity:  quantity,
	})
}

func (r *SqlInventoryRepository) RestoreStockByUUID(ctx context.Context, tx pgx.Tx, productUUID uuid.UUID, quantity int32) (sqlc.InventoryStock, error) {
	return r.querier(tx).RestoreStockByUUID(ctx, sqlc.RestoreStockByUUIDParams{
		ProductUuid: productUUID,
		Quantity:    quantity,
	})
}

func (r *SqlInventoryRepository) RestoreStockByID(ctx context.Context, tx pgx.Tx, productID int32, quantity int32) (sqlc.InventoryStock, error) {
	return r.querier(tx).RestoreStockByID(ctx, sqlc.RestoreStockByIDParams{
		ProductID: &productID,
		Quantity:  quantity,
	})
}

func (r *SqlInventoryRepository) AdjustStockByUUID(ctx context.Context, tx pgx.Tx, productUUID uuid.UUID, quantityChange int32) (sqlc.InventoryStock, error) {
	return r.querier(tx).AdjustStockByUUID(ctx, sqlc.AdjustStockByUUIDParams{
		ProductUuid: productUUID,
		Quantity:    quantityChange,
	})
}

func (r *SqlInventoryRepository) SetStockQuantity(ctx context.Context, tx pgx.Tx, productUUID uuid.UUID, quantity int32) (sqlc.InventoryStock, error) {
	return r.querier(tx).SetStockQuantity(ctx, sqlc.SetStockQuantityParams{
		ProductUuid: productUUID,
		Quantity:    quantity,
	})
}

func (r *SqlInventoryRepository) CreateStockMovement(ctx context.Context, tx pgx.Tx, params sqlc.CreateStockMovementParams) (sqlc.StockMovement, error) {
	return r.querier(tx).CreateStockMovement(ctx, params)
}

func (r *SqlInventoryRepository) GetMovementByRefAndProductUUID(ctx context.Context, refType, refID string, productUUID uuid.UUID) (sqlc.StockMovement, error) {
	return r.q.GetMovementByRefAndProductUUID(ctx, sqlc.GetMovementByRefAndProductUUIDParams{
		ReferenceType: &refType,
		ReferenceID:   &refID,
		ProductUuid:   productUUID,
	})
}

func (r *SqlInventoryRepository) GetMovementByRefAndProductID(ctx context.Context, refType, refID string, productID int32) (sqlc.StockMovement, error) {
	return r.q.GetMovementByRefAndProductID(ctx, sqlc.GetMovementByRefAndProductIDParams{
		ReferenceType: &refType,
		ReferenceID:   &refID,
		ProductID:     &productID,
	})
}

func (r *SqlInventoryRepository) ListStockMovements(ctx context.Context, params sqlc.ListStockMovementsParams) ([]sqlc.StockMovement, error) {
	return r.q.ListStockMovements(ctx, params)
}

func (r *SqlInventoryRepository) CountStockMovements(ctx context.Context, params sqlc.CountStockMovementsParams) (int64, error) {
	return r.q.CountStockMovements(ctx, params)
}
