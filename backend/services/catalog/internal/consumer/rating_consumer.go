package consumer

import (
	"context"
	"encoding/json"
	"strconv"

	"goshop/contracts/events"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/messaging"
	catalogsqlc "goshop/services/catalog/internal/db/sqlc"
)

// RatingConsumer keeps the denormalized products.product_avg_rating /
// product_review_count columns in sync with the Review service, which owns the
// reviews tables in its own database and cannot update them transactionally.
type RatingConsumer struct {
	querier catalogsqlc.Querier
	cache   cache.RedisCacheService
}

func NewRatingConsumer(querier catalogsqlc.Querier, cache cache.RedisCacheService) *RatingConsumer {
	return &RatingConsumer{querier: querier, cache: cache}
}

// HandleEvent decodes a ReviewRatingUpdatedEvent and applies it.
func (c *RatingConsumer) HandleEvent(_ messaging.Header, body []byte) error {
	var event events.ReviewRatingUpdatedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		// Malformed payload will never succeed on retry; ack it so the queue drains.
		logger.Log.Warn().Err(err).Msg("Dropping malformed review rating event")
		return nil
	}

	if event.ProductID <= 0 {
		logger.Log.Warn().Msg("Dropping review rating event without product_id")
		return nil
	}

	if err := c.querier.UpdateProductRating(context.Background(), catalogsqlc.UpdateProductRatingParams{
		ProductID:          event.ProductID,
		Column2:            strconv.FormatFloat(event.AvgRating, 'f', 1, 64),
		ProductReviewCount: int32(event.ReviewCount),
	}); err != nil {
		logger.Log.Error().Err(err).Int32("product_id", event.ProductID).Msg("Failed to apply review rating to product")
		return err
	}

	// Product listings cache product_avg_rating — invalidate after the update.
	if c.cache != nil {
		_ = c.cache.Clear("products:*")
	}

	return nil
}
