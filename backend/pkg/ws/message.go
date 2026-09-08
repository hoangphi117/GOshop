package ws

type EventType string

const (
	EventTypeNotification EventType = "notification"
	EventPing             EventType = "ping"
	EventPong             EventType = "pong"
)

type WSMessage struct {
	Type EventType `json:"type"`
	Data any       `json:"data"`
}
