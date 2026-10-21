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

func TestInventoryReturnRestockConsumer_Handle(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("Returns nil when stockPort is nil", func(t *testing.T) {
		consumer := worker.NewInventoryReturnRestockConsumer(nil, &logger)
		evt := rabbitmq.OrderReturnCompletedEvent{
			ReturnUUID:  uuid.New().String(),
			OrderUUID:   uuid.New().String(),
			OrderNumber: "ORD-111",
			Items: []rabbitmq.ReturnRestockItem{
				{ProductID: 1, Quantity: 2, Restock: true},
			},
		}
		body, _ := json.Marshal(evt)
		err := consumer.Handle(amqp.Table{}, body)
		assert.NoError(t, err)
	})

	t.Run("Returns nil when items list is empty", func(t *testing.T) {
		mockStock := &testutils.MockOrderStockPort{}
		consumer := worker.NewInventoryReturnRestockConsumer(mockStock, &logger)
		evt := rabbitmq.OrderReturnCompletedEvent{
			ReturnUUID: uuid.New().String(),
			Items:      nil,
		}
		body, _ := json.Marshal(evt)
		err := consumer.Handle(amqp.Table{}, body)
		assert.NoError(t, err)
	})

	t.Run("Returns nil when items have Restock=false or Quantity<=0", func(t *testing.T) {
		called := false
		mockStock := &testutils.MockOrderStockPort{
			RestoreStockWithRefFunc: func(ctx context.Context, items []v1service.RestoreItem, refID string, refType string, actorID *int32, note *string) error {
				called = true
				return nil
			},
		}
		consumer := worker.NewInventoryReturnRestockConsumer(mockStock, &logger)
		evt := rabbitmq.OrderReturnCompletedEvent{
			ReturnUUID:  uuid.New().String(),
			OrderNumber: "ORD-222",
			Items: []rabbitmq.ReturnRestockItem{
				{ProductID: 1, Quantity: 2, Restock: false}, // damaged, do not restock
				{ProductID: 2, Quantity: 0, Restock: true},  // zero quantity
			},
		}
		body, _ := json.Marshal(evt)
		err := consumer.Handle(amqp.Table{}, body)
		assert.NoError(t, err)
		assert.False(t, called, "stockPort should not be called when no restockable items")
	})

	t.Run("Returns error on invalid JSON payload", func(t *testing.T) {
		mockStock := &testutils.MockOrderStockPort{}
		consumer := worker.NewInventoryReturnRestockConsumer(mockStock, &logger)
		err := consumer.Handle(amqp.Table{}, []byte("invalid-json"))
		assert.Error(t, err)
	})

	t.Run("Successfully restores only restockable items", func(t *testing.T) {
		returnUUID := uuid.New().String()
		orderUUID := uuid.New().String()

		var capturedItems []v1service.RestoreItem
		var capturedRefID string
		var capturedRefType string
		var capturedNote *string

		mockStock := &testutils.MockOrderStockPort{
			RestoreStockWithRefFunc: func(ctx context.Context, items []v1service.RestoreItem, refID string, refType string, actorID *int32, note *string) error {
				capturedItems = items
				capturedRefID = refID
				capturedRefType = refType
				capturedNote = note
				return nil
			},
		}

		consumer := worker.NewInventoryReturnRestockConsumer(mockStock, &logger)
		evt := rabbitmq.OrderReturnCompletedEvent{
			EventID:     "evt-ret-1",
			ReturnUUID:  returnUUID,
			OrderUUID:   orderUUID,
			OrderNumber: "ORD-333",
			Items: []rabbitmq.ReturnRestockItem{
				{ProductID: 10, Quantity: 2, Restock: true},
				{ProductID: 20, Quantity: 1, Restock: false}, // item damaged, skip
				{ProductID: 30, Quantity: 5, Restock: true},
			},
		}
		body, err := json.Marshal(evt)
		require.NoError(t, err)

		err = consumer.Handle(amqp.Table{}, body)
		require.NoError(t, err)

		assert.Equal(t, returnUUID, capturedRefID)
		assert.Equal(t, "return", capturedRefType)
		require.NotNil(t, capturedNote)
		assert.Contains(t, *capturedNote, "Restock from completed return")
		require.Len(t, capturedItems, 2)
		assert.Equal(t, int32(10), capturedItems[0].ProductID)
		assert.Equal(t, evt.Items[0].ProductUUID, capturedItems[0].ProductUUID)
		assert.Equal(t, int32(2), capturedItems[0].Quantity)
		assert.Equal(t, int32(30), capturedItems[1].ProductID)
		assert.Equal(t, int32(5), capturedItems[1].Quantity)
	})

	t.Run("Returns error when stockPort fails", func(t *testing.T) {
		mockStock := &testutils.MockOrderStockPort{
			RestoreStockWithRefFunc: func(ctx context.Context, items []v1service.RestoreItem, refID string, refType string, actorID *int32, note *string) error {
				return errors.New("inventory deadlocked")
			},
		}

		consumer := worker.NewInventoryReturnRestockConsumer(mockStock, &logger)
		evt := rabbitmq.OrderReturnCompletedEvent{
			ReturnUUID:  uuid.New().String(),
			OrderNumber: "ORD-444",
			Items: []rabbitmq.ReturnRestockItem{
				{ProductID: 1, Quantity: 1, Restock: true},
			},
		}
		body, _ := json.Marshal(evt)
		err := consumer.Handle(amqp.Table{}, body)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "inventory deadlocked")
	})
}
