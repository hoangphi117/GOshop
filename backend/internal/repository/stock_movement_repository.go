package repository

import (
	"context"
	"goshop/internal/db/sqlc"

	"github.com/jackc/pgx/v5"
)

type SqlStockMovementRepository struct {
	db sqlc.Querier
}

func NewSqlStockMovementRepository(db sqlc.Querier) StockMovementRepository {
	return &SqlStockMovementRepository{
		db: db,
	}
}

func (r *SqlStockMovementRepository) Create(ctx context.Context, tx pgx.Tx, params sqlc.CreateStockMovementParams) (sqlc.StockMovement, error) {
	q := r.db
	if tx != nil {
		q = sqlc.New(tx)
	}
	return q.CreateStockMovement(ctx, params)
}

func (r *SqlStockMovementRepository) List(ctx context.Context, params sqlc.ListStockMovementsParams) ([]sqlc.ListStockMovementsRow, error) {
	return r.db.ListStockMovements(ctx, params)
}

func (r *SqlStockMovementRepository) Count(ctx context.Context, params sqlc.CountStockMovementsParams) (int64, error) {
	return r.db.CountStockMovements(ctx, params)
}
