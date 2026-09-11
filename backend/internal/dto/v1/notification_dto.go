package v1dto

import (
	"encoding/json"
	"goshop/internal/db/sqlc"
	"time"

	"github.com/google/uuid"
)

// ── DTO / Response ───────────────────────────────────────────────

type NotificationDTO struct {
	UUID      uuid.UUID              `json:"uuid"`
	Type      string                 `json:"type"`
	Title     string                 `json:"title"`
	Body      string                 `json:"body"`
	Data      map[string]interface{} `json:"data,omitempty"`
	IsRead    bool                   `json:"is_read"`
	ReadAt    *time.Time             `json:"read_at,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

type UnreadCountDTO struct {
	UnreadCount int64 `json:"unread_count"`
}

// ── Input ───────────────────────────────────────────────

type CreateNotificationInput struct {
	UserUUID uuid.UUID              `json:"user_uuid" binding:"required"`
	Type     string                 `json:"type" binding:"required,max=50"`
	Title    string                 `json:"title" binding:"required,max=255"`
	Body     string                 `json:"body" binding:"required"`
	Data     map[string]interface{} `json:"data,omitempty"`
}

type SendNotificationInput struct {
	UserID int32
	Type   string
	Title  string
	Body   string
	Data   map[string]any
}

type BroadcastNotificationInput struct {
	Type  string                 `json:"type" binding:"required,max=50"`
	Title string                 `json:"title" binding:"required,max=255"`
	Body  string                 `json:"body" binding:"required"`
	Data  map[string]interface{} `json:"data,omitempty"`
}

type GetNotificationsQuery struct {
	Page  int32 `form:"page" binding:"omitempty,min=1"`
	Limit int32 `form:"limit" binding:"omitempty,min=1,max=100"`
}

type NotificationUUID struct {
	UUID string `uri:"uuid" binding:"required,uuid"`
}

// ── Helper ───────────────────────────────────────────────

func MapToNotificationDTO(n sqlc.Notification) NotificationDTO {
	var responseData map[string]any
	if len(n.NotificationData) > 0 {
		_ = json.Unmarshal(n.NotificationData, &responseData)
	}
	var readAt *time.Time
	if n.NotificationReadAt.Valid {
		readAt = &n.NotificationReadAt.Time
	}
	return NotificationDTO{
		UUID:      n.NotificationUuid,
		Type:      n.NotificationType,
		Title:     n.NotificationTitle,
		Body:      n.NotificationBody,
		Data:      responseData,
		IsRead:    n.NotificationIsRead,
		ReadAt:    readAt,
		CreatedAt: n.NotificationCreatedAt,
	}
}
