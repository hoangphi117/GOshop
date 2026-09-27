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
	BroadCastToAdmins(msg WSMessage)
	BroadcastToUser(userID int32, msg WSMessage)
	BroadcastToAll(msg WSMessage)
	BroadcastToConversation(conversationID int32, msg WSMessage)
	IsUserOnline(userID int32) bool
	Shutdown()
}

type hub struct {
	// Registered clients mapped by UserID: map[UserID]map[*Client]bool
	clients             map[int32]map[*Client]bool
	adminClients        map[*Client]bool
	conversationClients map[int32]map[*Client]bool // map[ConversationID]map[*Client]bool
	mu                  sync.RWMutex
}

func NewHub() Hub {
	return &hub{
		clients:             make(map[int32]map[*Client]bool),
		adminClients:        make(map[*Client]bool),
		conversationClients: make(map[int32]map[*Client]bool),
		mu:                  sync.RWMutex{},
	}
}

func (h *hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client.UserID]; !ok {
		h.clients[client.UserID] = make(map[*Client]bool)
	}
	h.clients[client.UserID][client] = true

	if client.IsAdmin {
		h.adminClients[client] = true
	}

	// If client joins a specific conversation room
	if client.ConversationID != nil {
		convID := *client.ConversationID
		if _, ok := h.conversationClients[convID]; !ok {
			h.conversationClients[convID] = make(map[*Client]bool)
		}
		h.conversationClients[convID][client] = true
	}

	logger.Log.Info().
		Int32("user_id", client.UserID).
		Int("active_connections_for_user", len(h.clients[client.UserID])).
		Msg("WebSocket client registered")
}

func (h *hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if userClients, exists := h.clients[client.UserID]; exists {
		delete(userClients, client)
		client.Close()
		if len(userClients) == 0 {
			delete(h.clients, client.UserID)
		}
	}
	if client.IsAdmin {
		delete(h.adminClients, client)
	}

	if client.ConversationID != nil {
		convID := *client.ConversationID
		if convClients, ok := h.conversationClients[convID]; ok {
			delete(convClients, client)
			if len(convClients) == 0 {
				delete(h.conversationClients, convID)
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

	// Clean all room mappings
	h.conversationClients = make(map[int32]map[*Client]bool)
	h.adminClients = make(map[*Client]bool)

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

func (h *hub) BroadcastToAll(msg WSMessage) {
	h.mu.RLock()
	if len(h.clients) == 0 {
		h.mu.RUnlock()
		logger.Log.Warn().Msg("WebSocket Broadcast: no active WebSocket connections")
		return
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		h.mu.RUnlock()
		logger.Log.Error().
			Err(err).
			Msg("Failed to marshal WebSocket message for broadcast to all")
		return
	}

	var targets []*Client
	for _, userClients := range h.clients {
		for client := range userClients {
			targets = append(targets, client)
		}
	}
	h.mu.RUnlock()

	logger.Log.Info().
		Int("target_connections", len(targets)).
		Str("event_type", string(msg.Type)).
		Msg("WebSocket broadcasting to ALL connections")

	for _, client := range targets {
		select {
		case client.Send <- payload:
		default:
			// Buffer full, unregister problematic client
			h.Unregister(client)
		}
	}
}

func (h *hub) BroadCastToAdmins(msg WSMessage) {
	h.mu.RLock()
	if len(h.adminClients) == 0 {
		h.mu.RUnlock()
		return
	}

	targets := make([]*Client, 0, len(h.adminClients))
	for client := range h.adminClients {
		targets = append(targets, client)
	}
	h.mu.RUnlock()

	payload, err := json.Marshal(msg)
	if err != nil {
		logger.Log.Error().
			Err(err).
			Msg("Failed to marshal WebSocket message for broadcast to admins")
		return
	}

	for _, client := range targets {
		select {
		case client.Send <- payload:
		default:
			h.Unregister(client)
		}
	}
}

// BroadcastToConversation sends a message to all active connections in a specific conversation room and all online admins
func (h *hub) BroadcastToConversation(conversationID int32, msg WSMessage) {
	h.mu.RLock()
	convClients := h.conversationClients[conversationID]

	// Collect unique targets: clients in the conversation room + all admin clients
	targetMap := make(map[*Client]bool)
	for client := range convClients {
		targetMap[client] = true
	}
	for admin := range h.adminClients {
		targetMap[admin] = true
	}

	if len(targetMap) == 0 {
		h.mu.RUnlock()
		logger.Log.Debug().
			Int32("conversation_id", conversationID).
			Str("event_type", string(msg.Type)).
			Msg("WebSocket BroadcastToConversation: no active clients or admins")
		return
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		h.mu.RUnlock()
		logger.Log.Error().
			Err(err).
			Int32("conversation_id", conversationID).
			Msg("Failed to marshal WebSocket message for broadcast to conversation")
		return
	}

	// Copy clients slice to minimize lock holding time
	targets := make([]*Client, 0, len(targetMap))
	for client := range targetMap {
		targets = append(targets, client)
	}
	h.mu.RUnlock()

	logger.Log.Info().
		Int32("conversation_id", conversationID).
		Int("target_connections", len(targets)).
		Str("event_type", string(msg.Type)).
		Msg("WebSocket broadcasting to conversation room and admins")

	for _, client := range targets {
		select {
		case client.Send <- payload:
		default:
			// Buffer full, unregister problematic client
			h.Unregister(client)
		}
	}
}
