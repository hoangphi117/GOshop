package ws

import (
	"encoding/json"
	"goshop/pkg/logger"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Hub interface {
	Register(client *Client)
	Unregister(client *Client)
	BroadcastToUser(userID int32, msg WSMessage)
	IsUserOnline(userID int32) bool
	Shutdown()
}

type hub struct {
	// Registered clients mapped by UserID: map[UserID]map[*Client]bool
	clients map[int32]map[*Client]bool
	mu      sync.RWMutex
}

func NewHub() Hub {
	return &hub{
		clients: make(map[int32]map[*Client]bool),
		mu:      sync.RWMutex{},
	}
}

func (h *hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client.UserID]; !ok {
		h.clients[client.UserID] = make(map[*Client]bool)
	}
	h.clients[client.UserID][client] = true

	logger.Log.Info().
		Int32("user_id", client.UserID).
		Int("active_connections_for_user", len(h.clients[client.UserID])).
		Msg("WebSocket client registered")
}

func (h *hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if userClients, exists := h.clients[client.UserID]; exists {
		if _, ok := userClients[client]; ok {
			delete(userClients, client)
			client.Close()

			if len(userClients) == 0 {
				delete(h.clients, client.UserID)
			}
		}
	}

	logger.Log.Info().
		Int32("user_id", client.UserID).
		Msg("WebSocket client unregistered")
}

func (h *hub) Shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	logger.Log.Info().Msg("Closing all active WebSocket connections for graceful shutdown...")

	closeMsg := websocket.FormatCloseMessage(websocket.CloseGoingAway, "Server is shutting down")

	for userID, userClients := range h.clients {
		for client := range userClients {
			_ = client.Conn.WriteControl(
				websocket.CloseMessage,
				closeMsg,
				time.Now().Add(writeWait),
			)
			client.Close()
		}
		delete(h.clients, userID)
	}

	logger.Log.Info().Msg("WebSocket Hub shut down successfully")
}

// BroadcastToUser sends a message to all active connections belonging to a specific user
func (h *hub) BroadcastToUser(userID int32, msg WSMessage) {
	h.mu.RLock()
	userClients, exists := h.clients[userID]
	if !exists || len(userClients) == 0 {
		h.mu.RUnlock()
		logger.Log.Warn().
			Int32("target_user_id", userID).
			Str("event_type", string(msg.Type)).
			Msg("WebSocket Broadcast: target user is OFFLINE (no active WebSocket connections)")
		return
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		h.mu.RUnlock()
		logger.Log.Error().
			Err(err).
			Int32("user_id", userID).
			Msg("Failed to marshal WebSocket message")
		return
	}

	// Copy clients slice to minimize lock holding time
	targets := make([]*Client, 0, len(userClients))
	for client := range userClients {
		targets = append(targets, client)
	}
	h.mu.RUnlock()

	logger.Log.Info().
		Int32("target_user_id", userID).
		Int("target_connections", len(targets)).
		Str("event_type", string(msg.Type)).
		Msg("WebSocket broadcasting to user connections")

	for _, client := range targets {
		select {
		case client.Send <- payload:
		default:
			// Buffer full, unregister problematic client
			h.Unregister(client)
		}
	}
}

func (h *hub) IsUserOnline(userID int32) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients, exists := h.clients[userID]
	return exists && len(clients) > 0
}
