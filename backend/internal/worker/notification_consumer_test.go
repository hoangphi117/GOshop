package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	v1dto "goshop/internal/dto/v1"
	"goshop/internal/testutils"
	"goshop/internal/worker"
	"goshop/pkg/rabbitmq"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationConsumer_Handle(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("Case 1: Admin Fanout Event (UserID == nil) -> Calls NotifyAdmins", func(t *testing.T) {
		adminCalled := false
		mockService := &testutils.MockNotificationService{
			NotifyAdminsFunc: func(ctx context.Context, input v1dto.AdminNotifyInput) error {
				adminCalled = true
				assert.Equal(t, "order", input.Type)
				assert.Equal(t, "New Order Received", input.Title)
				assert.Equal(t, "Order #ORD-123 placed", input.Body)
				assert.Equal(t, "ORD-123", input.Data["order_number"])
				return nil
			},
		}

		consumer := worker.NewNotificationConsumer(mockService, &logger)
		event := rabbitmq.NewAdminNotificationEvent(
			"evt-admin-1",
			"order",
			"created",
			"New Order Received",
			"Order #ORD-123 placed",
			map[string]any{"order_number": "ORD-123"},
		)

		body, err := json.Marshal(event)
		require.NoError(t, err)

		err = consumer.Handle(amqp.Table{"x-message-id": "evt-admin-1"}, body)
		assert.NoError(t, err)
		assert.True(t, adminCalled)
	})

	t.Run("Case 2: Customer Unicast Event (UserID != nil) -> Calls Send", func(t *testing.T) {
		customerCalled := false
		mockService := &testutils.MockNotificationService{
			SendFunc: func(ctx context.Context, input v1dto.SendNotificationInput) error {
				customerCalled = true
				assert.Equal(t, int32(42), input.UserID)
				assert.Equal(t, "order_created", input.Type)
				assert.Equal(t, "Order Placed Successfully", input.Title)
				return nil
			},
		}

		consumer := worker.NewNotificationConsumer(mockService, &logger)
		event := rabbitmq.NewCustomerNotificationEvent(
			"evt-cust-1",
			42,
			"order_created",
			"send",
			"Order Placed Successfully",
			"Your order #ORD-123 has been placed.",
			map[string]any{"order_id": 123},
		)

		body, err := json.Marshal(event)
		require.NoError(t, err)

		err = consumer.Handle(amqp.Table{"x-message-id": "evt-cust-1"}, body)
		assert.NoError(t, err)
		assert.True(t, customerCalled)
	})

	t.Run("Case 3: Invalid JSON -> Returns Error", func(t *testing.T) {
		consumer := worker.NewNotificationConsumer(&testutils.MockNotificationService{}, &logger)
		err := consumer.Handle(amqp.Table{}, []byte("invalid-json"))
		assert.Error(t, err)
	})

	t.Run("Case 4: Customer Send Failure -> Returns Error", func(t *testing.T) {
		mockService := &testutils.MockNotificationService{
			SendFunc: func(ctx context.Context, input v1dto.SendNotificationInput) error {
				return errors.New("db write error")
			},
		}

		consumer := worker.NewNotificationConsumer(mockService, &logger)
		event := rabbitmq.NewCustomerNotificationEvent(
			"evt-cust-err",
			99,
			"order",
			"send",
			"Order Error",
			"Order failed",
			nil,
		)
		body, _ := json.Marshal(event)

		err := consumer.Handle(amqp.Table{}, body)
		assert.Error(t, err)
	})
}
