package identityworker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goshop/contracts/events"
	"goshop/internal/testutils"
	identitysqlc "goshop/services/identity/internal/db/sqlc"
	identityworker "goshop/services/identity/worker"
)

// fakeOutbox implements the identityworker.outboxStore contract for tests,
// backed by identitysqlc.OutboxEvent (not the legacy core sqlc types).
type fakeOutbox struct {
	events    []identitysqlc.OutboxEvent
	fetchErr  error
	markedIDs []int64
	failedIDs []int64
}

func (f *fakeOutbox) GetPendingByAggregateTypes(_ context.Context, _ []string, _ int32) ([]identitysqlc.OutboxEvent, error) {
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	return f.events, nil
}

func (f *fakeOutbox) MarkProcessed(_ context.Context, eventIDs []int64) error {
	f.markedIDs = append(f.markedIDs, eventIDs...)
	return nil
}

func (f *fakeOutbox) MarkFailed(_ context.Context, eventID int64, _ *string) error {
	f.failedIDs = append(f.failedIDs, eventID)
	return nil
}

func TestOutboxPublisherWorker_ProcessBatch(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	t.Run("Returns 0 when dependencies are nil", func(t *testing.T) {
		w1 := identityworker.NewOutboxPublisherWorker(nil, &testutils.MockRabbitMQService{}, time.Second, 10, &logger)
		count, err := w1.ProcessBatch(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, count)

		w2 := identityworker.NewOutboxPublisherWorker(&fakeOutbox{}, nil, time.Second, 10, &logger)
		count, err = w2.ProcessBatch(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("Returns 0 when no pending events", func(t *testing.T) {
		store := &fakeOutbox{}
		w := identityworker.NewOutboxPublisherWorker(store, &testutils.MockRabbitMQService{}, time.Second, 10, &logger)
		count, err := w.ProcessBatch(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("Returns error when fetch fails", func(t *testing.T) {
		store := &fakeOutbox{fetchErr: errors.New("db error")}
		w := identityworker.NewOutboxPublisherWorker(store, &testutils.MockRabbitMQService{}, time.Second, 10, &logger)
		_, err := w.ProcessBatch(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch pending identity outbox events")
	})

	t.Run("Successfully publishes events and marks them processed", func(t *testing.T) {
		publishedQueues := make([]string, 0)
		rabbit := &testutils.MockRabbitMQService{
			PublishFunc: func(_ context.Context, queue string, _ any) error {
				publishedQueues = append(publishedQueues, queue)
				return nil
			},
		}
		store := &fakeOutbox{
			events: []identitysqlc.OutboxEvent{
				{EventID: 1, AggregateType: "user", AggregateID: "u-1", EventType: events.QueueUserCreated, Payload: []byte(`{"user_id":1}`)},
				{EventID: 2, AggregateType: "user", AggregateID: "u-2", EventType: events.QueueUserUpdated, Payload: []byte(`{"user_id":2}`)},
			},
		}

		w := identityworker.NewOutboxPublisherWorker(store, rabbit, time.Second, 10, &logger)
		count, err := w.ProcessBatch(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
		assert.Equal(t, []int64{1, 2}, store.markedIDs)
		assert.Equal(t, []string{events.QueueUserCreated, events.QueueUserUpdated}, publishedQueues)
	})

	t.Run("Marks failed when RabbitMQ publish returns error", func(t *testing.T) {
		rabbit := &testutils.MockRabbitMQService{
			PublishFunc: func(_ context.Context, queue string, _ any) error {
				if queue == events.QueueUserCreated {
					return errors.New("rabbitmq connection down")
				}
				return nil
			},
		}
		store := &fakeOutbox{
			events: []identitysqlc.OutboxEvent{
				{EventID: 101, AggregateType: "user", EventType: events.QueueUserCreated, Payload: []byte(`{"user_id":101}`)},
				{EventID: 102, AggregateType: "user", EventType: events.QueueUserUpdated, Payload: []byte(`{"user_id":102}`)},
			},
		}

		w := identityworker.NewOutboxPublisherWorker(store, rabbit, time.Second, 10, &logger)
		count, err := w.ProcessBatch(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		assert.Equal(t, []int64{101}, store.failedIDs)
		assert.Equal(t, []int64{102}, store.markedIDs)
	})
}
