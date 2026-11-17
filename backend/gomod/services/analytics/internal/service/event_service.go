package service

import (
	"context"
	"time"

	analyticsdto "goshop/services/analytics/internal/dto"
	"goshop/services/analytics/internal/repository"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/messaging"
)

// EventService ingests interaction telemetry and serves search history.
type EventService interface {
	Ingest(ctx context.Context, anonID string, userID int32, events []analyticsdto.EngagementEventInput) (int, error)
	SearchHistory(ctx context.Context, anonID string, userID int32, limit int32) ([]repository.SearchHistoryEntry, error)
	DismissSearch(ctx context.Context, anonID string, userID int32, query string) error
	DismissAllSearches(ctx context.Context, anonID string, userID int32) error
}

// MaxEventsPerBatch bounds one request so a retry storm cannot flood the log.
const MaxEventsPerBatch = 50

// eventMaxSkew drops events stamped far in the future (a device with a broken
// clock) without dropping a slightly-ahead one.
const eventMaxSkew = 5 * time.Minute

type eventService struct {
	repo   repository.EventRepository
	broker messaging.Broker
}

func NewEventService(repo repository.EventRepository, broker messaging.Broker) EventService {
	return &eventService{repo: repo, broker: broker}
}

// publishTelemetry fans the stored events out to the personalization consumer
// that owns affinity and popularity. Kafka trouble must not lose telemetry:
// the rows are already durable, so a publish error is logged and the request
// still succeeds.
func (s *eventService) publishTelemetry(ctx context.Context, events []repository.EngagementEvent) {
	if s.broker == nil || len(events) == 0 {
		return
	}
	for _, e := range events {
		if err := s.broker.PublishTopic(ctx, e.EventType+".event", e); err != nil {
			logger.Log.Warn().Err(err).Str("event_type", e.EventType).Msg("Failed to publish engagement event")
			return
		}
	}
}
