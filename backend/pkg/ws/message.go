package ws

import "time"

type EventType string

const (
	EventTypeNotification EventType = "notification"
	EventPing             EventType = "ping"
	EventPong             EventType = "pong"

	// Chat room events
	EventChatMessage    EventType = "chat.message"
	EventChatRead       EventType = "chat.read"
	EventChatConvClosed EventType = "chat.conv_closed"
	EventChatTyping     EventType = "chat.typing"
)

type WSMessage struct {
	Type EventType `json:"type"`
	Data any       `json:"data"`
}

// ChatMessagePayload là data field trong WSMessage khi EventType = chat.message
type ChatMessagePayload struct {
	ConversationUUID string    `json:"conversation_uuid"`
	MessageUUID      string    `json:"message_uuid"`
	SenderID         int32     `json:"sender_id"`
	SenderRole       string    `json:"sender_role"`
	Content          string    `json:"content"`
	ContentType      string    `json:"content_type"`
	CreatedAt        time.Time `json:"created_at"`
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
