package dto

import (
	"time"

	"goshop/services/chat/internal/db/sqlc"

	"github.com/google/uuid"
)

// --- Requests ---

type SendMessageRequest struct {
	// Content is empty for attachment-only messages; the service enforces
	// "content or attachment" once it knows the content type.
	Content     string `json:"content"      binding:"max=4000"`
	ContentType string `json:"content_type" binding:"omitempty,oneof=text image video order_ref product_ref media_group"`

	// Attachment fields (optional, set by FE after upload)
	AttachmentURL  string `json:"attachment_url"  binding:"omitempty,max=8000"`
	AttachmentName string `json:"attachment_name" binding:"omitempty,max=2000"`
	AttachmentSize int64  `json:"attachment_size" binding:"omitempty,min=0"`

	// References (optional, used when content_type = 'order_ref' or 'product_ref')
	OrderUUID   string `json:"order_uuid"   binding:"omitempty,uuid"`
	ProductUUID string `json:"product_uuid" binding:"omitempty,uuid"`
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

type SendTypingRequest struct {
	IsTyping bool `json:"is_typing"`
}

// --- Responses ---

type ConversationDTO struct {
	ConversationID   int32      `json:"conversation_id"`
	ConversationUuid uuid.UUID  `json:"conversation_uuid"`
	UUID             uuid.UUID  `json:"uuid"`
	UserID           int32      `json:"user_id"`
	AdminID          *int32     `json:"admin_id,omitempty"`
	Status           string     `json:"status"`
	LastMessageAt    *time.Time `json:"last_message_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

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
	IsOnline           bool       `json:"is_online"`
	IsShopOnline       bool       `json:"is_shop_online"`
}

type ShopStatusResponse struct {
	IsOnline bool `json:"is_online"`
}

type MessageResponse struct {
	UUID           uuid.UUID `json:"uuid"`
	SenderID       int32     `json:"sender_id"`
	SenderRole     string    `json:"sender_role"`
	Content        string    `json:"content"`
	ContentType    string    `json:"content_type"`
	AttachmentURL  string    `json:"attachment_url,omitempty"`
	AttachmentName string    `json:"attachment_name,omitempty"`
	AttachmentSize int64     `json:"attachment_size,omitempty"`
	IsRead         bool      `json:"is_read"`
	CreatedAt      time.Time `json:"created_at"`
}

type SendMessageResponse struct {
	Message MessageResponse `json:"message"`
}

type TotalUnreadCountResponse struct {
	UnreadCount int64 `json:"unread_count"`
}

func MapChatMessageToResponse(m sqlc.ChatMessage) MessageResponse {
	resp := MessageResponse{
		UUID:        m.MessageUuid,
		SenderID:    m.SenderID,
		SenderRole:  m.SenderRole,
		Content:     m.Content,
		ContentType: m.ContentType,
		IsRead:      m.IsRead,
		CreatedAt:   m.CreatedAt,
	}

	if m.AttachmentUrl != nil {
		resp.AttachmentURL = *m.AttachmentUrl
	}
	if m.AttachmentName != nil {
		resp.AttachmentName = *m.AttachmentName
	}
	if m.AttachmentSize != nil {
		resp.AttachmentSize = *m.AttachmentSize
	}

	return resp
}

func MapChatConversationToDTO(conv sqlc.ChatConversation) *ConversationDTO {
	var lastMsgAt *time.Time
	if conv.LastMessageAt.Valid {
		t := conv.LastMessageAt.Time
		lastMsgAt = &t
	}
	return &ConversationDTO{
		ConversationID:   conv.ConversationID,
		ConversationUuid: conv.ConversationUuid,
		UUID:             conv.ConversationUuid,
		UserID:           conv.UserID,
		AdminID:          conv.AdminID,
		Status:           conv.Status,
		LastMessageAt:    lastMsgAt,
		CreatedAt:        conv.CreatedAt,
		UpdatedAt:        conv.UpdatedAt,
	}
}
