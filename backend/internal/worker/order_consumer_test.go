package worker_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	v1dto "goshop/internal/dto/v1"
	"goshop/internal/testutils"
	"goshop/internal/worker"
	"goshop/pkg/rabbitmq"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderConsumer_Handle(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("InvalidJSON_ReturnsError", func(t *testing.T) {
		consumer := worker.NewOrderConsumer(nil, nil, &logger)
		err := consumer.Handle(amqp.Table{}, []byte("invalid-json"))
		require.Error(t, err)
	})

	t.Run("ValidEvent_ExecutesCartCleanupAndNotifications", func(t *testing.T) {
		var cartCleanedUserID int32
		var cartCleanedItems []int32
		var customerNotified bool
		var adminNotified bool

		mockCartService := &testutils.MockCartService{
			RemovePurchasedItemsFunc: func(ctx context.Context, userID int32, productIDs []int32) error {
				cartCleanedUserID = userID
				cartCleanedItems = productIDs
				return nil
			},
		}

		mockNotifService := &testutils.MockNotificationService{
			SendFunc: func(ctx context.Context, input v1dto.SendNotificationInput) error {
				customerNotified = true
				assert.Equal(t, int32(42), input.UserID)
				assert.Equal(t, "order_created", input.Type)
				return nil
			},
			NotifyAdminsFunc: func(ctx context.Context, input v1dto.AdminNotifyInput) error {
				adminNotified = true
				assert.Equal(t, "order", input.Type)
				return nil
			},
		}

		consumer := worker.NewOrderConsumer(mockCartService, mockNotifService, &logger)

		event := rabbitmq.OrderCreatedEvent{
			EventID:        "evt-123",
			OrderID:        100,
			OrderUUID:      "11111111-1111-1111-1111-111111111111",
			OrderNumber:    "ORD-2026-001",
			UserID:         42,
			UserUUID:       "22222222-2222-2222-2222-222222222222",
			TotalAmount:    150.0,
			PurchasedItems: []int32{1, 2, 3},
			Timestamp:      time.Now().UTC(),
		}

		body, err := json.Marshal(event)
		require.NoError(t, err)

		err = consumer.Handle(amqp.Table{}, body)
		require.NoError(t, err)

		assert.Equal(t, int32(42), cartCleanedUserID)
		assert.Equal(t, []int32{1, 2, 3}, cartCleanedItems)
		assert.True(t, customerNotified)
		assert.True(t, adminNotified)
	})
}
