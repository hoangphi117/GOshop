package repository

import (
	"context"

	ordersqlc "goshop/services/order/internal/db/sqlc"

	"github.com/jackc/pgx/v5"
)

type SqlOrderItemRepository struct {
	db ordersqlc.Querier
}

func NewSqlOrderItemRepository(db ordersqlc.Querier) OrderItemRepository {
	return &SqlOrderItemRepository{
		db: db,
	}
}

func (r *SqlOrderItemRepository) Create(ctx context.Context, tx pgx.Tx, input ordersqlc.CreateOrderItemParams) (ordersqlc.OrderItem, error) {
	q := r.db
	if tx != nil {
		q = ordersqlc.New(tx)
	}
	return q.CreateOrderItem(ctx, input)
}

func (r *SqlOrderItemRepository) GetByOrderID(ctx context.Context, orderID int32) ([]ordersqlc.GetOrderItemsByOrderIDRow, error) {
	return r.db.GetOrderItemsByOrderID(ctx, orderID)
}
