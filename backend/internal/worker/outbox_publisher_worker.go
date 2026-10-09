package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"goshop/internal/repository"
	"goshop/pkg/rabbitmq"
	"time"

	"github.com/rs/zerolog"
)

type OutboxPublisherWorker struct {
	outboxRepo repository.OutboxRepository
	rabbitMQ   rabbitmq.RabbitMQService
	interval   time.Duration
	batchSize  int32
	logger     *zerolog.Logger
}

func NewOutboxPublisherWorker(
	outboxRepo repository.OutboxRepository,
	rabbitMQ rabbitmq.RabbitMQService,
	interval time.Duration,
	batchSize int32,
	logger *zerolog.Logger,
) *OutboxPublisherWorker {
	if interval <= 0 {
		interval = 1 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	return &OutboxPublisherWorker{
		outboxRepo: outboxRepo,
		rabbitMQ:   rabbitMQ,
		interval:   interval,
		batchSize:  batchSize,
		logger:     logger,
	}
}

// ProcessBatch fetches pending outbox events for order and payment aggregates,
// publishes each to RabbitMQ using the event type as queue/topic, and marks them processed.
func (w *OutboxPublisherWorker) ProcessBatch(ctx context.Context) (int, error) {
	if w.rabbitMQ == nil || w.outboxRepo == nil {
		return 0, nil
	}

	events, err := w.outboxRepo.GetPendingByAggregateTypes(ctx, []string{"order", "payment"}, w.batchSize)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch pending order/payment outbox events: %w", err)
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
			_ = w.outboxRepo.MarkFailed(ctx, ev.EventID, &errMsg)
			continue
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
		Msg("Starting Transactional Outbox Publisher worker (order/payment)")

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
