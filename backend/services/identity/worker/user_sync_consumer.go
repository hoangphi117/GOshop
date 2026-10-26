package identityworker

import (
	"context"
	"encoding/json"
	"fmt"
	"goshop/pkg/logger"
	"goshop/pkg/rabbitmq"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

// UserSyncConsumer handles user lifecycle events from the Identity Service (Eventual Consistency & Projection Sync)
type UserSyncConsumer struct {
	redisClient *redis.Client
	dbPool      *pgxpool.Pool
}

func NewUserSyncConsumer(redisClient *redis.Client, dbPool ...*pgxpool.Pool) *UserSyncConsumer {
	var pool *pgxpool.Pool
	if len(dbPool) > 0 {
		pool = dbPool[0]
	}
	return &UserSyncConsumer{
		redisClient: redisClient,
		dbPool:      pool,
	}
}

// Handle processes incoming user sync events: invalidates Redis cache and upserts minimal projection to core DB
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

	// 1. Invalidate Redis single user profile cache (by ID and UUID)
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

	// 2. Synchronize minimal user projection into Core DB (shopping)
	if c.dbPool != nil && event.UserID > 0 {
		ctx := context.Background()
		var deletedAt *time.Time
		if event.EventType == rabbitmq.QueueUserDeleted {
			now := time.Now().UTC()
			deletedAt = &now
		}

		userUUID, err := uuid.Parse(event.UserUUID)
		if err != nil {
			logger.Log.Error().Err(err).Str("user_uuid", event.UserUUID).Msg("Invalid user UUID in user sync event")
			return err
		}

		const query = `
			INSERT INTO users (
				user_id,
				user_uuid,
				user_fullname,
				user_email,
				user_avatar,
				user_avatar_url,
				user_status,
				user_level,
				user_deleted_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (user_id) DO UPDATE SET
				user_fullname = EXCLUDED.user_fullname,
				user_email = EXCLUDED.user_email,
				user_avatar = EXCLUDED.user_avatar,
				user_avatar_url = EXCLUDED.user_avatar_url,
				user_status = EXCLUDED.user_status,
				user_level = EXCLUDED.user_level,
				user_deleted_at = EXCLUDED.user_deleted_at,
				user_updated_at = now();
		`

		if _, err := c.dbPool.Exec(ctx, query,
			event.UserID,
			userUUID,
			event.UserFullname,
			event.UserEmail,
			event.UserAvatar,
			event.UserAvatarUrl,
			event.UserStatus,
			event.UserLevel,
			deletedAt,
		); err != nil {
			logger.Log.Error().Err(err).Int32("user_id", event.UserID).Msg("Failed to upsert user projection into core DB")
			return err
		}

		logger.Log.Info().
			Int32("user_id", event.UserID).
			Str("user_uuid", event.UserUUID).
			Str("event_type", event.EventType).
			Msg("Successfully synchronized user projection to core DB")
	}

	return nil
}
