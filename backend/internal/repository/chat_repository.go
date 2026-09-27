package repository

import (
	"context"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"

	"github.com/google/uuid"
)

type SqlChatRepository struct {
	db sqlc.Querier
}

func NewSqlChatRepository(db sqlc.Querier) ChatRepository {
	return &SqlChatRepository{
		db: db,
	}
}

func (cr *SqlChatRepository) getQuerier(ctx context.Context) sqlc.Querier {
	return db.GetQuerier(ctx, cr.db)
}

func (cr *SqlChatRepository) CreateConversation(ctx context.Context, userID int32) (sqlc.ChatConversation, error) {
	return cr.getQuerier(ctx).CreateConversation(ctx, userID)
}

func (cr *SqlChatRepository) GetConversationByUUID(ctx context.Context, conversationUUID uuid.UUID) (sqlc.ChatConversation, error) {
	return cr.getQuerier(ctx).GetConversationByUUID(ctx, conversationUUID)
}

func (cr *SqlChatRepository) GetConversationByID(ctx context.Context, conversationID int32) (sqlc.ChatConversation, error) {
	return cr.getQuerier(ctx).GetConversationByID(ctx, conversationID)
}

func (cr *SqlChatRepository) GetConversationByUserID(ctx context.Context, userID int32) (sqlc.ChatConversation, error) {
	return cr.getQuerier(ctx).GetConversationByUserID(ctx, userID)
}

func (cr *SqlChatRepository) UpdateConversationStatus(ctx context.Context, conversationID int32, status string) (sqlc.ChatConversation, error) {
	return cr.getQuerier(ctx).UpdateConversationStatus(ctx, sqlc.UpdateConversationStatusParams{
		ConversationID: conversationID,
		Status:         status,
	})
}

func (cr *SqlChatRepository) UpdateLastMessageAt(ctx context.Context, conversationID int32) error {
	return cr.getQuerier(ctx).UpdateLastMessageAt(ctx, conversationID)
}

func (cr *SqlChatRepository) ListConversations(ctx context.Context, params sqlc.ListConversationsParams) ([]sqlc.ListConversationsRow, error) {
	return cr.getQuerier(ctx).ListConversations(ctx, params)
}

func (cr *SqlChatRepository) CountConversations(ctx context.Context, params sqlc.CountConversationsParams) (int64, error) {
	return cr.getQuerier(ctx).CountConversations(ctx, params)
}

func (cr *SqlChatRepository) CreateMessage(ctx context.Context, params sqlc.CreateMessageParams) (sqlc.ChatMessage, error) {
	return cr.getQuerier(ctx).CreateMessage(ctx, params)
}

func (cr *SqlChatRepository) ListMessages(ctx context.Context, params sqlc.ListMessagesParams) ([]sqlc.ChatMessage, error) {
	return cr.getQuerier(ctx).ListMessages(ctx, params)
}

func (cr *SqlChatRepository) ListMessagesCursor(ctx context.Context, params sqlc.ListMessagesCursorParams) ([]sqlc.ChatMessage, error) {
	return cr.getQuerier(ctx).ListMessagesCursor(ctx, params)
}

func (cr *SqlChatRepository) CountMessagesByConversationID(ctx context.Context, conversationID int32) (int64, error) {
	return cr.getQuerier(ctx).CountMessagesByConversationID(ctx, conversationID)
}

func (cr *SqlChatRepository) MarkMessagesRead(ctx context.Context, conversationID int32, readerID int32) error {
	return cr.getQuerier(ctx).MarkMessagesRead(ctx, sqlc.MarkMessagesReadParams{
		ConversationID: conversationID,
		ReaderID:       readerID,
	})
}

func (cr *SqlChatRepository) CountUnreadMessages(ctx context.Context, conversationID int32, readerID int32) (int64, error) {
	return cr.getQuerier(ctx).CountUnreadMessages(ctx, sqlc.CountUnreadMessagesParams{
		ConversationID: conversationID,
		ReaderID:       readerID,
	})
}

func (cr *SqlChatRepository) GetTotalUnreadMessagesForUser(ctx context.Context, userID int32) (int64, error) {
	return cr.getQuerier(ctx).GetTotalUnreadMessagesForUser(ctx, userID)
}

func (cr *SqlChatRepository) GetTotalUnreadMessagesForAdmin(ctx context.Context) (int64, error) {
	return cr.getQuerier(ctx).GetTotalUnreadMessagesForAdmin(ctx)
}
