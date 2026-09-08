package repository

import (
	"context"
	"goshop/internal/db/sqlc"

	"github.com/jackc/pgx/v5"
)

type SqlOrderItemRepository struct {
	db sqlc.Querier
}

func NewSqlOrderItemRepository(db sqlc.Querier) OrderItemRepository {
	return &SqlOrderItemRepository{
		db: db,
	}
}

func (r *SqlOrderItemRepository) Create(ctx context.Context, tx pgx.Tx, input sqlc.CreateOrderItemParams) (sqlc.OrderItem, error) {

	q := sqlc.New(tx)

	orderItem, err := q.CreateOrderItem(ctx, input)
	if err != nil {
		return sqlc.OrderItem{}, err
	}

	return orderItem, nil
}

func (r *SqlOrderItemRepository) GetByOrderID(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
	orderItems, err := r.db.GetOrderItemsByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return orderItems, nil
}
