package app

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/messaging"
)

// withIdempotency marks a message id as processed for 24h in Redis so redelivered
// order events do not double-apply to the read model.
func withIdempotency(cacheService cache.RedisCacheService, logger *zerolog.Logger) func(string, messaging.Handler) messaging.Handler {
	const dedupTTL = 24 * time.Hour

	return func(queue string, next messaging.Handler) messaging.Handler {
		return func(headers messaging.Header, body []byte) error {
			messageID := headers.Get("x-message-id")
			if messageID == "" {
				return next(headers, body)
			}

			key := fmt.Sprintf("dedup:%s:%s", queue, messageID)

			if exists, err := cacheService.Exists(key); err == nil && exists {
				logger.Info().Str("message_id", messageID).Msg("Duplicate message, skipping")
				return nil
			}

			if err := next(headers, body); err != nil {
				return err
			}

			if _, err := cacheService.SetNX(key, "1", dedupTTL); err != nil {
				logger.Error().Err(err).Str("message_id", messageID).Msg("Failed to mark message as processed")
			}

			return nil
		}
	}
}
