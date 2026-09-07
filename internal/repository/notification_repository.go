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

func (r *SqlNotificationRepository) GetByUserID(ctx context.Context, userID int32, limit, offset int32) ([]sqlc.Notification, error) {
	return r.db.GetNotificationsByUserId(ctx, sqlc.GetNotificationsByUserIdParams{
		NotificationUserID: userID,
		Limit:              limit,
		Offset:             offset,
	})
}

func (r *SqlNotificationRepository) CountByUserID(ctx context.Context, userID int32) (int64, error) {
	count, err := r.db.CountNotificationsByUserID(ctx, userID)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *SqlNotificationRepository) CountUnreadByUserID(ctx context.Context, userID int32) (int64, error) {
	count, err := r.db.CountUnreadNotificationsByUserID(ctx, userID)
	if err != nil {
		return 0, err
	}
	return count, nil
}
