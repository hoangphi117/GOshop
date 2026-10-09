package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"goshop/internal/testutils"
	"goshop/internal/worker"
	"goshop/pkg/rabbitmq"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentExpiredConsumer_Handle(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("InvalidJSON_ReturnsError", func(t *testing.T) {
		mockService := &testutils.MockOrderService{}
		consumer := worker.NewPaymentExpiredConsumer(mockService, &logger)

		err := consumer.Handle(amqp.Table{}, []byte("invalid-json"))
		require.Error(t, err)
	})

	t.Run("InvalidUUID_ReturnsError", func(t *testing.T) {
		mockService := &testutils.MockOrderService{}
		consumer := worker.NewPaymentExpiredConsumer(mockService, &logger)

		event := rabbitmq.PaymentExpiredEvent{
			EventID:   uuid.New().String(),
			OrderUUID: "not-a-valid-uuid",
			Timestamp: time.Now().UTC(),
		}
		body, _ := json.Marshal(event)

		err := consumer.Handle(amqp.Table{}, body)
		require.Error(t, err)
	})

	t.Run("ValidEvent_CancelsExpiredOrderSuccessfully", func(t *testing.T) {
		called := false
		targetOrderUUID := uuid.New()
		mockService := &testutils.MockOrderService{
			CancelExpiredOrderFunc: func(ctx context.Context, orderUUID uuid.UUID) error {
				called = true
				assert.Equal(t, targetOrderUUID, orderUUID)
				return nil
			},
		}

		consumer := worker.NewPaymentExpiredConsumer(mockService, &logger)
		event := rabbitmq.NewPaymentExpiredEvent(
			uuid.New().String(),
			uuid.New().String(),
			targetOrderUUID.String(),
			"ORD-EXP-001",
			uuid.New().String(),
			"momo",
		)
		body, err := json.Marshal(event)
		require.NoError(t, err)

		err = consumer.Handle(amqp.Table{}, body)
		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("OrderServiceFails_PropagatesError", func(t *testing.T) {
		targetOrderUUID := uuid.New()
		mockService := &testutils.MockOrderService{
			CancelExpiredOrderFunc: func(ctx context.Context, orderUUID uuid.UUID) error {
				return errors.New("db connection failure")
			},
		}

		consumer := worker.NewPaymentExpiredConsumer(mockService, &logger)
		event := rabbitmq.NewPaymentExpiredEvent(
			uuid.New().String(),
			uuid.New().String(),
			targetOrderUUID.String(),
			"ORD-EXP-002",
			uuid.New().String(),
			"momo",
		)
		body, err := json.Marshal(event)
		require.NoError(t, err)

		err = consumer.Handle(amqp.Table{}, body)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db connection failure")
	})
}
