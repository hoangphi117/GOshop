package repository

import (
	"context"
	"goshop/internal/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type SqlOrderReturnsRepository struct {
	db sqlc.Querier
}

func NewSqlOrderReturnsRepository(db sqlc.Querier) OrderReturnsRepository {
	return &SqlOrderReturnsRepository{
		db: db,
	}
}

func (r *SqlOrderReturnsRepository) Create(ctx context.Context, tx pgx.Tx, params sqlc.CreateOrderReturnParams) (sqlc.OrderReturn, error) {
	q := sqlc.New(tx)
	returnOrder, err := q.CreateOrderReturn(ctx, params)
	if err != nil {
		return sqlc.OrderReturn{}, err
	}
	return returnOrder, nil
}

func (r *SqlOrderReturnsRepository) GetActiveByOrderID(ctx context.Context, orderID int32) (sqlc.OrderReturn, error) {
	returnOrder, err := r.db.GetActiveReturnByOrderID(ctx, orderID)
	if err != nil {
		return sqlc.OrderReturn{}, err
	}
	return returnOrder, nil
}

func (r *SqlOrderReturnsRepository) GetReturns(ctx context.Context, params sqlc.AdminGetReturnsParams) ([]sqlc.AdminGetReturnsRow, error) {
	returns, err := r.db.AdminGetReturns(ctx, params)
	if err != nil {
		return nil, err
	}
	return returns, nil
}

func (r *SqlOrderReturnsRepository) GetByUUID(ctx context.Context, returnUUID uuid.UUID) (sqlc.OrderReturn, error) {
	returnOrder, err := r.db.GetReturnByUUID(ctx, returnUUID)
	if err != nil {
		return sqlc.OrderReturn{}, err
	}
	return returnOrder, nil
}

func (r *SqlOrderReturnsRepository) CountReturns(ctx context.Context, params sqlc.AdminCountReturnsParams) (int64, error) {
	count, err := r.db.AdminCountReturns(ctx, params)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *SqlOrderReturnsRepository) UpdateStatus(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnStatusParams) (sqlc.OrderReturn, error) {
	q := sqlc.New(tx)
	returnOrder, err := q.UpdateReturnStatus(ctx, params)
	if err != nil {
		return sqlc.OrderReturn{}, err
	}
	return returnOrder, nil
}
