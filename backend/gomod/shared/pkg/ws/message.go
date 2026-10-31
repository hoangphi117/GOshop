package ws

import "time"

type EventType string

const (
	EventTypeNotification EventType = "notification"
	EventPing             EventType = "ping"
	EventPong             EventType = "pong"

	// Chat room events
	EventChatMessage         EventType = "chat.message"
	EventChatMessageRecalled EventType = "chat.message_recalled"
	EventChatRead            EventType = "chat.read"
	EventChatConvClosed      EventType = "chat.conv_closed"
	EventChatTyping          EventType = "chat.typing"
	EventChatPresence        EventType = "chat.presence"
)

type WSMessage struct {
	Type EventType `json:"type"`
	Data any       `json:"data"`
}

// ChatMessagePayload là data field trong WSMessage khi EventType = chat.message
type ChatMessagePayload struct {
	ConversationUUID string `json:"conversation_uuid"`
	MessageUUID      string `json:"message_uuid"`
	SenderID         int32  `json:"sender_id"`
	SenderRole       string `json:"sender_role"`
	Content          string `json:"content"`
	ContentType      string `json:"content_type"`

	// nil if text/order_red
	AttachmentURL  *string `json:"attachment_url,omitempty"`
	AttachmentName *string `json:"attachment_name,omitempty"`
	AttachmentSize *int64  `json:"attachment_size,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// ChatReadPayload là data field khi EventType = chat.read
type ChatReadPayload struct {
	ConversationUUID string `json:"conversation_uuid"`
	ReaderID         int32  `json:"reader_id"`
}

// ChatTypingPayload là data field khi EventType = chat.typing
type ChatTypingPayload struct {
	ConversationUUID string `json:"conversation_uuid"`
	UserID           int32  `json:"user_id"`
	SenderRole       string `json:"sender_role"`
	IsTyping         bool   `json:"is_typing"`
}

// ChatPresencePayload là data field khi EventType = chat.presence
type ChatPresencePayload struct {
	ConversationUUID string `json:"conversation_uuid,omitempty"`
	UserID           int32  `json:"user_id,omitempty"`
	Role             string `json:"role"` // "admin" or "user"
	IsOnline         bool   `json:"is_online"`
}
