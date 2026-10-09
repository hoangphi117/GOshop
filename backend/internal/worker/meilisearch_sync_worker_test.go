package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"goshop/internal/db/sqlc"
	"goshop/internal/testutils"
	"goshop/internal/worker"
	"goshop/pkg/search"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMeilisearchSyncWorker_ProcessBatch(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	t.Run("Returns 0 when searchClient is nil", func(t *testing.T) {
		mockOutbox := &testutils.MockOutboxRepository{}
		w := worker.NewMeilisearchSyncWorker(mockOutbox, nil, time.Second, 10, &logger)

		count, err := w.ProcessBatch(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("Returns 0 when no pending events", func(t *testing.T) {
		mockOutbox := &testutils.MockOutboxRepository{
			GetPendingFunc: func(ctx context.Context, limit int32) ([]sqlc.OutboxEvent, error) {
				return []sqlc.OutboxEvent{}, nil
			},
		}
		// Create worker with non-nil search client placeholder
		client, _ := search.NewClient("http://localhost:7700", "testkey", "products")
		w := worker.NewMeilisearchSyncWorker(mockOutbox, client, time.Second, 10, &logger)

		count, err := w.ProcessBatch(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("Returns error when GetPending fails", func(t *testing.T) {
		mockOutbox := &testutils.MockOutboxRepository{
			GetPendingFunc: func(ctx context.Context, limit int32) ([]sqlc.OutboxEvent, error) {
				return nil, errors.New("db down")
			},
		}
		client, _ := search.NewClient("http://localhost:7700", "testkey", "products")
		w := worker.NewMeilisearchSyncWorker(mockOutbox, client, time.Second, 10, &logger)

		_, err := w.ProcessBatch(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch pending outbox events")
	})

	t.Run("Marks invalid payload as failed", func(t *testing.T) {
		failedMarked := false
		mockOutbox := &testutils.MockOutboxRepository{
			GetPendingFunc: func(ctx context.Context, limit int32) ([]sqlc.OutboxEvent, error) {
				return []sqlc.OutboxEvent{
					{
						EventID:       1,
						AggregateType: "product",
						AggregateID:   "p-1",
						EventType:     "PRODUCT_UPSERT",
						Payload:       []byte("invalid-json"),
					},
				}, nil
			},
			MarkFailedFunc: func(ctx context.Context, eventID int64, lastErr *string) error {
				if eventID == 1 {
					failedMarked = true
				}
				return nil
			},
			MarkProcessedFunc: func(ctx context.Context, ids []int64) error {
				return nil
			},
		}
		client, _ := search.NewClient("http://localhost:7700", "testkey", "products")
		w := worker.NewMeilisearchSyncWorker(mockOutbox, client, time.Second, 10, &logger)

		count, err := w.ProcessBatch(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
		assert.True(t, failedMarked)
	})

	t.Run("Marks processed events properly", func(t *testing.T) {
		processedIDs := []int64{}
		mockDoc := search.ProductDocument{
			ID:   "p-100",
			Name: "Product Test",
		}
		docBytes, _ := json.Marshal(mockDoc)

		mockOutbox := &testutils.MockOutboxRepository{
			GetPendingFunc: func(ctx context.Context, limit int32) ([]sqlc.OutboxEvent, error) {
				return []sqlc.OutboxEvent{
					{
						EventID:       10,
						AggregateType: "product",
						AggregateID:   "p-100",
						EventType:     "UNKNOWN_EVENT",
						Payload:       docBytes,
					},
				}, nil
			},
			MarkProcessedFunc: func(ctx context.Context, ids []int64) error {
				processedIDs = ids
				return nil
			},
		}
		client, _ := search.NewClient("http://localhost:7700", "testkey", "products")
		w := worker.NewMeilisearchSyncWorker(mockOutbox, client, time.Second, 10, &logger)

		count, err := w.ProcessBatch(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		assert.Equal(t, []int64{10}, processedIDs)
	})
}
