package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"goshop/internal/repository"
	"goshop/pkg/search"
	"time"

	"github.com/rs/zerolog"
)

type MeilisearchSyncWorker struct {
	outboxRepo   repository.OutboxRepository
	searchClient *search.Client
	interval     time.Duration
	batchSize    int32
	logger       *zerolog.Logger
}

func NewMeilisearchSyncWorker(
	outboxRepo repository.OutboxRepository,
	searchClient *search.Client,
	interval time.Duration,
	batchSize int32,
	logger *zerolog.Logger,
) *MeilisearchSyncWorker {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	return &MeilisearchSyncWorker{
		outboxRepo:   outboxRepo,
		searchClient: searchClient,
		interval:     interval,
		batchSize:    batchSize,
		logger:       logger,
	}
}

func (w *MeilisearchSyncWorker) ProcessBatch(ctx context.Context) (int, error) {
	if w.searchClient == nil {
		return 0, nil
	}

	events, err := w.outboxRepo.GetPendingByAggregateTypes(ctx, []string{"product"}, w.batchSize)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch pending outbox events: %w", err)
	}

	if len(events) == 0 {
		return 0, nil
	}

	upsertDocs := make([]search.ProductDocument, 0)
	deleteIDs := make([]string, 0)
	processedIDs := make([]int64, 0, len(events))

	for _, ev := range events {
		switch ev.EventType {
		case "PRODUCT_UPSERT":
			var doc search.ProductDocument
			if err := json.Unmarshal(ev.Payload, &doc); err != nil {
				w.logger.Error().Err(err).Int64("event_id", ev.EventID).Msg("Failed to unmarshal outbox product doc")
				errMsg := err.Error()
				_ = w.outboxRepo.MarkFailed(ctx, ev.EventID, &errMsg)
				continue
			}
			upsertDocs = append(upsertDocs, doc)
			processedIDs = append(processedIDs, ev.EventID)

		case "PRODUCT_DELETE":
			deleteIDs = append(deleteIDs, ev.AggregateID)
			processedIDs = append(processedIDs, ev.EventID)

		default:
			processedIDs = append(processedIDs, ev.EventID)
		}
	}

	// 1. Process batch upserts to Meilisearch
	if len(upsertDocs) > 0 {
		if _, err := w.searchClient.IndexProducts(ctx, upsertDocs); err != nil {
			w.logger.Error().Err(err).Int("count", len(upsertDocs)).Msg("Failed to batch index products to Meilisearch")
			errMsg := err.Error()
			for _, id := range processedIDs {
				_ = w.outboxRepo.MarkFailed(ctx, id, &errMsg)
			}
			return 0, err
		}
	}

	// 2. Process deletes to Meilisearch
	for _, id := range deleteIDs {
		if _, err := w.searchClient.DeleteProduct(ctx, id); err != nil {
			w.logger.Warn().Err(err).Str("id", id).Msg("Failed to delete product from Meilisearch")
		}
	}

	// 3. Mark outbox events as PROCESSED
	if err := w.outboxRepo.MarkProcessed(ctx, processedIDs); err != nil {
		w.logger.Error().Err(err).Msg("Failed to mark outbox events as processed")
		return len(processedIDs), err
	}

	w.logger.Debug().Int("processed", len(processedIDs)).Msg("Synced outbox events to Meilisearch")
	return len(processedIDs), nil
}

func (w *MeilisearchSyncWorker) Start(ctx context.Context) {
	if w.searchClient == nil {
		w.logger.Info().Msg("Meilisearch client not configured, outbox sync worker skipped")
		return
	}

	w.logger.Info().Dur("interval", w.interval).Int32("batch_size", w.batchSize).Msg("Starting Meilisearch outbox sync worker")
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info().Msg("Stopping Meilisearch outbox sync worker")
			return
		case <-ticker.C:
			count, err := w.ProcessBatch(ctx)
			if err != nil {
				w.logger.Error().Err(err).Msg("Error processing Meilisearch outbox batch")
			} else if count > 0 {
				w.logger.Info().Int("count", count).Msg("Processed Meilisearch outbox events batch")
			}
		}
	}
}
