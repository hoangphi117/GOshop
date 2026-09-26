package ws

import (
	"context"
	"encoding/json"
	"goshop/pkg/logger"
	"sync"

	"github.com/redis/go-redis/v9"
)

const NotificationRedisChannel = "channel:ws:notifications"

// RedisNotificationEnvelop is the cross-node payload for horizontal delivery
type RedisNotificationEnvelope struct {
	UserID  *int32    `json:"user_id,omitempty"`
	IsAdmin bool      `json:"is_admin,omitempty"`
	IsAll   bool      `json:"is_all,omitempty"`
	Message WSMessage `json:"message"`
}

type RedisBridge interface {
	Publish(ctx context.Context, envelope RedisNotificationEnvelope) error
	StartSubscriber(ctx context.Context)
	Stop()
}

type redisBridge struct {
	rdb    *redis.Client
	hub    Hub
	pubsub *redis.PubSub
	mu     sync.Mutex
	stopCh chan struct{}
}

func NewRedisBridge(rdb *redis.Client, hub Hub) RedisBridge {
	return &redisBridge{
		rdb:    rdb,
		hub:    hub,
		stopCh: make(chan struct{}),
	}
}

func (rb *redisBridge) Publish(ctx context.Context, envelope RedisNotificationEnvelope) error {
	if rb.rdb == nil {
		return nil // Redis not configured, skip publishing
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return err
	}

	return rb.rdb.Publish(ctx, NotificationRedisChannel, data).Err()
}

func (rb *redisBridge) StartSubscriber(ctx context.Context) {
	if rb.rdb == nil || rb.hub == nil {
		return // Redis or Hub not configured, skip subscribing
	}

	rb.mu.Lock()
	rb.pubsub = rb.rdb.Subscribe(ctx, NotificationRedisChannel)
	rb.mu.Unlock()

	ch := rb.pubsub.Channel()
	logger.Log.Info().Str("channel", NotificationRedisChannel).Msg("Redis Pub/Sub WebSocket subscriber started")

	for {
		select {
		case <-ctx.Done():
			rb.Stop()
			return
		case <-rb.stopCh:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}

			var envelope RedisNotificationEnvelope
			if err := json.Unmarshal([]byte(msg.Payload), &envelope); err != nil {
				logger.Log.Error().Err(err).Msg("Failed to unmarshal Redis Pub/Sub notification envelope")
				continue
			}

			if envelope.IsAll {
				rb.hub.BroadcastToAll(envelope.Message)
			} else if envelope.IsAdmin || envelope.UserID == nil {
				rb.hub.BroadCastToAdmins(envelope.Message)
			} else {
				if envelope.UserID != nil {
					rb.hub.BroadcastToUser(*envelope.UserID, envelope.Message)
				}
			}
		}
	}
}

func (rb *redisBridge) Stop() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	select {
	case <-rb.stopCh:
	default:
		close(rb.stopCh)
	}

	if rb.pubsub != nil {
		if err := rb.pubsub.Close(); err != nil {
			logger.Log.Error().Err(err).Msg("Failed to close Redis Pub/Sub")
			rb.pubsub = nil
		}
	}
}
