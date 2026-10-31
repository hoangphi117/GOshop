package service

import (
	"context"
	"goshop/services/notification/internal/dto"

	"github.com/google/uuid"
)

type NotificationService interface {
	Send(ctx context.Context, input dto.SendNotificationInput) error
	CreateNotification(ctx context.Context, input dto.CreateNotificationInput) (dto.NotificationDTO, error)
	BroadcastNotification(ctx context.Context, input dto.BroadcastNotificationInput) error
	GetMyNotifications(ctx context.Context, userUUID uuid.UUID, query dto.GetNotificationsQuery) ([]dto.NotificationDTO, int32, error)
	GetUnreadNotificationCount(ctx context.Context, userUUID uuid.UUID) (dto.UnreadCountDTO, error)
	MarkNotificationAsRead(ctx context.Context, userUUID uuid.UUID, notificationUUID uuid.UUID) (dto.NotificationDTO, error)
	MarkAllNotificationsAsRead(ctx context.Context, userUUID uuid.UUID) error
	DeleteNotification(ctx context.Context, userUUID uuid.UUID, notificationUUID uuid.UUID) error

	// Admin
	NotifyAdmins(ctx context.Context, input dto.AdminNotifyInput) error
	GetAdminNotifications(ctx context.Context, userUUID uuid.UUID, query dto.AdminGetNotificationsQuery) ([]dto.NotificationDTO, int32, error)
	GetAdminUnreadNotificationCount(ctx context.Context, userUUID uuid.UUID) (dto.UnreadCountDTO, error)
	MarkAllAdminNotificationsAsRead(ctx context.Context, userUUID uuid.UUID) error
	PurgeStaleNotifications(ctx context.Context, retentionDays int) (int64, error)
}
