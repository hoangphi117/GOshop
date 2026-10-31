package repository

import (
	"context"
	"time"

	"goshop/services/notification/internal/db/sqlc"

	"github.com/google/uuid"
)

type NotificationRepository interface {
	Create(ctx context.Context, params sqlc.CreateNotificationParams) (sqlc.Notification, error)
	GetByUserID(ctx context.Context, userID int32, limit, offset int32) ([]sqlc.Notification, error)
	CountByUserID(ctx context.Context, userID int32) (int64, error)
	CountUnreadByUserID(ctx context.Context, userID int32) (int64, error)
	MarkAsRead(ctx context.Context, notifUUID uuid.UUID, userID int32) (sqlc.Notification, error)
	MarkAllAsRead(ctx context.Context, userID int32) error
	Delete(ctx context.Context, notifUUID uuid.UUID, userID int32) error

	// Admin
	GetAdminNotifications(ctx context.Context, userID int32, nType *string, isRead *bool, limit, offset int32) ([]sqlc.Notification, error)
	CountAdminNotifications(ctx context.Context, userID int32, nType *string, isRead *bool) (int64, error)
	CountAdminUnreadNotifications(ctx context.Context, userID int32) (int64, error)
	MarkAllAdminAsRead(ctx context.Context, userID int32) error
	DeleteOldReadNotifications(ctx context.Context, cutoff time.Time) (int64, error)
}
