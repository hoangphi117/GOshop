package v1dto

import (
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
