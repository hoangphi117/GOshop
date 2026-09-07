package repository

import (
	"context"
	"goshop/internal/db/sqlc"
)

type SqlNotificationRepository struct {
	db sqlc.Querier
}

func NewSqlNotificationRepository(db sqlc.Querier) NotificationRepository {
	return &SqlNotificationRepository{db: db}
}

func (r *SqlNotificationRepository) Create(ctx context.Context, params sqlc.CreateNotificationParams) (sqlc.Notification, error) {
	return r.db.CreateNotification(ctx, params)
}
