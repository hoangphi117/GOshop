package service

import (
	"context"
	"errors"
	"testing"
	"time"

	catalogsqlc "goshop/services/catalog/internal/db/sqlc"

	"github.com/google/uuid"
)

// The outbox worker is what turns enqueued rows into indexed documents. Both
// regressions below shipped and cost 329 of 5000 products their search entry.

// A failed MarkOutboxEventProcessed used to be discarded with `_ =`, so a row
// that never left PENDING looked identical to one that did. ProcessOutboxBatch
// itself still succeeds (the next tick retries), but the failure must not be
// silently swallowed on the way out.
func TestProcessOutboxBatch_MarkFailureDoesNotAbortBatch(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	// INITIAL_STOCK_SET with no inventory client fails the stock init before
	// any marking, so the deletion path is what drives this batch. Use an
	// unknown event type instead: the switch marks it processed with no side
	// effects, which isolates the marking loop.
	pending := []catalogsqlc.OutboxEvent{
		{EventID: 1, EventType: "UNKNOWN_EVENT", AggregateID: first.String()},
		{EventID: 2, EventType: "UNKNOWN_EVENT", AggregateID: second.String()},
	}
	repo := &mockRepo{
		getPendingOutbox: func(context.Context, int32) ([]catalogsqlc.OutboxEvent, error) {
			return pending, nil
		},
		markProcessed: func(_ context.Context, eventID int64) error {
			if eventID == 1 {
				return errors.New("connection reset")
			}
			return nil
		},
	}

	svc := newWorkflowService(repo, &mockInventory{})
	if _, err := svc.ProcessOutboxBatch(context.Background()); err != nil {
		t.Fatalf("batch must not fail because one mark failed: %v", err)
	}
	if len(repo.markedEventIDs) != 2 {
		t.Fatalf("a mark error must not skip the remaining ids, marked %v", repo.markedEventIDs)
	}
}

// The worker loop must not start a second batch while one is running. Ticks
// used to call ProcessOutboxBatch straight from the select, so a bulk enqueue
// had overlapping ticks re-reading rows that were not yet marked.
func TestStartOutboxWorker_DoesNotOverlapBatches(t *testing.T) {
	var inFlight, maxInFlight int
	repo := &mockRepo{
		getPendingOutbox: func(context.Context, int32) ([]catalogsqlc.OutboxEvent, error) {
			inFlight++
			if inFlight > maxInFlight {
				maxInFlight = inFlight
			}
			// Hold the batch long enough for at least one more tick to fire.
			time.Sleep(250 * time.Millisecond)
			inFlight--
			return nil, nil
		},
	}

	svc := newWorkflowService(repo, &mockInventory{})
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()

	svc.StartOutboxWorker(ctx)
	<-ctx.Done()
	// Let the goroutine observe ctx.Done and exit before asserting.
	time.Sleep(100 * time.Millisecond)

	if maxInFlight > 1 {
		t.Fatalf("worker ran %d batches concurrently; ticks must be serialised", maxInFlight)
	}
}
