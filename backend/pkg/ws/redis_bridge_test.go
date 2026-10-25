package ws_test

import (
	"context"
	"encoding/json"
	"testing"

	"goshop/pkg/ws"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockHub struct {
	broadcastToUserCalls []struct {
		UserID int32
		Msg    ws.WSMessage
	}
	broadcastToAdminsCalls       []ws.WSMessage
	broadcastToAllCalls          []ws.WSMessage
	broadcastToConversationCalls []struct {
		ConvID int32
		Msg    ws.WSMessage
	}
}

func (m *mockHub) Register(client *ws.Client)            {}
func (m *mockHub) Unregister(client *ws.Client)          {}
func (m *mockHub) SetPresenceStore(ps *ws.PresenceStore) {}
func (m *mockHub) BroadcastToUser(userID int32, msg ws.WSMessage) {
	m.broadcastToUserCalls = append(m.broadcastToUserCalls, struct {
		UserID int32
		Msg    ws.WSMessage
	}{userID, msg})
}
func (m *mockHub) BroadCastToAdmins(msg ws.WSMessage) {
	m.broadcastToAdminsCalls = append(m.broadcastToAdminsCalls, msg)
}
func (m *mockHub) BroadcastToAll(msg ws.WSMessage) {
	m.broadcastToAllCalls = append(m.broadcastToAllCalls, msg)
}
func (m *mockHub) BroadcastToConversation(conversationID int32, msg ws.WSMessage) {
	m.broadcastToConversationCalls = append(m.broadcastToConversationCalls, struct {
		ConvID int32
		Msg    ws.WSMessage
	}{conversationID, msg})
}
func (m *mockHub) BroadcastAdminPresence(isOnline bool)                             {}
func (m *mockHub) BroadcastUserPresence(userID int32, convID *int32, isOnline bool) {}
func (m *mockHub) IsUserOnline(userID int32) bool                                   { return false }
func (m *mockHub) HasOnlineAdmin() bool                                             { return false }
func (m *mockHub) Shutdown()                                                        {}

func TestRedisNotificationEnvelope_Serialization(t *testing.T) {
	userID := int32(42)
	convID := int32(99)
	envelope := ws.RedisNotificationEnvelope{
		UserID:         &userID,
		IsAdmin:        false,
		IsAll:          false,
		ConversationID: &convID,
		Message: ws.WSMessage{
			Type: ws.EventChatMessage,
			Data: map[string]any{"content": "hello from redis"},
		},
	}

	data, err := json.Marshal(envelope)
	require.NoError(t, err)

	var decoded ws.RedisNotificationEnvelope
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.NotNil(t, decoded.UserID)
	assert.Equal(t, int32(42), *decoded.UserID)
	assert.NotNil(t, decoded.ConversationID)
	assert.Equal(t, int32(99), *decoded.ConversationID)
	assert.False(t, decoded.IsAdmin)
	assert.Equal(t, ws.EventChatMessage, decoded.Message.Type)
}

func TestRedisBridge_NilRedisSafe(t *testing.T) {
	hub := &mockHub{}
	bridge := ws.NewRedisBridge(nil, hub)

	// Publish with nil Redis should not panic
	err := bridge.Publish(context.Background(), ws.RedisNotificationEnvelope{})
	assert.NoError(t, err)

	// StartSubscriber with nil Redis should exit immediately without panic
	bridge.StartSubscriber(context.Background())

	// Stop should not panic
	bridge.Stop()
}
