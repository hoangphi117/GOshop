package contract

import (
	"context"
	"fmt"

	"goshop/shared/pkg/grpcclient"
	chatv1 "goshop/shared/proto/chat/v1"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Conversation is the resolved chat room a WebSocket client may join.
type Conversation struct {
	ConversationID   int32
	ConversationUUID uuid.UUID
	UserID           int32
	Status           string
}

// ChatClient is the internal read surface of the chat domain.
type ChatClient interface {
	ResolveConversationByUUID(ctx context.Context, conversationUUID uuid.UUID) (*Conversation, error)
	ResolveConversationByID(ctx context.Context, conversationID int32) (*Conversation, error)
	Close() error
}

type chatClient struct {
	client chatv1.ChatServiceClient
	conn   *grpc.ClientConn
}

func NewGRPCChatClient(target string, opts ...grpc.DialOption) (ChatClient, error) {
	if len(opts) == 0 {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpcclient.NewClient(target, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to chat service at %s: %w", target, err)
	}

	return &chatClient{
		client: chatv1.NewChatServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *chatClient) ResolveConversationByUUID(ctx context.Context, conversationUUID uuid.UUID) (*Conversation, error) {
	return c.resolve(ctx, &chatv1.ResolveConversationRequest{ConversationUuid: conversationUUID.String()})
}

func (c *chatClient) ResolveConversationByID(ctx context.Context, conversationID int32) (*Conversation, error) {
	return c.resolve(ctx, &chatv1.ResolveConversationRequest{ConversationId: conversationID})
}

func (c *chatClient) resolve(ctx context.Context, req *chatv1.ResolveConversationRequest) (*Conversation, error) {
	resp, err := c.client.ResolveConversation(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("resolve conversation response is empty")
	}

	conversationUUID, err := uuid.Parse(resp.GetConversationUuid())
	if err != nil {
		return nil, fmt.Errorf("invalid conversation uuid %q: %w", resp.GetConversationUuid(), err)
	}

	return &Conversation{
		ConversationID:   resp.GetConversationId(),
		ConversationUUID: conversationUUID,
		UserID:           resp.GetUserId(),
		Status:           resp.GetStatus(),
	}, nil
}

func (c *chatClient) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
