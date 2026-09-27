package v1dto

import (
	"time"

	"github.com/google/uuid"
)

// --- Requests ---

type SendMessageRequest struct {
	Content     string `json:"content"      binding:"required,max=4000"`
	ContentType string `json:"content_type" binding:"omitempty,oneof=text image file"`
}

type ListMessagesRequest struct {
	Page     int32 `form:"page"      binding:"omitempty,min=1"`
	Limit    int32 `form:"limit"     binding:"omitempty,min=1,max=100"`
	BeforeID int32 `form:"before_id" binding:"omitempty,min=0"` // Cursor-based pagination
}

type ListConversationsRequest struct {
	Status string `form:"status" binding:"omitempty,oneof=open closed pending"`
	Search string `form:"search" binding:"omitempty,max=100"`
	Page   int32  `form:"page"   binding:"omitempty,min=1"`
	Limit  int32  `form:"limit"  binding:"omitempty,min=1,max=100"`
}

// --- Responses ---

type ConversationResponse struct {
	ConversationID     int32      `json:"conversation_id"`
	UUID               uuid.UUID  `json:"uuid"`
	Status             string     `json:"status"`
	UserID             int32      `json:"user_id"`
	AdminID            *int32     `json:"admin_id,omitempty"`
	UserEmail          string     `json:"user_email"`
	UserFullname       string     `json:"user_fullname"`
	UserAvatar         string     `json:"user_avatar"`
	UnreadCount        int64      `json:"unread_count"`
	LastMessageContent string     `json:"last_message_content"`
	LastMessageAt      *time.Time `json:"last_message_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type MessageResponse struct {
	UUID        uuid.UUID `json:"uuid"`
	SenderID    int32     `json:"sender_id"`
	SenderRole  string    `json:"sender_role"`
	Content     string    `json:"content"`
	ContentType string    `json:"content_type"`
	IsRead      bool      `json:"is_read"`
	CreatedAt   time.Time `json:"created_at"`
}

type SendMessageResponse struct {
	Message MessageResponse `json:"message"`
}

type TotalUnreadCountResponse struct {
	UnreadCount int64 `json:"unread_count"`
}
