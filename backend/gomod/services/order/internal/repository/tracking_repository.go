package repository

import (
	"context"

	ordersqlc "goshop/services/order/internal/db/sqlc"

	"github.com/jackc/pgx/v5"
)

type SqlTrackingRepository struct {
	db ordersqlc.Querier
}

func NewSqlTrackingRepository(db ordersqlc.Querier) TrackingRepository {
	return &SqlTrackingRepository{
		db: db,
	}
}

func (r *SqlTrackingRepository) InsertEvent(ctx context.Context, tx pgx.Tx, params ordersqlc.InsertTrackingEventParams) (ordersqlc.OrderTrackingEvent, error) {
	q := r.db
	if tx != nil {
		q = ordersqlc.New(tx)
	}
	return q.InsertTrackingEvent(ctx, params)
}

func (r *SqlTrackingRepository) GetByOrderID(ctx context.Context, orderID int32) ([]ordersqlc.OrderTrackingEvent, error) {
	return r.db.GetTrackingEventsByOrderID(ctx, orderID)
}
