package service

import (
	"context"
	"encoding/json"
	"fmt"

	ws "goshop/pkg/ws"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// RedisWSPublisher publishes chat events on the per-conversation Redis channel
// that the ws-gateway bridge subscribes to. It replaces the in-process hub the
// monolith used: chat no longer owns sockets.
type RedisWSPublisher struct {
	rdb    *redis.Client
	logger zerolog.Logger
}

func NewRedisWSPublisher(rdb *redis.Client, logger zerolog.Logger) *RedisWSPublisher {
	return &RedisWSPublisher{rdb: rdb, logger: logger}
}

func (p *RedisWSPublisher) PublishToConversation(ctx context.Context, conversationID int32, msgType string, payload any) {
	if p.rdb == nil {
		return
	}
	conversationIDCopy := conversationID
	envelope := ws.RedisNotificationEnvelope{
		ConversationID: &conversationIDCopy,
		Message: ws.WSMessage{
			Type: ws.EventType(msgType),
			Data: payload,
		},
	}
	channel := fmt.Sprintf("%s%d", ws.ChatRedisChannelPrefix, conversationID)
	// go-redis stringifies non-string args with fmt.Sprintf — the bridge json.Unmarshal's
	// the payload, so the envelope MUST arrive as JSON bytes (monolith parity: it used
	// redisBridge.PublishToConversation which marshals internally).
	data, err := json.Marshal(envelope)
	if err != nil {
		p.logger.Error().Err(err).
			Int32("conversation_id", conversationID).
			Str("event_type", msgType).
			Msg("Failed to marshal chat event envelope")
		return
	}
	if err := p.rdb.Publish(ctx, channel, data).Err(); err != nil {
		p.logger.Error().Err(err).
			Int32("conversation_id", conversationID).
			Str("event_type", msgType).
			Msg("Failed to publish chat event")
	}
}
