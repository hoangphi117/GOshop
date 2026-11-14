package service

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"time"

	sqlc "goshop/services/chat/internal/db/sqlc"
	chatdto "goshop/services/chat/internal/dto"
	"goshop/services/chat/internal/repository"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/utils"
	ws "goshop/shared/pkg/ws"

	pkgpgx "goshop/shared/pkg/pgx"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
)

const recallWindow = 60 * time.Minute

type chatService struct {
	chatRepo    repository.ChatRepository
	userRepo    UserReader
	orderReader OrderReader
	presence    PresenceReader
	publisher   WSPublisher
	txManager   pkgpgx.TxManager
	logger      zerolog.Logger
}

func NewChatService(
	chatRepo repository.ChatRepository,
	userRepo UserReader,
	orderReader OrderReader,
	presence PresenceReader,
	publisher WSPublisher,
	txManager pkgpgx.TxManager,
) ChatService {
	return &chatService{
		chatRepo:    chatRepo,
		userRepo:    userRepo,
		orderReader: orderReader,
		presence:    presence,
		publisher:   publisher,
		txManager:   txManager,
		logger:      *logger.Log,
	}
}

// -------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------

func (s *chatService) publishToConversation(conversationID int32, msgType ws.EventType, payload any) {
	if s.publisher == nil {
		return
	}
	go func() {
		s.publisher.PublishToConversation(context.Background(), conversationID, string(msgType), payload)
	}()
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

func (s *chatService) determineSenderRole(user User) string {
	// user_level: 1 = Admin, 2 = Moderator, 3 = Member
	if user.UserLevel == UserLevelAdmin || user.UserLevel == UserLevelModerator {
		return "admin"
	}
	return "user"
}

func (s *chatService) getActiveUser(ctx context.Context, userUUID uuid.UUID) (User, error) {
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return User{}, utils.NewError(utils.ErrCodeNotFound, "user not found")
		}
		return User{}, utils.WrapError(utils.ErrCodeInternal, "failed to get user", err)
	}

	if user.UserStatus != 1 { // 1: Active
		return User{}, utils.NewError(utils.ErrCodeForbidden, "user account is not active")
	}
	return user, nil
}

// -------------------------------------------------------------------------
// Business logic (ported 1:1 from the monolith chat service)
// -------------------------------------------------------------------------

func (s *chatService) GetOrCreateConversation(ctx context.Context, userUUID uuid.UUID) (*chatdto.ConversationDTO, error) {
	user, err := s.getActiveUser(ctx, userUUID)
	if err != nil {
		return nil, err
	}

	conversation, err := s.chatRepo.GetConversationByUserID(ctx, user.UserID)
	if err == nil {
		return chatdto.MapChatConversationToDTO(conversation), nil
	}

	if !errors.Is(err, sql.ErrNoRows) && !errors.Is(err, pgx.ErrNoRows) {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get conversation", err)
	}

	newConv, err := s.chatRepo.CreateConversation(ctx, sqlc.CreateConversationParams{
		UserID:       user.UserID,
		UserEmail:    user.UserEmail,
		UserFullname: user.UserFullname,
		UserAvatar:   user.UserAvatar,
		OwnerIsStaff: s.determineSenderRole(user) == "admin",
	})
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to create conversation", err)
	}

	return chatdto.MapChatConversationToDTO(newConv), nil
}

func (s *chatService) SendMessage(
	ctx context.Context,
	conversationUUID uuid.UUID,
	senderUUID uuid.UUID,
	senderRole string,
	req chatdto.SendMessageRequest,
) (chatdto.MessageResponse, error) {
	sender, err := s.getActiveUser(ctx, senderUUID)
	if err != nil {
		return chatdto.MessageResponse{}, err
	}

	conv, err := s.getConversation(ctx, conversationUUID)
	if err != nil {
		return chatdto.MessageResponse{}, err
	}

	if conv.Status == "closed" {
		_, _ = s.chatRepo.UpdateConversationStatus(ctx, conv.ConversationID, "open")
	}

	computedRole := "user"
	if senderRole == "admin" || (conv.UserID != sender.UserID && (sender.UserLevel == UserLevelAdmin || sender.UserLevel == UserLevelModerator)) {
		computedRole = "admin"
	} else if conv.UserID == sender.UserID {
		computedRole = "user"
	}

	if computedRole == "user" && conv.UserID != sender.UserID {
		return chatdto.MessageResponse{}, utils.NewError(utils.ErrCodeForbidden, "user is not part of this conversation")
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
		}
		if contentType == "" {
			contentType = "text"
		}
	}

	content := strings.TrimSpace(req.Content)
	switch contentType {
	case "order_ref":
		if req.OrderUUID == "" {
			return chatdto.MessageResponse{}, utils.NewError(utils.ErrCodeBadRequest, "order_uuid is required for order_ref messages")
		}
		if computedRole == "user" {
			orderUUID, err := uuid.Parse(req.OrderUUID)
			if err != nil {
				return chatdto.MessageResponse{}, utils.WrapError(utils.ErrCodeBadRequest, "invalid order UUID", err)
			}
			if s.orderReader == nil {
				return chatdto.MessageResponse{}, utils.NewError(utils.ErrCodeInternal, "order reader not configured")
			}
			order, err := s.orderReader.GetOrderSummary(ctx, orderUUID)
			if err != nil {
				var appErr *utils.AppError
				if errors.As(err, &appErr) {
					return chatdto.MessageResponse{}, appErr
				}
				if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
					return chatdto.MessageResponse{}, utils.NewError(utils.ErrCodeNotFound, "order not found")
				}
				return chatdto.MessageResponse{}, utils.WrapError(utils.ErrCodeInternal, "failed to get order", err)
			}
			if order.UserID != sender.UserID {
				return chatdto.MessageResponse{}, utils.NewError(utils.ErrCodeForbidden, "user is not authorized to reference this order")
			}
		}
		content = req.OrderUUID
	case "product_ref":
		if req.ProductUUID != "" {
			content = req.ProductUUID
		}
	}

	if contentType != "order_ref" && contentType != "product_ref" && content == "" && req.AttachmentURL == "" {
		return chatdto.MessageResponse{}, utils.NewError(utils.ErrCodeBadRequest, "content or attachment is required for non-reference messages")
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
	createParams := sqlc.CreateMessageParams{
		ConversationID: conv.ConversationID,
		SenderID:       sender.UserID,
		SenderRole:     computedRole,
		Content:        content,
		ContentType:    contentType,
		AttachmentUrl:  attachURL,
		AttachmentName: attachName,
		AttachmentSize: attachSize,
	}
	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		var txErr error
		msg, txErr = s.chatRepo.CreateMessage(txCtx, createParams)
		if txErr != nil {
			return utils.WrapError(utils.ErrCodeInternal, "failed to save chat message", txErr)
		}

		if txErr = s.chatRepo.UpdateLastMessageAt(txCtx, conv.ConversationID); txErr != nil {
			return utils.WrapError(utils.ErrCodeInternal, "failed to update conversation timestamp", txErr)
		}
		return nil
	})

	if err != nil {
		return chatdto.MessageResponse{}, utils.WrapError(utils.ErrCodeInternal, "failed to save chat message", err)
	}

	// Publish the new message realtime event to the conversation
	s.publishToConversation(conv.ConversationID, ws.EventChatMessage, ws.ChatMessagePayload{
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
	})

	return chatdto.MapChatMessageToResponse(msg), nil
}

func (s *chatService) ListMessages(
	ctx context.Context,
	conversationUUID uuid.UUID,
	callerUUID uuid.UUID,
	req chatdto.ListMessagesRequest,
) ([]chatdto.MessageResponse, int64, error) {
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

	res := make([]chatdto.MessageResponse, len(messages))
	for i, m := range messages {
		res[i] = chatdto.MapChatMessageToResponse(m)
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
	s.publishToConversation(conv.ConversationID, ws.EventChatRead, ws.ChatReadPayload{
		ConversationUUID: conv.ConversationUuid.String(),
		ReaderID:         caller.UserID,
	})

	return nil
}

func (s *chatService) ListConversations(
	ctx context.Context,
	callerUUID uuid.UUID,
	req chatdto.ListConversationsRequest,
) ([]chatdto.ConversationResponse, int64, error) {
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
	switch status {
	case "active":
		status = "open"
	case "archived":
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

	res := make([]chatdto.ConversationResponse, 0, len(rows))
	for _, r := range rows {
		var lastMsgAt *time.Time
		if r.LastMessageAt.Valid {
			lastMsgAt = &r.LastMessageAt.Time
		}

		// Self-heal the denormalized user snapshot: conversations seeded from the
		// monolith have all three columns empty (pre-extraction rows), which
		// renders as "?" in the admin list. A missing avatar is healed on its own
		// because the avatar feature stores a new column and every pre-existing
		// conversation row still carries the old (often empty) value. ponytail:
		// renamed users keep their stale snapshot — refresh wipes identity or a
		// real backfill fixes that.
		userEmail, userFullname, userAvatar := r.UserEmail, r.UserFullname, r.UserAvatar
		if (userEmail == "" && userFullname == "") || userAvatar == "" {
			if owner, ownerErr := s.userRepo.GetByID(ctx, r.UserID); ownerErr == nil {
				userEmail, userFullname, userAvatar = owner.UserEmail, owner.UserFullname, owner.UserAvatar
				if userFullname == "" {
					userFullname = userEmail
				}
				if backfillErr := s.chatRepo.UpdateConversationUserInfo(ctx, sqlc.UpdateConversationUserInfoParams{
					ConversationID: r.ConversationID,
					UserEmail:      userEmail,
					UserFullname:   userFullname,
					UserAvatar:     userAvatar,
				}); backfillErr != nil {
					s.logger.Warn().Err(backfillErr).Int32("conversation_id", r.ConversationID).
						Msg("Failed to backfill conversation user snapshot")
				}
			} else {
				s.logger.Warn().Err(ownerErr).Int32("user_id", r.UserID).
					Msg("Failed to load conversation owner from identity")
			}
		}

		isCustomerOnline := s.presence.IsUserOnline(ctx, r.UserID)
		isShopOnline := s.presence.HasOnlineAdmin(ctx)

		res = append(res, chatdto.ConversationResponse{
			ConversationID:     r.ConversationID,
			UUID:               r.ConversationUuid,
			Status:             r.Status,
			UserID:             r.UserID,
			AdminID:            r.AdminID,
			UserEmail:          userEmail,
			UserFullname:       userFullname,
			UserAvatar:         userAvatar,
			UnreadCount:        r.UnreadCount,
			LastMessageContent: r.LastMessageContent,
			LastMessageAt:      lastMsgAt,
			CreatedAt:          r.CreatedAt,
			UpdatedAt:          r.UpdatedAt,
			IsOnline:           isCustomerOnline,
			IsShopOnline:       isShopOnline,
		})
	}

	return res, total, nil
}

func (s *chatService) CloseConversation(
	ctx context.Context,
	conversationUUID uuid.UUID,
) (*chatdto.ConversationDTO, error) {
	conv, err := s.getConversation(ctx, conversationUUID)
	if err != nil {
		return nil, err
	}

	if conv.Status == "closed" {
		return chatdto.MapChatConversationToDTO(conv), nil
	}

	updatedConv, err := s.chatRepo.UpdateConversationStatus(ctx, conv.ConversationID, "closed")
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to close conversation", err)
	}

	// Publish the conversation closed event to the conversation
	s.publishToConversation(conv.ConversationID, ws.EventChatConvClosed, map[string]any{
		"conversation_uuid": conv.ConversationUuid.String(),
	})

	return chatdto.MapChatConversationToDTO(updatedConv), nil
}

func (s *chatService) ReopenConversation(
	ctx context.Context,
	conversationUUID uuid.UUID,
) (*chatdto.ConversationDTO, error) {
	conv, err := s.getConversation(ctx, conversationUUID)
	if err != nil {
		return nil, err
	}

	if conv.Status == "open" {
		return chatdto.MapChatConversationToDTO(conv), nil
	}

	updatedConv, err := s.chatRepo.UpdateConversationStatus(ctx, conv.ConversationID, "open")
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to reopen conversation", err)
	}

	return chatdto.MapChatConversationToDTO(updatedConv), nil
}

func (s *chatService) GetConversationDetails(
	ctx context.Context,
	conversationUUID uuid.UUID,
	callerUUID uuid.UUID,
) (chatdto.ConversationResponse, error) {
	caller, err := s.getActiveUser(ctx, callerUUID)
	if err != nil {
		return chatdto.ConversationResponse{}, err
	}

	conv, err := s.getConversation(ctx, conversationUUID)
	if err != nil {
		return chatdto.ConversationResponse{}, err
	}

	role := s.determineSenderRole(caller)
	if role == "user" && conv.UserID != caller.UserID {
		return chatdto.ConversationResponse{}, utils.NewError(utils.ErrCodeForbidden, "you are not authorized to view this conversation")
	}

	convOwner := caller
	if conv.UserID != caller.UserID {
		owner, ownerErr := s.userRepo.GetByID(ctx, conv.UserID)
		if ownerErr != nil && !errors.Is(ownerErr, sql.ErrNoRows) && !errors.Is(ownerErr, pgx.ErrNoRows) {
			return chatdto.ConversationResponse{}, utils.WrapError(utils.ErrCodeInternal, "failed to get conversation owner", ownerErr)
		}
		convOwner = owner
	}
	unreadCount, _ := s.chatRepo.CountUnreadMessages(ctx, conv.ConversationID, caller.UserID)

	var lastMsgAt *time.Time
	if conv.LastMessageAt.Valid {
		lastMsgAt = &conv.LastMessageAt.Time
	}

	isCustOnline := s.presence.IsUserOnline(ctx, conv.UserID)
	isShopOnline := s.presence.HasOnlineAdmin(ctx)

	return chatdto.ConversationResponse{
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
	if conv.UserID != caller.UserID && (caller.UserLevel == UserLevelAdmin || caller.UserLevel == UserLevelModerator) {
		role = "admin"
	} else if conv.UserID == caller.UserID {
		role = "user"
	}

	if role == "user" && conv.UserID != caller.UserID {
		return utils.NewError(utils.ErrCodeForbidden, "you cannot send typing in this conversation")
	}

	s.publishToConversation(conv.ConversationID, ws.EventChatTyping, ws.ChatTypingPayload{
		ConversationUUID: conv.ConversationUuid.String(),
		UserID:           caller.UserID,
		SenderRole:       role,
		IsTyping:         isTyping,
	})

	return nil
}

func (s *chatService) RecallMessage(
	ctx context.Context,
	messageUUID uuid.UUID,
	callerUUID uuid.UUID,
) (chatdto.MessageResponse, error) {
	caller, err := s.getActiveUser(ctx, callerUUID)
	if err != nil {
		return chatdto.MessageResponse{}, err
	}

	msg, err := s.chatRepo.GetMessageByUUID(ctx, messageUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return chatdto.MessageResponse{}, utils.NewError(utils.ErrCodeNotFound, "message not found")
		}
		return chatdto.MessageResponse{}, utils.WrapError(utils.ErrCodeInternal, "failed to get message", err)
	}

	callerRole := s.determineSenderRole(caller)
	// Only the message sender or an admin can recall the message
	if msg.SenderID != caller.UserID && callerRole != "admin" {
		return chatdto.MessageResponse{}, utils.NewError(utils.ErrCodeForbidden, "you can only recall your own messages")
	}

	// 60-minute time window check
	if time.Since(msg.CreatedAt) > recallWindow {
		return chatdto.MessageResponse{}, utils.NewError(utils.ErrCodeBadRequest, "messages can only be recalled within 60 minutes of sending")
	}

	if msg.ContentType == "recalled" {
		return chatdto.MapChatMessageToResponse(msg), nil
	}

	recalledMsg, err := s.chatRepo.RecallMessage(ctx, msg.MessageID)
	if err != nil {
		return chatdto.MessageResponse{}, utils.WrapError(utils.ErrCodeInternal, "failed to recall message", err)
	}

	conv, err := s.chatRepo.GetConversationByID(ctx, msg.ConversationID)
	if err == nil {
		s.publishToConversation(conv.ConversationID, ws.EventChatMessageRecalled, map[string]any{
			"conversation_uuid": conv.ConversationUuid.String(),
			"message_uuid":      msg.MessageUuid.String(),
		})
	}

	return chatdto.MapChatMessageToResponse(recalledMsg), nil
}

func (s *chatService) GetShopStatus(ctx context.Context) (chatdto.ShopStatusResponse, error) {
	isOnline := s.presence.HasOnlineAdmin(ctx)
	return chatdto.ShopStatusResponse{
		IsOnline: isOnline,
	}, nil
}
