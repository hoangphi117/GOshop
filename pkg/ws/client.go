package ws

import (
	"bytes"
	"goshop/pkg/logger"
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
	Hub    Hub
	Conn   *websocket.Conn
	Send   chan []byte
	UserID int32
}

func NewClient(hub Hub, conn *websocket.Conn, userID int32) *Client {
	return &Client{
		Hub:    hub,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		UserID: userID,
	}
}

// ReadPump pumps messages from the websocket connection to the hub.
// Runs in its own goroutine for each connection.
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister(c)
		_ = c.Conn.Close()
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
		message = bytes.TrimSpace(bytes.ReplaceAll(message, newline, space))

		// Handle incoming messages if needed (e.g. client ping or chat)
		_ = message
	}
}

// WritePump pumps messages from the hub to the websocket connection.
// Runs in its own goroutine for each connection.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.Conn.Close()
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
			for i := 0; i < n; i++ {
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
