package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"goshop/pkg/logger"
	"goshop/pkg/rabbitmq"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

// UserSyncConsumer handles user lifecycle events from the Identity Service (Eventual Consistency)
type UserSyncConsumer struct {
	redisClient *redis.Client
}

func NewUserSyncConsumer(redisClient *redis.Client) *UserSyncConsumer {
	return &UserSyncConsumer{
		redisClient: redisClient,
	}
}

// Handle processes incoming user sync events and invalidates stale caches across services
func (c *UserSyncConsumer) Handle(headers amqp.Table, body []byte) error {
	var event rabbitmq.UserSyncEvent
	if err := json.Unmarshal(body, &event); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to unmarshal user sync event")
		return err
	}

	logger.Log.Info().
		Str("event_type", event.EventType).
		Int32("user_id", event.UserID).
		Str("user_uuid", event.UserUUID).
		Msg("Processing user sync event from Identity Service")

	if c.redisClient != nil && event.UserID > 0 {
		cacheKey := fmt.Sprintf("user:profile:%d", event.UserID)
		if err := c.redisClient.Del(context.Background(), cacheKey).Err(); err != nil {
			logger.Log.Warn().Err(err).Str("cache_key", cacheKey).Msg("Failed to invalidate user profile cache")
			return err
		}
		logger.Log.Info().Str("cache_key", cacheKey).Msg("Successfully invalidated user profile cache on event")
	}

	return nil
}
