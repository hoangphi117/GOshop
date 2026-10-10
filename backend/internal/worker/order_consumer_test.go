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

	t.Run("OrderCancelled_NotifiesCustomerAndAdmin", func(t *testing.T) {
		var customerNotified, adminNotified bool
		mockNotifService := &testutils.MockNotificationService{
			SendFunc: func(ctx context.Context, input v1dto.SendNotificationInput) error {
				customerNotified = true
				assert.Equal(t, int32(42), input.UserID)
				assert.Equal(t, "order_cancelled", input.Type)
				return nil
			},
			NotifyAdminsFunc: func(ctx context.Context, input v1dto.AdminNotifyInput) error {
				adminNotified = true
				assert.Equal(t, "order", input.Type)
				return nil
			},
		}

		consumer := worker.NewOrderConsumer(nil, mockNotifService, &logger)
		event := rabbitmq.NewOrderCancelledEvent("evt-1", "order-uuid", "ORD-001", "user-uuid", 1, 42, "Customer request")
		body, _ := json.Marshal(event)

		err := consumer.HandleOrderCancelled(amqp.Table{}, body)
		require.NoError(t, err)
		assert.True(t, customerNotified)
		assert.True(t, adminNotified)
	})

	t.Run("OrderStatusChanged_NotifiesCustomer", func(t *testing.T) {
		var customerNotified bool
		mockNotifService := &testutils.MockNotificationService{
			SendFunc: func(ctx context.Context, input v1dto.SendNotificationInput) error {
				customerNotified = true
				assert.Equal(t, int32(42), input.UserID)
				assert.Equal(t, "order_status", input.Type)
				return nil
			},
		}

		consumer := worker.NewOrderConsumer(nil, mockNotifService, &logger)
		event := rabbitmq.NewOrderStatusChangedEvent("evt-2", "order-uuid", "ORD-001", "user-uuid", 1, 42, 2, 3, "GHN123")
		body, _ := json.Marshal(event)

		err := consumer.HandleOrderStatusChanged(amqp.Table{}, body)
		require.NoError(t, err)
		assert.True(t, customerNotified)
	})

	t.Run("OrderReturnEvents_NotifiesCustomerAndAdmin", func(t *testing.T) {
		var reqCustomerNotified, reqAdminNotified bool
		var updatedCustomerNotified bool

		mockNotifService := &testutils.MockNotificationService{
			SendFunc: func(ctx context.Context, input v1dto.SendNotificationInput) error {
				if input.Type == "return_requested" {
					reqCustomerNotified = true
				} else if input.Type == "return_approved" {
					updatedCustomerNotified = true
				}
				return nil
			},
			NotifyAdminsFunc: func(ctx context.Context, input v1dto.AdminNotifyInput) error {
				if input.Type == "return" {
					reqAdminNotified = true
				}
				return nil
			},
		}

		consumer := worker.NewOrderConsumer(nil, mockNotifService, &logger)

		reqEvent := rabbitmq.NewOrderReturnRequestedEvent("evt-3", "ret-uuid", "order-uuid", "ORD-001", 10, 1, 42, "Defective product")
		reqBody, _ := json.Marshal(reqEvent)
		err := consumer.HandleOrderReturnRequested(amqp.Table{}, reqBody)
		require.NoError(t, err)
		assert.True(t, reqCustomerNotified)
		assert.True(t, reqAdminNotified)

		updEvent := rabbitmq.NewOrderReturnUpdatedEvent("evt-4", "ret-uuid", "order-uuid", "ORD-001", 10, 1, 42, 2, "Approved by admin")
		updBody, _ := json.Marshal(updEvent)
		err = consumer.HandleOrderReturnUpdated(amqp.Table{}, updBody)
		require.NoError(t, err)
		assert.True(t, updatedCustomerNotified)
	})
}
