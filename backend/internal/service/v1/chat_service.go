package v1service

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/logger"
	"goshop/pkg/ws"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type chatService struct {
	chatRepo    repository.ChatRepository
	userRepo    repository.UserRepository
	orderRepo   repository.OrderRepository
	rdb         *redis.Client
	redisBridge ws.RedisBridge
	wsHub       ws.Hub // Fallback local mode if Redis is not available
	txManager   db.TxManager
	logger      zerolog.Logger
}

func NewChatService(
	chatRepo repository.ChatRepository,
	userRepo repository.UserRepository,
	orderRepo repository.OrderRepository,
	txManager db.TxManager,
	rdb *redis.Client,
) ChatService {
	return &chatService{
		chatRepo:  chatRepo,
		userRepo:  userRepo,
		orderRepo: orderRepo,
		txManager: txManager,
		rdb:       rdb,
		logger:    *logger.Log,
	}
}

// -------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------

// SetRedisBridge supports DI injection for horizontal scaling bridge
func (s *chatService) SetRedisBridge(bridge ws.RedisBridge) {
	s.redisBridge = bridge
}

// SetWSHub supports DI injection for local in-memory hub
func (s *chatService) SetWSHub(wsHub ws.Hub) {
	s.wsHub = wsHub
}

func (s *chatService) publishToConversation(conversationID int32, msg ws.WSMessage) {
	if s.redisBridge != nil {
		go func() {
			if err := s.redisBridge.PublishToConversation(context.Background(), conversationID, msg); err != nil {
				s.logger.Error().
					Err(err).
					Int32("conversation_id", conversationID).
					Str("event_type", string(msg.Type)).
					Msg("Failed to publish chat event to Redis bridge")
			}
		}()
		return
	}

	// Fallback local memory hub
	if s.wsHub != nil {
		s.wsHub.BroadcastToConversation(conversationID, msg)
	}
}

func (s *chatService) getConversation(ctx context.Context, convUUID uuid.UUID) (sqlc.ChatConversation, error) {
	conv, err := s.chatRepo.GetConversationByUUID(ctx, convUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.ChatConversation{}, utils.NewError(utils.ErrCodeNotFound, "conversation not found")
		}
		return sqlc.ChatConversation{}, utils.WrapError(utils.ErrCodeInternal, "failed to get conversation", err)
	}
	return conv, nil
}

func (s *chatService) determineSenderRole(user sqlc.User) string {
	// user_level: 1 = Admin, 2 = Moderator, 3 = Member
	if user.UserLevel == 1 || user.UserLevel == 2 {
		return "admin"
	}
	return "user"
}

func (s *chatService) getActiveUser(ctx context.Context, userUUID uuid.UUID) (sqlc.User, error) {
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.User{}, utils.NewError(utils.ErrCodeNotFound, "user not found")
		}
		return sqlc.User{}, utils.WrapError(utils.ErrCodeInternal, "failed to get user", err)
	}

	if user.UserStatus != 1 { // 1: Active
		return sqlc.User{}, utils.NewError(utils.ErrCodeForbidden, "user account is not active")
	}
	return user, nil
}

func (s *chatService) GetOrCreateConversation(ctx context.Context, userUUID uuid.UUID) (sqlc.ChatConversation, error) {
	user, err := s.getActiveUser(ctx, userUUID)
	if err != nil {
		return sqlc.ChatConversation{}, err
	}

	conversation, err := s.chatRepo.GetConversationByUserID(ctx, user.UserID)
	if err == nil {
		return conversation, nil
	}

	if !errors.Is(err, sql.ErrNoRows) && !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.ChatConversation{}, utils.WrapError(utils.ErrCodeInternal, "failed to get conversation", err)
	}

	newConv, err := s.chatRepo.CreateConversation(ctx, user.UserID)
	if err != nil {
		return sqlc.ChatConversation{}, utils.WrapError(utils.ErrCodeInternal, "failed to create conversation", err)
	}

	return newConv, nil
}

func (s *chatService) SendMessage(
	ctx context.Context,
	conversationUUID uuid.UUID,
	senderUUID uuid.UUID,
	senderRole string,
	req v1dto.SendMessageRequest,
) (v1dto.MessageResponse, error) {
	sender, err := s.getActiveUser(ctx, senderUUID)
	if err != nil {
		return v1dto.MessageResponse{}, err
	}

	conv, err := s.getConversation(ctx, conversationUUID)
	if err != nil {
		return v1dto.MessageResponse{}, err
	}

	if conv.Status == "closed" {
		_, _ = s.chatRepo.UpdateConversationStatus(ctx, conv.ConversationID, "open")
	}

	computedRole := "user"
	if senderRole == "admin" || (conv.UserID != sender.UserID && (sender.UserLevel == 1 || sender.UserLevel == 2)) {
		computedRole = "admin"
	} else if conv.UserID == sender.UserID {
		computedRole = "user"
	}

	if computedRole == "user" && conv.UserID != sender.UserID {
		return v1dto.MessageResponse{}, utils.NewError(utils.ErrCodeForbidden, "user is not part of this conversation")
	}

	contentType := strings.TrimSpace(req.ContentType)
	if contentType == "" {
		if req.AttachmentURL != "" {
			// Infer from URL extension
			ext := strings.ToLower(filepath.Ext(req.AttachmentURL))
			if ext == ".mp4" || ext == ".mov" || ext == ".avi" || ext == ".webm" {
				contentType = "video"
			} else {
				contentType = "image"
			}
		} else if req.OrderUUID != "" {
			contentType = "order_ref"
		} else if req.ProductUUID != "" {
			contentType = "product_ref"
		} else {
			contentType = "text"
		}
	}

	content := strings.TrimSpace(req.Content)
	if contentType == "order_ref" {
		if req.OrderUUID == "" {
			return v1dto.MessageResponse{}, utils.NewError(utils.ErrCodeBadRequest, "order_uuid is required for order_ref messages")
		}
		if computedRole == "user" {
			orderUUID, err := uuid.Parse(req.OrderUUID)
			if err != nil {
				return v1dto.MessageResponse{}, utils.WrapError(utils.ErrCodeBadRequest, "invalid order UUID", err)
			}
			if s.orderRepo == nil {
				return v1dto.MessageResponse{}, utils.NewError(utils.ErrCodeInternal, "order repository not configured")
			}
			order, err := s.orderRepo.GetByUUID(ctx, orderUUID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
					return v1dto.MessageResponse{}, utils.NewError(utils.ErrCodeNotFound, "order not found")
				}
				return v1dto.MessageResponse{}, utils.WrapError(utils.ErrCodeInternal, "failed to get order", err)
			}
			if order.OrderUserID != sender.UserID {
				return v1dto.MessageResponse{}, utils.NewError(utils.ErrCodeForbidden, "user is not authorized to reference this order")
			}
		}
		content = req.OrderUUID
	} else if contentType == "product_ref" {
		if req.ProductUUID != "" {
			content = req.ProductUUID
		}
	}

	if contentType != "order_ref" && contentType != "product_ref" && content == "" && req.AttachmentURL == "" {
		return v1dto.MessageResponse{}, utils.NewError(utils.ErrCodeBadRequest, "content or attachment is required for non-reference messages")
	}

	var attachURL, attachName *string
	var attachSize *int64
	if req.AttachmentURL != "" {
		attachURL = &req.AttachmentURL
	}
	if req.AttachmentName != "" {
		attachName = &req.AttachmentName
	}
	if req.AttachmentSize > 0 {
		attachSize = &req.AttachmentSize
	}

	var msg sqlc.ChatMessage
	if s.txManager != nil {
		err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
			var txErr error
			msg, txErr = s.chatRepo.CreateMessage(txCtx, sqlc.CreateMessageParams{
				ConversationID: conv.ConversationID,
				SenderID:       sender.UserID,
				SenderRole:     computedRole,
				Content:        content,
				ContentType:    contentType,
				AttachmentUrl:  attachURL,
				AttachmentName: attachName,
				AttachmentSize: attachSize,
			})
			if txErr != nil {
				return utils.WrapError(utils.ErrCodeInternal, "failed to save chat message", txErr)
			}

			if txErr = s.chatRepo.UpdateLastMessageAt(txCtx, conv.ConversationID); txErr != nil {
				return utils.WrapError(utils.ErrCodeInternal, "failed to update conversation timestamp", txErr)
			}
			return nil
		})
	} else {
		msg, err = s.chatRepo.CreateMessage(ctx, sqlc.CreateMessageParams{
			ConversationID: conv.ConversationID,
			SenderID:       sender.UserID,
			SenderRole:     computedRole,
			Content:        content,
			ContentType:    contentType,
			AttachmentUrl:  attachURL,
			AttachmentName: attachName,
			AttachmentSize: attachSize,
		})
		if err == nil {
			_ = s.chatRepo.UpdateLastMessageAt(ctx, conv.ConversationID)
		}
	}

	if err != nil {
		return v1dto.MessageResponse{}, utils.WrapError(utils.ErrCodeInternal, "failed to save chat message", err)
	}

	// Publish the new message realtime event to the conversation
	s.publishToConversation(conv.ConversationID, ws.WSMessage{
		Type: ws.EventChatMessage,
		Data: ws.ChatMessagePayload{
			ConversationUUID: conv.ConversationUuid.String(),
			MessageUUID:      msg.MessageUuid.String(),
			SenderID:         msg.SenderID,
			SenderRole:       msg.SenderRole,
			Content:          msg.Content,
			ContentType:      msg.ContentType,
			AttachmentURL:    msg.AttachmentUrl,
			AttachmentName:   msg.AttachmentName,
			AttachmentSize:   msg.AttachmentSize,
			CreatedAt:        msg.CreatedAt,
		},
	})

	return v1dto.MapChatMessageToResponse(msg), nil
}

func (s *chatService) ListMessages(
	ctx context.Context,
	conversationUUID uuid.UUID,
	callerUUID uuid.UUID,
	req v1dto.ListMessagesRequest,
) ([]v1dto.MessageResponse, int64, error) {
	caller, err := s.getActiveUser(ctx, callerUUID)
	if err != nil {
		return nil, 0, err
	}

	conv, err := s.getConversation(ctx, conversationUUID)
	if err != nil {
		return nil, 0, err
	}

	role := s.determineSenderRole(caller)
	if role == "user" && conv.UserID != caller.UserID {
		return nil, 0, utils.NewError(utils.ErrCodeForbidden, "you are not authorized to view messages in this conversation")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	var messages []sqlc.ChatMessage
	if req.BeforeID > 0 {
		messages, err = s.chatRepo.ListMessagesCursor(ctx, sqlc.ListMessagesCursorParams{
			ConversationID: conv.ConversationID,
			BeforeID:       req.BeforeID,
			LimitCount:     limit,
		})
	} else {
		messages, err = s.chatRepo.ListMessages(ctx, sqlc.ListMessagesParams{
			ConversationID: conv.ConversationID,
			LimitCount:     limit,
			OffsetCount:    offset,
		})
	}
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "failed to list messages", err)
	}

	total, err := s.chatRepo.CountMessagesByConversationID(ctx, conv.ConversationID)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "failed to count messages", err)
	}

	res := make([]v1dto.MessageResponse, len(messages))
	for i, m := range messages {

		res[i] = v1dto.MapChatMessageToResponse(m)
	}

	return res, total, nil
}

func (s *chatService) MarkRead(
	ctx context.Context,
	conversationUUID uuid.UUID,
	callerUUID uuid.UUID,
) error {
	caller, err := s.getActiveUser(ctx, callerUUID)
	if err != nil {
		return err
	}

	conv, err := s.getConversation(ctx, conversationUUID)
	if err != nil {
		return err
	}

	role := s.determineSenderRole(caller)
	if role == "user" && conv.UserID != caller.UserID {
		return utils.NewError(utils.ErrCodeForbidden, "you cannot mark messages as read in this conversation")
	}

	if err := s.chatRepo.MarkMessagesRead(ctx, conv.ConversationID, caller.UserID); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "failed to mark messages as read", err)
	}

	// Publish the read realtime event to the conversation
	s.publishToConversation(conv.ConversationID, ws.WSMessage{
		Type: ws.EventChatRead,
		Data: ws.ChatReadPayload{
			ConversationUUID: conv.ConversationUuid.String(),
			ReaderID:         caller.UserID,
		},
	})

	return nil
}

func (s *chatService) ListConversations(
	ctx context.Context,
	callerUUID uuid.UUID,
	req v1dto.ListConversationsRequest,
) ([]v1dto.ConversationResponse, int64, error) {
	caller, err := s.getActiveUser(ctx, callerUUID)
	if err != nil {
		return nil, 0, err
	}

	if s.determineSenderRole(caller) != "admin" {
		return nil, 0, utils.NewError(utils.ErrCodeForbidden, "access denied: admin privileges required")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	search := strings.TrimSpace(req.Search)
	status := strings.TrimSpace(req.Status)
	if status == "active" {
		status = "open"
	} else if status == "archived" {
		status = "closed"
	}

	rows, err := s.chatRepo.ListConversations(ctx, sqlc.ListConversationsParams{
		Status:      status,
		Search:      search,
		LimitCount:  limit,
		OffsetCount: offset,
	})
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "failed to list conversations", err)
	}

	total, err := s.chatRepo.CountConversations(ctx, sqlc.CountConversationsParams{
		Status: status,
		Search: search,
	})
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "failed to count conversations", err)
	}

	res := make([]v1dto.ConversationResponse, len(rows))
	for i, r := range rows {
		var lastMsgAt *time.Time
		if r.LastMessageAt.Valid {
			lastMsgAt = &r.LastMessageAt.Time
		}

		isCustomerOnline := false
		isShopOnline := false
		if s.wsHub != nil {
			isCustomerOnline = s.wsHub.IsUserOnline(r.UserID)
			isShopOnline = s.wsHub.HasOnlineAdmin()
		}

		res[i] = v1dto.ConversationResponse{
			ConversationID:     r.ConversationID,
			UUID:               r.ConversationUuid,
			Status:             r.Status,
			UserID:             r.UserID,
			AdminID:            r.AdminID,
			UserEmail:          r.UserEmail,
			UserFullname:       r.UserFullname,
			UserAvatar:         r.UserAvatar,
			UnreadCount:        r.UnreadCount,
			LastMessageContent: r.LastMessageContent,
			LastMessageAt:      lastMsgAt,
			CreatedAt:          r.CreatedAt,
			UpdatedAt:          r.UpdatedAt,
			IsOnline:           isCustomerOnline,
			IsShopOnline:       isShopOnline,
		}
	}

	return res, total, nil
}

func (s *chatService) CloseConversation(
	ctx context.Context,
	conversationUUID uuid.UUID,
) (sqlc.ChatConversation, error) {
	conv, err := s.getConversation(ctx, conversationUUID)
	if err != nil {
		return sqlc.ChatConversation{}, err
	}

	if conv.Status == "closed" {
		return conv, nil
	}

	updatedConv, err := s.chatRepo.UpdateConversationStatus(ctx, conv.ConversationID, "closed")
	if err != nil {
		return sqlc.ChatConversation{}, utils.WrapError(utils.ErrCodeInternal, "failed to close conversation", err)
	}

	// Publish the conversation closed event to the conversation
	s.publishToConversation(conv.ConversationID, ws.WSMessage{
		Type: ws.EventChatConvClosed,
		Data: map[string]any{
			"conversation_uuid": conv.ConversationUuid.String(),
		},
	})

	return updatedConv, nil
}

func (s *chatService) ReopenConversation(
	ctx context.Context,
	conversationUUID uuid.UUID,
) (sqlc.ChatConversation, error) {
	conv, err := s.getConversation(ctx, conversationUUID)
	if err != nil {
		return sqlc.ChatConversation{}, err
	}

	if conv.Status == "open" {
		return conv, nil
	}

	return s.chatRepo.UpdateConversationStatus(ctx, conv.ConversationID, "open")
}

func (s *chatService) GetConversationDetails(
	ctx context.Context,
	conversationUUID uuid.UUID,
	callerUUID uuid.UUID,
) (v1dto.ConversationResponse, error) {
	caller, err := s.getActiveUser(ctx, callerUUID)
	if err != nil {
		return v1dto.ConversationResponse{}, err
	}

	conv, err := s.getConversation(ctx, conversationUUID)
	if err != nil {
		return v1dto.ConversationResponse{}, err
	}

	role := s.determineSenderRole(caller)
	if role == "user" && conv.UserID != caller.UserID {
		return v1dto.ConversationResponse{}, utils.NewError(utils.ErrCodeForbidden, "you are not authorized to view this conversation")
	}

	convOwner, _ := s.userRepo.GetByUUID(ctx, callerUUID)
	unreadCount, _ := s.chatRepo.CountUnreadMessages(ctx, conv.ConversationID, caller.UserID)

	var lastMsgAt *time.Time
	if conv.LastMessageAt.Valid {
		lastMsgAt = &conv.LastMessageAt.Time
	}

	isCustOnline := false
	isShopOnline := false
	if s.wsHub != nil {
		isCustOnline = s.wsHub.IsUserOnline(conv.UserID)
		isShopOnline = s.wsHub.HasOnlineAdmin()
	}

	return v1dto.ConversationResponse{
		ConversationID: conv.ConversationID,
		UUID:           conv.ConversationUuid,
		Status:         conv.Status,
		UserID:         conv.UserID,
		AdminID:        conv.AdminID,
		UserEmail:      convOwner.UserEmail,
		UserFullname:   convOwner.UserFullname,
		UserAvatar:     convOwner.UserAvatar,
		UnreadCount:    unreadCount,
		LastMessageAt:  lastMsgAt,
		CreatedAt:      conv.CreatedAt,
		UpdatedAt:      conv.UpdatedAt,
		IsOnline:       isCustOnline,
		IsShopOnline:   isShopOnline,
	}, nil
}

func (s *chatService) GetTotalUnreadCount(
	ctx context.Context,
	callerUUID uuid.UUID,
) (int64, error) {
	caller, err := s.getActiveUser(ctx, callerUUID)
	if err != nil {
		return 0, err
	}

	var count int64
	role := s.determineSenderRole(caller)
	if role == "admin" {
		count, err = s.chatRepo.GetTotalUnreadMessagesForAdmin(ctx)
	} else {
		count, err = s.chatRepo.GetTotalUnreadMessagesForUser(ctx, caller.UserID)
	}

	if err != nil {
		return 0, utils.WrapError(utils.ErrCodeInternal, "failed to count total unread messages", err)
	}

	return count, nil
}

func (s *chatService) SendTyping(
	ctx context.Context,
	conversationUUID uuid.UUID,
	callerUUID uuid.UUID,
	isTyping bool,
) error {
	caller, err := s.getActiveUser(ctx, callerUUID)
	if err != nil {
		return err
	}

	conv, err := s.getConversation(ctx, conversationUUID)
	if err != nil {
		return err
	}

	role := "user"
	if conv.UserID != caller.UserID && (caller.UserLevel == 1 || caller.UserLevel == 2) {
		role = "admin"
	} else if conv.UserID == caller.UserID {
		role = "user"
	}

	if role == "user" && conv.UserID != caller.UserID {
		return utils.NewError(utils.ErrCodeForbidden, "you cannot send typing in this conversation")
	}

	s.publishToConversation(conv.ConversationID, ws.WSMessage{
		Type: ws.EventChatTyping,
		Data: ws.ChatTypingPayload{
			ConversationUUID: conv.ConversationUuid.String(),
			UserID:           caller.UserID,
			SenderRole:       role,
			IsTyping:         isTyping,
		},
	})

	return nil
}

func (s *chatService) RecallMessage(
	ctx context.Context,
	messageUUID uuid.UUID,
	callerUUID uuid.UUID,
) (v1dto.MessageResponse, error) {
	caller, err := s.getActiveUser(ctx, callerUUID)
	if err != nil {
		return v1dto.MessageResponse{}, err
	}

	msg, err := s.chatRepo.GetMessageByUUID(ctx, messageUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return v1dto.MessageResponse{}, utils.NewError(utils.ErrCodeNotFound, "message not found")
		}
		return v1dto.MessageResponse{}, utils.WrapError(utils.ErrCodeInternal, "failed to get message", err)
	}

	callerRole := s.determineSenderRole(caller)
	// Only the message sender or an admin can recall the message
	if msg.SenderID != caller.UserID && callerRole != "admin" {
		return v1dto.MessageResponse{}, utils.NewError(utils.ErrCodeForbidden, "you can only recall your own messages")
	}

	// 60-minute time window check
	if time.Since(msg.CreatedAt) > 60*time.Minute {
		return v1dto.MessageResponse{}, utils.NewError(utils.ErrCodeBadRequest, "messages can only be recalled within 60 minutes of sending")
	}

	if msg.ContentType == "recalled" {
		return v1dto.MapChatMessageToResponse(msg), nil
	}

	recalledMsg, err := s.chatRepo.RecallMessage(ctx, msg.MessageID)
	if err != nil {
		return v1dto.MessageResponse{}, utils.WrapError(utils.ErrCodeInternal, "failed to recall message", err)
	}

	conv, err := s.chatRepo.GetConversationByID(ctx, msg.ConversationID)
	if err == nil {
		s.publishToConversation(conv.ConversationID, ws.WSMessage{
			Type: ws.EventChatMessageRecalled,
			Data: map[string]any{
				"conversation_uuid": conv.ConversationUuid.String(),
				"message_uuid":      msg.MessageUuid.String(),
			},
		})
	}

	return v1dto.MapChatMessageToResponse(recalledMsg), nil
}

func (s *chatService) GetShopStatus(ctx context.Context) (v1dto.ShopStatusResponse, error) {
	isOnline := false
	if s.wsHub != nil {
		isOnline = s.wsHub.HasOnlineAdmin()
	}
	return v1dto.ShopStatusResponse{
		IsOnline: isOnline,
	}, nil
}
