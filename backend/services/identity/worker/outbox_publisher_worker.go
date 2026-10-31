package identityworker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"goshop/pkg/messaging"
	identitysqlc "goshop/services/identity/internal/db/sqlc"
)

// outboxStore is the slice of the identity outbox repository this worker needs.
// Declared here (not shared) so the worker owns no cross-service contract and can
// be tested with a local fake.
type outboxStore interface {
	GetPendingByAggregateTypes(ctx context.Context, aggregateTypes []string, limit int32) ([]identitysqlc.OutboxEvent, error)
	MarkProcessed(ctx context.Context, eventIDs []int64) error
	MarkFailed(ctx context.Context, eventID int64, lastErr *string) error
}

type OutboxPublisherWorker struct {
	outboxRepo     outboxStore
	rabbitMQ       messaging.Broker
	interval       time.Duration
	batchSize      int32
	aggregateTypes []string
	logger         *zerolog.Logger
}

func NewOutboxPublisherWorker(
	outboxRepo outboxStore,
	rabbitMQ messaging.Broker,
	interval time.Duration,
	batchSize int32,
	logger *zerolog.Logger,
) *OutboxPublisherWorker {
	return NewOutboxPublisherWorkerWithAggregates(outboxRepo, rabbitMQ, interval, batchSize, logger, []string{"user"})
}

func NewOutboxPublisherWorkerWithAggregates(
	outboxRepo outboxStore,
	rabbitMQ messaging.Broker,
	interval time.Duration,
	batchSize int32,
	logger *zerolog.Logger,
	aggregateTypes []string,
) *OutboxPublisherWorker {
	if interval <= 0 {
		interval = 1 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	return &OutboxPublisherWorker{
		outboxRepo:     outboxRepo,
		rabbitMQ:       rabbitMQ,
		interval:       interval,
		batchSize:      batchSize,
		aggregateTypes: aggregateTypes,
		logger:         logger,
	}
}

// ProcessBatch fetches pending outbox events for configured aggregate types,
// publishes each to RabbitMQ using the event type as queue/topic, and marks them processed.
func (w *OutboxPublisherWorker) ProcessBatch(ctx context.Context) (int, error) {
	if w.rabbitMQ == nil || w.outboxRepo == nil {
		return 0, nil
	}

	types := w.aggregateTypes
	if len(types) == 0 {
		types = []string{"user"}
	}

	events, err := w.outboxRepo.GetPendingByAggregateTypes(ctx, types, w.batchSize)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch pending identity outbox events: %w", err)
	}

	if len(events) == 0 {
		return 0, nil
	}

	processedIDs := make([]int64, 0, len(events))

	for _, ev := range events {
		queue := ev.EventType
		if queue == "" {
			w.logger.Warn().Int64("event_id", ev.EventID).Msg("Skipping outbox event with empty event_type")
			processedIDs = append(processedIDs, ev.EventID)
			continue
		}

		rawPayload := json.RawMessage(ev.Payload)
		if err := w.rabbitMQ.Publish(ctx, queue, rawPayload); err != nil {
			w.logger.Error().
				Err(err).
				Int64("event_id", ev.EventID).
				Str("queue", queue).
				Str("aggregate_type", ev.AggregateType).
				Str("aggregate_id", ev.AggregateID).
				Msg("Failed to publish outbox event to RabbitMQ")

			errMsg := err.Error()
			if markErr := w.outboxRepo.MarkFailed(ctx, ev.EventID, &errMsg); markErr != nil {
				w.logger.Error().Err(markErr).Int64("event_id", ev.EventID).Msg("Failed to mark outbox event failed in db")
			}
			if ev.RetryCount+1 >= 5 {
				w.logger.Error().
					Int64("event_id", ev.EventID).
					Str("aggregate_type", ev.AggregateType).
					Str("aggregate_id", ev.AggregateID).
					Str("event_type", ev.EventType).
					Int32("retry_count", ev.RetryCount+1).
					Msg("CRITICAL: Outbox event reached max retry limit (5) and transitioned to FAILED dead-letter status")
			}
			continue
		}

		// Also broadcast to topic exchange for decoupled microservices (e.g. notification service)
		if topicErr := w.rabbitMQ.PublishTopic(ctx, queue, rawPayload); topicErr != nil {
			w.logger.Warn().Err(topicErr).Str("topic", queue).Msg("Failed to broadcast outbox event to topic exchange")
		}

		processedIDs = append(processedIDs, ev.EventID)
	}

	if len(processedIDs) > 0 {
		if err := w.outboxRepo.MarkProcessed(ctx, processedIDs); err != nil {
			w.logger.Error().Err(err).Int("count", len(processedIDs)).Msg("Failed to mark outbox events as processed")
			return len(processedIDs), err
		}
	}

	return len(processedIDs), nil
}

func (w *OutboxPublisherWorker) Start(ctx context.Context) {
	if w.rabbitMQ == nil || w.outboxRepo == nil {
		w.logger.Info().Msg("RabbitMQ or Outbox repo not configured, outbox publisher worker skipped")
		return
	}

	w.logger.Info().
		Dur("interval", w.interval).
		Int32("batch_size", w.batchSize).
		Msg("Starting Transactional Outbox Publisher worker (identity)")

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info().Msg("Stopping Transactional Outbox Publisher worker")
			return
		case <-ticker.C:
			count, err := w.ProcessBatch(ctx)
			if err != nil {
				w.logger.Error().Err(err).Msg("Error processing outbox events batch")
			} else if count > 0 {
				w.logger.Debug().Int("count", count).Msg("Published outbox events to RabbitMQ")
			}
		}
	}
}
