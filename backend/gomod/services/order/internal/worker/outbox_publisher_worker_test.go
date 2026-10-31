package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goshop/contracts/events"
	ordersqlc "goshop/services/order/internal/db/sqlc"
	"goshop/shared/testutils"
)

func TestOutboxPublisherWorker_ProcessBatch(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	t.Run("Returns 0 when dependencies are nil", func(t *testing.T) {
		w1 := NewOutboxPublisherWorker(nil, &testutils.MockRabbitMQService{}, time.Second, 10, &logger)
		count, err := w1.ProcessBatch(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, count)

		w2 := NewOutboxPublisherWorker(&MockOutboxRepository{}, nil, time.Second, 10, &logger)
		count, err = w2.ProcessBatch(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("Returns 0 when no pending events", func(t *testing.T) {
		mockOutbox := &MockOutboxRepository{
			GetPendingByAggregateTypesFunc: func(ctx context.Context, limit int32, types []string) ([]ordersqlc.OutboxEvent, error) {
				assert.ElementsMatch(t, []string{"order", "payment"}, types)
				return []ordersqlc.OutboxEvent{}, nil
			},
		}
		mockRabbitMQ := &testutils.MockRabbitMQService{}

		w := NewOutboxPublisherWorker(mockOutbox, mockRabbitMQ, time.Second, 10, &logger)
		count, err := w.ProcessBatch(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("Returns error when GetPendingByAggregateTypes fails", func(t *testing.T) {
		mockOutbox := &MockOutboxRepository{
			GetPendingByAggregateTypesFunc: func(ctx context.Context, limit int32, types []string) ([]ordersqlc.OutboxEvent, error) {
				return nil, errors.New("db error")
			},
		}
		mockRabbitMQ := &testutils.MockRabbitMQService{}

		w := NewOutboxPublisherWorker(mockOutbox, mockRabbitMQ, time.Second, 10, &logger)
		_, err := w.ProcessBatch(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch pending order/payment outbox events")
	})

	t.Run("Successfully publishes order and payment events and marks them processed", func(t *testing.T) {
		publishedQueues := make([]string, 0)
		publishedPayloads := make([]string, 0)
		markedIDs := make([]int64, 0)

		mockRabbitMQ := &testutils.MockRabbitMQService{
			PublishFunc: func(ctx context.Context, queue string, message any) error {
				publishedQueues = append(publishedQueues, queue)
				bytes, err := json.Marshal(message)
				require.NoError(t, err)
				publishedPayloads = append(publishedPayloads, string(bytes))
				return nil
			},
		}

		mockOutbox := &MockOutboxRepository{
			GetPendingByAggregateTypesFunc: func(ctx context.Context, limit int32, types []string) ([]ordersqlc.OutboxEvent, error) {
				return []ordersqlc.OutboxEvent{
					{
						EventID:       1,
						AggregateType: "order",
						AggregateID:   "ord-uuid-1",
						EventType:     events.QueueOrderCreated,
						Payload:       []byte(`{"order_id":1,"order_number":"ORD-001"}`),
					},
					{
						EventID:       2,
						AggregateType: "payment",
						AggregateID:   "pt-uuid-2",
						EventType:     events.QueuePaymentSucceeded,
						Payload:       []byte(`{"pt_uuid":"pt-uuid-2","order_uuid":"ord-uuid-1"}`),
					},
				}, nil
			},
			MarkProcessedFunc: func(ctx context.Context, ids []int64) error {
				markedIDs = append(markedIDs, ids...)
				return nil
			},
		}

		w := NewOutboxPublisherWorker(mockOutbox, mockRabbitMQ, time.Second, 10, &logger)
		count, err := w.ProcessBatch(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
		assert.Equal(t, []string{events.QueueOrderCreated, events.QueuePaymentSucceeded}, publishedQueues)
		assert.Equal(t, []int64{1, 2}, markedIDs)
		assert.Equal(t, `{"order_id":1,"order_number":"ORD-001"}`, publishedPayloads[0])
		assert.Equal(t, `{"pt_uuid":"pt-uuid-2","order_uuid":"ord-uuid-1"}`, publishedPayloads[1])
	})

	t.Run("Marks failed when RabbitMQ publish returns error", func(t *testing.T) {
		failedEventIDs := make([]int64, 0)
		markedProcessedIDs := make([]int64, 0)

		mockRabbitMQ := &testutils.MockRabbitMQService{
			PublishFunc: func(ctx context.Context, queue string, message any) error {
				if queue == events.QueueOrderCreated {
					return errors.New("rabbitmq connection down")
				}
				return nil
			},
		}

		mockOutbox := &MockOutboxRepository{
			GetPendingByAggregateTypesFunc: func(ctx context.Context, limit int32, types []string) ([]ordersqlc.OutboxEvent, error) {
				return []ordersqlc.OutboxEvent{
					{
						EventID:       101,
						AggregateType: "order",
						AggregateID:   "ord-uuid-101",
						EventType:     events.QueueOrderCreated,
						Payload:       []byte(`{"order_id":101}`),
					},
					{
						EventID:       102,
						AggregateType: "payment",
						AggregateID:   "pt-uuid-102",
						EventType:     events.QueuePaymentSucceeded,
						Payload:       []byte(`{"pt_uuid":"pt-uuid-102"}`),
					},
				}, nil
			},
			MarkFailedFunc: func(ctx context.Context, id int64, lastErr string) error {
				failedEventIDs = append(failedEventIDs, id)
				return nil
			},
			MarkProcessedFunc: func(ctx context.Context, ids []int64) error {
				markedProcessedIDs = append(markedProcessedIDs, ids...)
				return nil
			},
		}

		w := NewOutboxPublisherWorker(mockOutbox, mockRabbitMQ, time.Second, 10, &logger)
		count, err := w.ProcessBatch(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		assert.Equal(t, []int64{101}, failedEventIDs)
		assert.Equal(t, []int64{102}, markedProcessedIDs)
	})
}
