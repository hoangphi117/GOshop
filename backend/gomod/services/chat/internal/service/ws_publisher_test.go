package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	ws "goshop/shared/pkg/ws"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// Regression: PublishToConversation must deliver a JSON envelope. Passing the
// struct directly makes go-redis stringify it with fmt.Sprintf, which the
// ws-gateway bridge fails to json.Unmarshal — chat realtime events were
// silently dropped.
func TestRedisWSPublisher_PublishesJSONEnvelope(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	defer func() { _ = rdb.Close() }()

	const conversationID = int32(987654)
	channel := ws.ChatRedisChannelPrefix + "987654"
	pubsub := rdb.Subscribe(ctx, channel)
	defer func() { _ = pubsub.Close() }()
	if _, err := pubsub.Receive(ctx); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	publisher := NewRedisWSPublisher(rdb, zerolog.Nop())
	publisher.PublishToConversation(context.Background(), conversationID, string(ws.EventChatMessage), map[string]any{"text": "hi"})

	select {
	case m := <-pubsub.Channel():
		var envelope ws.RedisNotificationEnvelope
		if err := json.Unmarshal([]byte(m.Payload), &envelope); err != nil {
			t.Fatalf("payload is not valid JSON (bridge would drop it): %q: %v", m.Payload, err)
		}
		if envelope.ConversationID == nil || *envelope.ConversationID != conversationID {
			t.Fatalf("envelope conversation_id = %v, want %d", envelope.ConversationID, conversationID)
		}
		if envelope.Message.Type != ws.EventChatMessage {
			t.Fatalf("envelope type = %q, want %q", envelope.Message.Type, ws.EventChatMessage)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for published envelope")
	}
}
