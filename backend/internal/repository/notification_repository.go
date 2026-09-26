package repository

import (
	"context"
	"goshop/internal/db/sqlc"
	"time"

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
func (r *SqlNotificationRepository) CreateForAllUsers(ctx context.Context, nType, title, body string, data []byte) error {
	return r.db.CreateNotificationForAllUsers(ctx, sqlc.CreateNotificationForAllUsersParams{
		NotificationType:  nType,
		NotificationTitle: title,
		NotificationBody:  body,
		NotificationData:  data,
	})
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

// ── Admin ───────────────────────────────────────────────

func (r *SqlNotificationRepository) CreateForAdmins(ctx context.Context, nType, title, body string, data []byte) error {
	return r.db.CreateNotificationForAdmins(ctx, sqlc.CreateNotificationForAdminsParams{
		NotificationType:  nType,
		NotificationTitle: title,
		NotificationBody:  body,
		NotificationData:  data,
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
	count, err := r.db.CountAdminNotifications(ctx, sqlc.CountAdminNotificationsParams{
		UserID: userID,
		Type:   nType,
		IsRead: isRead,
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *SqlNotificationRepository) DeleteOldReadNotifications(ctx context.Context, cutoff time.Time) (int64, error) {
	return r.db.DeleteOldNotifications(ctx, cutoff)
}
