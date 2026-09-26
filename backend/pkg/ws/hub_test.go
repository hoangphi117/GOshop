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
