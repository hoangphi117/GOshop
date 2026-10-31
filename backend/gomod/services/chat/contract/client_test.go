package contract

import (
	"context"
	"net"
	"testing"

	chatv1 "goshop/shared/proto/chat/v1"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

const testConversationUUID = "5f0c9a3e-8b1d-4c2a-9e6f-7a3b5c1d2e4f"

type mockChatServer struct {
	chatv1.UnimplementedChatServiceServer
}

func (s *mockChatServer) ResolveConversation(ctx context.Context, req *chatv1.ResolveConversationRequest) (*chatv1.ResolveConversationResponse, error) {
	return &chatv1.ResolveConversationResponse{
		ConversationId:   req.GetConversationId(),
		ConversationUuid: testConversationUUID, // must stay a parseable uuid; the client parses it
		UserId:           7,
		Status:           "active",
	}, nil
}

func newTestChatClient(t *testing.T) ChatClient {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	chatv1.RegisterChatServiceServer(grpcServer, &mockChatServer{})
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(grpcServer.Stop)

	client, err := NewGRPCChatClient(listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestGRPCChatClient_ResolveConversationByUUID(t *testing.T) {
	client := newTestChatClient(t)

	conversation, err := client.ResolveConversationByUUID(context.Background(), uuid.MustParse(testConversationUUID))
	require.NoError(t, err)
	require.NotNil(t, conversation)

	assert.Equal(t, testConversationUUID, conversation.ConversationUUID.String())
	assert.Equal(t, "active", conversation.Status)
}

func TestGRPCChatClient_ResolveConversationByID(t *testing.T) {
	client := newTestChatClient(t)

	conversation, err := client.ResolveConversationByID(context.Background(), 55)
	require.NoError(t, err)
	require.NotNil(t, conversation)

	assert.Equal(t, int32(55), conversation.ConversationID)
	assert.Equal(t, int32(7), conversation.UserID)
	assert.Equal(t, "active", conversation.Status)
}
