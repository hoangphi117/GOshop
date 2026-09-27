package ws

import (
	"encoding/json"
	"goshop/pkg/logger"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait = 10 * time.Second // time allowed to write a message to the peer

	pongWait = 60 * time.Second // time allowed to read the next pong message from the peer

	pingPeriod = (pongWait * 9) / 10 // send pings to peer with this period. Must be less than pongWait

	maxMessageSize = 4096 // maximum message size allowed from peer
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

type Client struct {
	Hub            Hub
	Conn           *websocket.Conn
	Send           chan []byte
	UserID         int32
	IsAdmin        bool
	ConversationID *int32 // nil: general listener | non-nil: active in conversation room
	closeOnce      sync.Once
}

func NewClient(hub Hub, conn *websocket.Conn, userID int32, isAdmin bool) *Client {
	return &Client{
		Hub:     hub,
		Conn:    conn,
		Send:    make(chan []byte, 256),
		UserID:  userID,
		IsAdmin: isAdmin,
	}
}

// Close gracefully and idempotently closes the Send channel and underlying websocket connection
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.Send)
		if c.Conn != nil {
			_ = c.Conn.Close()
		}
	})
}

// ReadPump pumps messages from the websocket connection to the hub.
// Runs in its own goroutine for each connection.
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister(c)
		c.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Log.Warn().Err(err).Int32("user_id", c.UserID).Msg("WebSocket unexpected close error")
			}
			break
		}
		var inbound struct {
			Type EventType       `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(message, &inbound); err == nil {
			switch inbound.Type {
			case EventPing:
				pongMsg, _ := json.Marshal(WSMessage{Type: EventPong})
				select {
				case c.Send <- pongMsg:
				default:
				}
			case EventChatTyping:
				var typingData struct {
					ConversationUUID string `json:"conversation_uuid"`
					IsTyping         bool   `json:"is_typing"`
				}
				if err := json.Unmarshal(inbound.Data, &typingData); err == nil {
					senderRole := "user"
					if c.IsAdmin {
						senderRole = "admin"
					}
					outMsg := WSMessage{
						Type: EventChatTyping,
						Data: ChatTypingPayload{
							ConversationUUID: typingData.ConversationUUID,
							UserID:           c.UserID,
							SenderRole:       senderRole,
							IsTyping:         typingData.IsTyping,
						},
					}

					if c.ConversationID != nil {
						c.Hub.BroadcastToConversation(*c.ConversationID, outMsg)
					} else {
						c.Hub.BroadCastToAdmins(outMsg)
					}
				}
			}
		}
	}
}

// WritePump pumps messages from the hub to the websocket connection.
// Runs in its own goroutine for each connection.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			_, _ = w.Write(message)

			n := len(c.Send)
			for range n {
				_, _ = w.Write(newline)
				_, _ = w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
