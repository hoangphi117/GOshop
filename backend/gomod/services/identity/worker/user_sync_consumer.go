package identityworker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"

	"goshop/contracts/events"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/messaging"
)

// UserSyncConsumer handles user lifecycle events from the Identity Service:
// it invalidates cached user profiles so readers pick up the change. User data
// itself lives only in the identity service database; the monolith users
// projection was dropped (migration 000002).
type UserSyncConsumer struct {
	redisClient *redis.Client
}

func NewUserSyncConsumer(redisClient *redis.Client) *UserSyncConsumer {
	return &UserSyncConsumer{
		redisClient: redisClient,
	}
}

// Handle processes incoming user sync events by invalidating the cached user profile.
func (c *UserSyncConsumer) Handle(headers messaging.Header, body []byte) error {
	var event events.UserSyncEvent
	if err := json.Unmarshal(body, &event); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to unmarshal user sync event")
		return err
	}

	logger.Log.Info().
		Str("event_type", event.EventType).
		Int32("user_id", event.UserID).
		Str("user_uuid", event.UserUUID).
		Msg("Processing user sync event from Identity Service")

	// Invalidate Redis single user profile cache (by ID and UUID)
	if c.redisClient != nil {
		if event.UserID > 0 {
			cacheKey := fmt.Sprintf("user:profile:%d", event.UserID)
			_ = c.redisClient.Del(context.Background(), cacheKey).Err()
			logger.Log.Info().Str("cache_key", cacheKey).Msg("Successfully invalidated user profile cache on event")
		}
		if event.UserUUID != "" {
			cacheKeyUUID := fmt.Sprintf("user:profile:uuid:%s", event.UserUUID)
			_ = c.redisClient.Del(context.Background(), cacheKeyUUID).Err()
			logger.Log.Info().Str("cache_key", cacheKeyUUID).Msg("Successfully invalidated user profile uuid cache on event")
		}
	}

	return nil
}
