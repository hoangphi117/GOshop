package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	v1service "goshop/internal/service/v1"
	"goshop/internal/testutils"
	"goshop/internal/worker"
	"goshop/pkg/rabbitmq"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInventoryOrderCancelledConsumer_Handle(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("Returns nil when stockPort is nil", func(t *testing.T) {
		consumer := worker.NewInventoryOrderCancelledConsumer(nil, &logger)
		evt := rabbitmq.OrderCancelledEvent{
			OrderUUID: uuid.New().String(),
			Items: []rabbitmq.OrderCancelledItem{
				{ProductID: 1, Quantity: 2},
			},
		}
		body, _ := json.Marshal(evt)
		err := consumer.Handle(amqp.Table{}, body)
		assert.NoError(t, err)
	})

	t.Run("Returns nil when items list is empty", func(t *testing.T) {
		mockStock := &testutils.MockOrderStockPort{}
		consumer := worker.NewInventoryOrderCancelledConsumer(mockStock, &logger)
		evt := rabbitmq.OrderCancelledEvent{
			OrderUUID: uuid.New().String(),
			Items:     nil,
		}
		body, _ := json.Marshal(evt)
		err := consumer.Handle(amqp.Table{}, body)
		assert.NoError(t, err)
	})

	t.Run("Returns error on invalid JSON payload", func(t *testing.T) {
		mockStock := &testutils.MockOrderStockPort{}
		consumer := worker.NewInventoryOrderCancelledConsumer(mockStock, &logger)
		err := consumer.Handle(amqp.Table{}, []byte("invalid-json"))
		assert.Error(t, err)
	})

	t.Run("Successfully restores stock for cancelled order", func(t *testing.T) {
		orderUUID := uuid.New().String()
		productUUID := uuid.New().String()

		var capturedItems []v1service.RestoreItem
		var capturedRefID string
		var capturedRefType string
		var capturedReason *string

		mockStock := &testutils.MockOrderStockPort{
			RestoreStockWithRefFunc: func(ctx context.Context, items []v1service.RestoreItem, refID string, refType string, actorID *int32, note *string) error {
				capturedItems = items
				capturedRefID = refID
				capturedRefType = refType
				capturedReason = note
				return nil
			},
		}

		consumer := worker.NewInventoryOrderCancelledConsumer(mockStock, &logger)
		evt := rabbitmq.OrderCancelledEvent{
			EventID:     "evt-123",
			OrderUUID:   orderUUID,
			OrderNumber: "ORD-999",
			Reason:      "Customer requested cancellation",
			Items: []rabbitmq.OrderCancelledItem{
				{ProductID: 10, ProductUUID: productUUID, Quantity: 3},
				{ProductID: 20, Quantity: 1},
			},
		}
		body, err := json.Marshal(evt)
		require.NoError(t, err)

		err = consumer.Handle(amqp.Table{}, body)
		require.NoError(t, err)

		assert.Equal(t, orderUUID, capturedRefID)
		assert.Equal(t, "order_cancel", capturedRefType)
		require.NotNil(t, capturedReason)
		assert.Equal(t, "Customer requested cancellation", *capturedReason)
		require.Len(t, capturedItems, 2)
		assert.Equal(t, int32(10), capturedItems[0].ProductID)
		assert.Equal(t, int32(3), capturedItems[0].Quantity)
		assert.Equal(t, int32(20), capturedItems[1].ProductID)
		assert.Equal(t, int32(1), capturedItems[1].Quantity)
	})

	t.Run("Returns error when stockPort fails", func(t *testing.T) {
		mockStock := &testutils.MockOrderStockPort{
			RestoreStockWithRefFunc: func(ctx context.Context, items []v1service.RestoreItem, refID string, refType string, actorID *int32, note *string) error {
				return errors.New("db connection failure")
			},
		}

		consumer := worker.NewInventoryOrderCancelledConsumer(mockStock, &logger)
		evt := rabbitmq.OrderCancelledEvent{
			OrderUUID: uuid.New().String(),
			Items: []rabbitmq.OrderCancelledItem{
				{ProductID: 1, Quantity: 1},
			},
		}
		body, _ := json.Marshal(evt)
		err := consumer.Handle(amqp.Table{}, body)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "db connection failure")
	})
}
