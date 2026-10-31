package repository

import (
	"context"
	"time"

	"goshop/services/notification/internal/db/sqlc"

	"github.com/google/uuid"
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
	return r.db.CountNotificationsByUserID(ctx, userID)
}

func (r *SqlNotificationRepository) CountUnreadByUserID(ctx context.Context, userID int32) (int64, error) {
	return r.db.CountUnreadNotificationsByUserID(ctx, userID)
}

func (r *SqlNotificationRepository) MarkAsRead(ctx context.Context, notifUUID uuid.UUID, userID int32) (sqlc.Notification, error) {
	return r.db.MarkNotificationRead(ctx, sqlc.MarkNotificationReadParams{
		NotificationUuid: notifUUID,
		UserID:           userID,
	})
}

func (r *SqlNotificationRepository) MarkAllAsRead(ctx context.Context, userID int32) error {
	return r.db.MarkAllNotificationsRead(ctx, userID)
}

func (r *SqlNotificationRepository) Delete(ctx context.Context, notifUUID uuid.UUID, userID int32) error {
	return r.db.DeleteNotification(ctx, sqlc.DeleteNotificationParams{
		NotificationUuid: notifUUID,
		UserID:           userID,
	})
}

func (r *SqlNotificationRepository) GetAdminNotifications(ctx context.Context, userID int32, nType *string, isRead *bool, limit, offset int32) ([]sqlc.Notification, error) {
	return r.db.GetAdminNotifications(ctx, sqlc.GetAdminNotificationsParams{
		UserID: userID,
		Type:   nType,
		IsRead: isRead,
		Limit:  limit,
		Offset: offset,
	})
}

func (r *SqlNotificationRepository) CountAdminNotifications(ctx context.Context, userID int32, nType *string, isRead *bool) (int64, error) {
	return r.db.CountAdminNotifications(ctx, sqlc.CountAdminNotificationsParams{
		UserID: userID,
		Type:   nType,
		IsRead: isRead,
	})
}

func (r *SqlNotificationRepository) CountAdminUnreadNotifications(ctx context.Context, userID int32) (int64, error) {
	isReadFalse := false
	return r.db.CountAdminNotifications(ctx, sqlc.CountAdminNotificationsParams{
		UserID: userID,
		IsRead: &isReadFalse,
	})
}

func (r *SqlNotificationRepository) MarkAllAdminAsRead(ctx context.Context, userID int32) error {
	return r.db.MarkAllAdminNotificationsRead(ctx, userID)
}

func (r *SqlNotificationRepository) DeleteOldReadNotifications(ctx context.Context, cutoff time.Time) (int64, error) {
	return r.db.DeleteOldNotifications(ctx, cutoff)
}
