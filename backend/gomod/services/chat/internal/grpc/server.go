package grpc

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	chatrepo "goshop/services/chat/internal/db/sqlc"
	"goshop/services/chat/internal/repository"
	chatv1 "goshop/shared/proto/chat/v1"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ChatGRPCServer serves internal gRPC callers (currently the WS gateway, which
// needs room resolution now that chat owns its own database).
type ChatGRPCServer struct {
	chatv1.UnimplementedChatServiceServer
	chatRepo repository.ChatRepository
}

func NewChatGRPCServer(chatRepo repository.ChatRepository) *ChatGRPCServer {
	return &ChatGRPCServer{chatRepo: chatRepo}
}

func (s *ChatGRPCServer) ResolveConversation(ctx context.Context, req *chatv1.ResolveConversationRequest) (*chatv1.ResolveConversationResponse, error) {
	var (
		conv chatrepo.ChatConversation
		err  error
	)

	switch {
	case strings.TrimSpace(req.GetConversationUuid()) != "":
		conversationUUID, parseErr := uuid.Parse(req.GetConversationUuid())
		if parseErr != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid conversation_uuid")
		}
		conv, err = s.chatRepo.GetConversationByUUID(ctx, conversationUUID)
	case req.GetConversationId() > 0:
		conv, err = s.chatRepo.GetConversationByID(ctx, req.GetConversationId())
	default:
		return nil, status.Error(codes.InvalidArgument, "conversation_uuid or conversation_id is required")
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "conversation not found")
		}
		return nil, status.Error(codes.Internal, "failed to resolve conversation")
	}

	return &chatv1.ResolveConversationResponse{
		ConversationId:   conv.ConversationID,
		ConversationUuid: conv.ConversationUuid.String(),
		UserId:           conv.UserID,
		Status:           conv.Status,
	}, nil
}
