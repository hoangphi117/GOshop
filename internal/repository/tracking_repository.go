package repository

import (
	"context"
	"goshop/internal/db/sqlc"

	"github.com/jackc/pgx/v5"
)

type SqlTrackingRepository struct {
	db sqlc.Querier
}

func NewSqlTrackingRepository(db sqlc.Querier) TrackingRepository {
	return &SqlTrackingRepository{
		db: db,
	}
}

func (r *SqlTrackingRepository) InsertEvent(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
	q := r.db
	if tx != nil {
		q = sqlc.New(tx)
	}

	return q.InsertTrackingEvent(ctx, params)
}

func (r *SqlTrackingRepository) GetByOrderID(ctx context.Context, orderID int32) ([]sqlc.OrderTrackingEvent, error) {
	return r.db.GetTrackingEventsByOrderID(ctx, orderID)
}
