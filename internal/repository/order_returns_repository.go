package repository

import (
	"context"
	"goshop/internal/db/sqlc"

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
