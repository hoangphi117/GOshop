package ws_test

import (
	"encoding/json"
	"testing"

	"goshop/pkg/ws"

	"github.com/stretchr/testify/assert"
)

func TestHub_AdminAndUserBroadcast(t *testing.T) {
	hub := ws.NewHub()

	// Create mock customer client
	customerClient := &ws.Client{
		Hub:     hub,
		Send:    make(chan []byte, 10),
		UserID:  100,
		IsAdmin: false,
	}

	// Create mock admin client
	adminClient := &ws.Client{
		Hub:     hub,
		Send:    make(chan []byte, 10),
		UserID:  200,
		IsAdmin: true,
	}

	hub.Register(customerClient)
	hub.Register(adminClient)

	assert.True(t, hub.IsUserOnline(100))
	assert.True(t, hub.IsUserOnline(200))
	assert.False(t, hub.IsUserOnline(999))

	t.Run("BroadcastToUser sends only to designated user", func(t *testing.T) {
		msg := ws.WSMessage{
			Type: ws.EventTypeNotification,
			Data: "hello customer",
		}

		hub.BroadcastToUser(100, msg)

		select {
		case payload := <-customerClient.Send:
			var received ws.WSMessage
			err := json.Unmarshal(payload, &received)
			assert.NoError(t, err)
			assert.Equal(t, "hello customer", received.Data)
		default:
			t.Fatal("expected message on customer client channel")
		}

		// Admin should receive nothing
		select {
		case <-adminClient.Send:
			t.Fatal("admin client received message intended only for customer")
		default:
			// OK
		}
	})

	t.Run("BroadCastToAdmins sends only to admin clients", func(t *testing.T) {
		msg := ws.WSMessage{
			Type: ws.EventTypeNotification,
			Data: "hello admin",
		}

		hub.BroadCastToAdmins(msg)

		select {
		case payload := <-adminClient.Send:
			var received ws.WSMessage
			err := json.Unmarshal(payload, &received)
			assert.NoError(t, err)
			assert.Equal(t, "hello admin", received.Data)
		default:
			t.Fatal("expected message on admin client channel")
		}

		// Customer should receive nothing
		select {
		case <-customerClient.Send:
			t.Fatal("customer client received message intended only for admin")
		default:
			// OK
		}
	})

	t.Run("BroadcastToAll sends to all connected clients", func(t *testing.T) {
		msg := ws.WSMessage{
			Type: ws.EventTypeNotification,
			Data: "system broadcast",
		}

		hub.BroadcastToAll(msg)

		select {
		case payload := <-customerClient.Send:
			var received ws.WSMessage
			_ = json.Unmarshal(payload, &received)
			assert.Equal(t, "system broadcast", received.Data)
		default:
			t.Fatal("expected message on customer client channel")
		}

		select {
		case payload := <-adminClient.Send:
			var received ws.WSMessage
			_ = json.Unmarshal(payload, &received)
			assert.Equal(t, "system broadcast", received.Data)
		default:
			t.Fatal("expected message on admin client channel")
		}
	})

	t.Run("Unregister removes client correctly", func(t *testing.T) {
		hub.Unregister(adminClient)
		assert.False(t, hub.IsUserOnline(200))

		// Broadcast to admins should now be a no-op
		msg := ws.WSMessage{
			Type: ws.EventTypeNotification,
			Data: "another admin alert",
		}
		hub.BroadCastToAdmins(msg)

		select {
		case _, ok := <-adminClient.Send:
			if ok {
				t.Fatal("unregistered admin client received message")
			}
		default:
			// OK
		}
	})
}

func TestHub_ConversationBroadcast(t *testing.T) {
	hub := ws.NewHub()

	convID1 := int32(10)
	convID2 := int32(20)

	clientInRoom1 := &ws.Client{
		Hub:            hub,
		Send:           make(chan []byte, 10),
		UserID:         101,
		IsAdmin:        false,
		ConversationID: &convID1,
	}

	adminInRoom1 := &ws.Client{
		Hub:            hub,
		Send:           make(chan []byte, 10),
		UserID:         901,
		IsAdmin:        true,
		ConversationID: &convID1,
	}

	clientInRoom2 := &ws.Client{
		Hub:            hub,
		Send:           make(chan []byte, 10),
		UserID:         102,
		IsAdmin:        false,
		ConversationID: &convID2,
	}

	clientNoRoom := &ws.Client{
		Hub:            hub,
		Send:           make(chan []byte, 10),
		UserID:         103,
		IsAdmin:        false,
		ConversationID: nil,
	}

	hub.Register(clientInRoom1)
	hub.Register(adminInRoom1)
	hub.Register(clientInRoom2)
	hub.Register(clientNoRoom)

	t.Run("BroadcastToConversation delivers to all clients in that conversation", func(t *testing.T) {
		msg := ws.WSMessage{
			Type: ws.EventChatMessage,
			Data: "room 1 message",
		}

		hub.BroadcastToConversation(convID1, msg)

		// Both clients in room 1 must receive
		select {
		case payload := <-clientInRoom1.Send:
			var received ws.WSMessage
			err := json.Unmarshal(payload, &received)
			assert.NoError(t, err)
			assert.Equal(t, "room 1 message", received.Data)
		default:
			t.Fatal("expected message on clientInRoom1 channel")
		}

		select {
		case payload := <-adminInRoom1.Send:
			var received ws.WSMessage
			err := json.Unmarshal(payload, &received)
			assert.NoError(t, err)
			assert.Equal(t, "room 1 message", received.Data)
		default:
			t.Fatal("expected message on adminInRoom1 channel")
		}

		// Client in room 2 must NOT receive
		select {
		case <-clientInRoom2.Send:
			t.Fatal("clientInRoom2 received message intended for room 1")
		default:
			// OK
		}

		// Client not in any room must NOT receive
		select {
		case <-clientNoRoom.Send:
			t.Fatal("clientNoRoom received message intended for room 1")
		default:
			// OK
		}
	})

	t.Run("BroadcastToConversation on empty room with no clients does not panic", func(t *testing.T) {
		emptyHub := ws.NewHub()
		msg := ws.WSMessage{
			Type: ws.EventChatMessage,
			Data: "empty room message",
		}
		assert.NotPanics(t, func() {
			emptyHub.BroadcastToConversation(9999, msg)
		})
	})

	t.Run("Unregister removes client from conversation mapping", func(t *testing.T) {
		hub.Unregister(clientInRoom1)

		msg := ws.WSMessage{
			Type: ws.EventChatMessage,
			Data: "subsequent room 1 message",
		}
		hub.BroadcastToConversation(convID1, msg)

		// Unregistered client does not receive
		select {
		case _, ok := <-clientInRoom1.Send:
			if ok {
				t.Fatal("unregistered client received room message")
			}
		default:
			// OK
		}

		// Remaining client in room 1 still receives
		select {
		case payload := <-adminInRoom1.Send:
			var received ws.WSMessage
			_ = json.Unmarshal(payload, &received)
			assert.Equal(t, "subsequent room 1 message", received.Data)
		default:
			t.Fatal("adminInRoom1 should have received message")
		}
	})
}
