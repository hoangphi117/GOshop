package service

import (
	"context"

	chatdto "goshop/services/chat/internal/dto"

	"github.com/google/uuid"
)

// User is the identity projection the chat domain needs. Fetched over gRPC;
// the chat service never reads the identity database.
type User struct {
	UserID       int32
	UserUUID     uuid.UUID
	UserFullname string
	UserEmail    string
	// UserAvatar is the display URL. The avatar feature writes user_avatar_url;
	// user_avatar is a legacy column that predates it, so it is only a fallback.
	UserAvatar string
	UserStatus int32
	UserLevel  int32
}

// user_level values shared with the identity contract (1 = Admin, 2 = Moderator).
const (
	UserLevelAdmin     int32 = 1
	UserLevelModerator int32 = 2
)

// UserReader is the inbound identity port.
type UserReader interface {
	GetByUUID(ctx context.Context, userUUID uuid.UUID) (User, error)
	GetByID(ctx context.Context, id int32) (User, error)
}

// OrderReader is the inbound order port used to validate order_ref messages.
type OrderReader interface {
	GetOrderSummary(ctx context.Context, orderUUID uuid.UUID) (*OrderSummary, error)
}

// OrderSummary mirrors the order contract's cross-domain read result.
type OrderSummary struct {
	OrderUUID uuid.UUID
	UserID    int32
}

// PresenceReader is the inbound presence port. Presence state is owned by the
// WebSocket gateway and published to Redis; the chat domain only reads it.
type PresenceReader interface {
	IsUserOnline(ctx context.Context, userID int32) bool
	HasOnlineAdmin(ctx context.Context) bool
}

// WSPublisher is the outbound realtime port. Events are fanned out through the
// Redis pub/sub channels that the WS gateway's bridge subscribes to.
type WSPublisher interface {
	PublishToConversation(ctx context.Context, conversationID int32, msgType string, payload any)
}

// ChatService is the chat domain boundary exposed to the HTTP handlers.
type ChatService interface {
	GetOrCreateConversation(ctx context.Context, userUUID uuid.UUID) (*chatdto.ConversationDTO, error)
	SendMessage(ctx context.Context, conversationUUID uuid.UUID, senderUUID uuid.UUID, senderRole string, req chatdto.SendMessageRequest) (chatdto.MessageResponse, error)
	ListMessages(ctx context.Context, conversationUUID uuid.UUID, callerUUID uuid.UUID, req chatdto.ListMessagesRequest) ([]chatdto.MessageResponse, int64, error)
	MarkRead(ctx context.Context, conversationUUID uuid.UUID, callerUUID uuid.UUID) error
	ListConversations(ctx context.Context, callerUUID uuid.UUID, req chatdto.ListConversationsRequest) ([]chatdto.ConversationResponse, int64, error)
	GetConversationDetails(ctx context.Context, conversationUUID uuid.UUID, callerUUID uuid.UUID) (chatdto.ConversationResponse, error)
	CloseConversation(ctx context.Context, conversationUUID uuid.UUID) (*chatdto.ConversationDTO, error)
	ReopenConversation(ctx context.Context, conversationUUID uuid.UUID) (*chatdto.ConversationDTO, error)
	GetTotalUnreadCount(ctx context.Context, callerUUID uuid.UUID) (int64, error)
	SendTyping(ctx context.Context, conversationUUID uuid.UUID, callerUUID uuid.UUID, isTyping bool) error
	RecallMessage(ctx context.Context, messageUUID uuid.UUID, callerUUID uuid.UUID) (chatdto.MessageResponse, error)
	GetShopStatus(ctx context.Context) (chatdto.ShopStatusResponse, error)
}
