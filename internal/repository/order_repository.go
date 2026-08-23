package repository

import (
	"context"
	"goshop/internal/db/sqlc"

	"github.com/jackc/pgx/v5"
)

type SqlOrderRepository struct {
	db sqlc.Querier
}

func NewSqlOrderRepository(db sqlc.Querier) OrderRepository {
	return &SqlOrderRepository{
		db: db,
	}
}

func (r *SqlOrderRepository) Create(ctx context.Context, tx pgx.Tx, params sqlc.CreateOrderParams) (sqlc.Order, error) {

	q := sqlc.New(tx)

	order, err := q.CreateOrder(ctx, params)
	if err != nil {
		return sqlc.Order{}, err
	}

	return order, nil
}

func (r *SqlOrderRepository) GetLastOrderNumber(ctx context.Context, orderNumber string) (string, error) {
	lastOrderNumber, err := r.db.GetLatestOrderNumber(ctx, orderNumber)
	if err != nil {
		return "", err
	}
	return lastOrderNumber, nil
}
