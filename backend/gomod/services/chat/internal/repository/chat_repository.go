package repository

import (
	"context"

	"goshop/services/chat/internal/db/sqlc"

	"github.com/google/uuid"
)

type ChatRepository interface {
	CreateConversation(ctx context.Context, params sqlc.CreateConversationParams) (sqlc.ChatConversation, error)
	GetConversationByUUID(ctx context.Context, conversationUUID uuid.UUID) (sqlc.ChatConversation, error)
	GetConversationByID(ctx context.Context, conversationID int32) (sqlc.ChatConversation, error)
	GetConversationByUserID(ctx context.Context, userID int32) (sqlc.ChatConversation, error)
	UpdateConversationStatus(ctx context.Context, conversationID int32, status string) (sqlc.ChatConversation, error)
	UpdateLastMessageAt(ctx context.Context, conversationID int32) error
	UpdateConversationUserInfo(ctx context.Context, params sqlc.UpdateConversationUserInfoParams) error
	ListConversations(ctx context.Context, params sqlc.ListConversationsParams) ([]sqlc.ListConversationsRow, error)
	CountConversations(ctx context.Context, params sqlc.CountConversationsParams) (int64, error)
	CreateMessage(ctx context.Context, params sqlc.CreateMessageParams) (sqlc.ChatMessage, error)
	ListMessages(ctx context.Context, params sqlc.ListMessagesParams) ([]sqlc.ChatMessage, error)
	ListMessagesCursor(ctx context.Context, params sqlc.ListMessagesCursorParams) ([]sqlc.ChatMessage, error)
	CountMessagesByConversationID(ctx context.Context, conversationID int32) (int64, error)
	MarkMessagesRead(ctx context.Context, conversationID int32, readerID int32) error
	CountUnreadMessages(ctx context.Context, conversationID int32, readerID int32) (int64, error)
	GetTotalUnreadMessagesForUser(ctx context.Context, userID int32) (int64, error)
	GetTotalUnreadMessagesForAdmin(ctx context.Context) (int64, error)
	GetMessageByUUID(ctx context.Context, messageUUID uuid.UUID) (sqlc.ChatMessage, error)
	RecallMessage(ctx context.Context, messageID int32) (sqlc.ChatMessage, error)
}

type SqlChatRepository struct {
	db sqlc.Querier
}

func NewSqlChatRepository(db sqlc.Querier) ChatRepository {
	return &SqlChatRepository{db: db}
}

func (r *SqlChatRepository) CreateConversation(ctx context.Context, params sqlc.CreateConversationParams) (sqlc.ChatConversation, error) {
	return r.db.CreateConversation(ctx, params)
}

func (r *SqlChatRepository) GetConversationByUUID(ctx context.Context, conversationUUID uuid.UUID) (sqlc.ChatConversation, error) {
	return r.db.GetConversationByUUID(ctx, conversationUUID)
}

func (r *SqlChatRepository) GetConversationByID(ctx context.Context, conversationID int32) (sqlc.ChatConversation, error) {
	return r.db.GetConversationByID(ctx, conversationID)
}

func (r *SqlChatRepository) GetConversationByUserID(ctx context.Context, userID int32) (sqlc.ChatConversation, error) {
	return r.db.GetConversationByUserID(ctx, userID)
}

func (r *SqlChatRepository) UpdateConversationStatus(ctx context.Context, conversationID int32, status string) (sqlc.ChatConversation, error) {
	return r.db.UpdateConversationStatus(ctx, sqlc.UpdateConversationStatusParams{
		ConversationID: conversationID,
		Status:         status,
	})
}

func (r *SqlChatRepository) UpdateLastMessageAt(ctx context.Context, conversationID int32) error {
	return r.db.UpdateLastMessageAt(ctx, conversationID)
}

func (r *SqlChatRepository) UpdateConversationUserInfo(ctx context.Context, params sqlc.UpdateConversationUserInfoParams) error {
	return r.db.UpdateConversationUserInfo(ctx, params)
}

func (r *SqlChatRepository) ListConversations(ctx context.Context, params sqlc.ListConversationsParams) ([]sqlc.ListConversationsRow, error) {
	return r.db.ListConversations(ctx, params)
}

func (r *SqlChatRepository) CountConversations(ctx context.Context, params sqlc.CountConversationsParams) (int64, error) {
	return r.db.CountConversations(ctx, params)
}

func (r *SqlChatRepository) CreateMessage(ctx context.Context, params sqlc.CreateMessageParams) (sqlc.ChatMessage, error) {
	return r.db.CreateMessage(ctx, params)
}

func (r *SqlChatRepository) ListMessages(ctx context.Context, params sqlc.ListMessagesParams) ([]sqlc.ChatMessage, error) {
	return r.db.ListMessages(ctx, params)
}

func (r *SqlChatRepository) ListMessagesCursor(ctx context.Context, params sqlc.ListMessagesCursorParams) ([]sqlc.ChatMessage, error) {
	return r.db.ListMessagesCursor(ctx, params)
}

func (r *SqlChatRepository) CountMessagesByConversationID(ctx context.Context, conversationID int32) (int64, error) {
	return r.db.CountMessagesByConversationID(ctx, conversationID)
}

func (r *SqlChatRepository) MarkMessagesRead(ctx context.Context, conversationID int32, readerID int32) error {
	return r.db.MarkMessagesRead(ctx, sqlc.MarkMessagesReadParams{
		ConversationID: conversationID,
		ReaderID:       readerID,
	})
}

func (r *SqlChatRepository) CountUnreadMessages(ctx context.Context, conversationID int32, readerID int32) (int64, error) {
	return r.db.CountUnreadMessages(ctx, sqlc.CountUnreadMessagesParams{
		ConversationID: conversationID,
		ReaderID:       readerID,
	})
}

func (r *SqlChatRepository) GetTotalUnreadMessagesForUser(ctx context.Context, userID int32) (int64, error) {
	return r.db.GetTotalUnreadMessagesForUser(ctx, userID)
}

func (r *SqlChatRepository) GetTotalUnreadMessagesForAdmin(ctx context.Context) (int64, error) {
	return r.db.GetTotalUnreadMessagesForAdmin(ctx)
}

func (r *SqlChatRepository) GetMessageByUUID(ctx context.Context, messageUUID uuid.UUID) (sqlc.ChatMessage, error) {
	return r.db.GetMessageByUUID(ctx, messageUUID)
}

func (r *SqlChatRepository) RecallMessage(ctx context.Context, messageID int32) (sqlc.ChatMessage, error) {
	return r.db.RecallMessage(ctx, messageID)
}
