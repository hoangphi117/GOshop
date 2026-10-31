package worker

import (
	"context"
	"goshop/services/order/internal/repository"
	orderservice "goshop/services/order/internal/service"
	"time"

	"github.com/rs/zerolog"
)

type SagaRecoveryWorker struct {
	sagaRepo     repository.SagaRepository
	orchestrator orderservice.OrderSagaOrchestrator
	interval     time.Duration
	batchSize    int32
	logger       *zerolog.Logger
}

func NewSagaRecoveryWorker(
	sagaRepo repository.SagaRepository,
	orchestrator orderservice.OrderSagaOrchestrator,
	interval time.Duration,
	batchSize int32,
	logger *zerolog.Logger,
) *SagaRecoveryWorker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	return &SagaRecoveryWorker{
		sagaRepo:     sagaRepo,
		orchestrator: orchestrator,
		interval:     interval,
		batchSize:    batchSize,
		logger:       logger,
	}
}

// ProcessBatch claims stale or retry-due sagas using FOR UPDATE SKIP LOCKED
// and executes backwards compensation on each.
func (w *SagaRecoveryWorker) ProcessBatch(ctx context.Context) (int, error) {
	if w.sagaRepo == nil || w.orchestrator == nil {
		return 0, nil
	}

	staleSagas, err := w.sagaRepo.ClaimStale(ctx, nil, w.batchSize)
	if err != nil {
		return 0, err
	}

	if len(staleSagas) == 0 {
		return 0, nil
	}

	for _, saga := range staleSagas {
		reason := "Stale saga recovered by background compensation worker"
		if saga.FailureReason != nil && *saga.FailureReason != "" {
			reason = *saga.FailureReason
		}

		if err := w.orchestrator.CompensateSaga(ctx, saga.OrderUuid, reason); err != nil {
			if w.logger != nil {
				w.logger.Warn().
					Err(err).
					Str("order_uuid", saga.OrderUuid.String()).
					Int32("attempts", saga.Attempts).
					Msg("Saga compensation retry scheduled by recovery worker")
			}
		} else {
			if w.logger != nil {
				w.logger.Info().
					Str("order_uuid", saga.OrderUuid.String()).
					Msg("Stale saga successfully compensated and marked failed by recovery worker")
			}
		}
	}

	return len(staleSagas), nil
}

func (w *SagaRecoveryWorker) Start(ctx context.Context) {
	if w.sagaRepo == nil || w.orchestrator == nil {
		if w.logger != nil {
			w.logger.Info().Msg("Saga repository or orchestrator not configured, saga recovery worker skipped")
		}
		return
	}

	if w.logger != nil {
		w.logger.Info().
			Dur("interval", w.interval).
			Int32("batch_size", w.batchSize).
			Msg("Starting Background Saga Recovery & Compensation worker")
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if w.logger != nil {
				w.logger.Info().Msg("Stopping Background Saga Recovery & Compensation worker")
			}
			return
		case <-ticker.C:
			count, err := w.ProcessBatch(ctx)
			if err != nil {
				if w.logger != nil {
					w.logger.Error().Err(err).Msg("Error processing stale sagas batch in recovery worker")
				}
			} else if count > 0 {
				if w.logger != nil {
					w.logger.Info().Int("count", count).Msg("Processed and compensated stale sagas")
				}
			}
		}
	}
}
