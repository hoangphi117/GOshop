package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goshop/contracts/events"
	"goshop/pkg/messaging"
)

func TestPaymentExpiredConsumer_Handle(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("InvalidJSON_ReturnsError", func(t *testing.T) {
		mockService := &MockOrderService{}
		consumer := NewPaymentExpiredConsumer(mockService, &logger)

		err := consumer.Handle(messaging.Header{}, []byte("invalid-json"))
		require.Error(t, err)
	})

	t.Run("InvalidUUID_ReturnsError", func(t *testing.T) {
		mockService := &MockOrderService{}
		consumer := NewPaymentExpiredConsumer(mockService, &logger)

		event := events.PaymentExpiredEvent{
			EventID:   uuid.New().String(),
			OrderUUID: "not-a-valid-uuid",
			Timestamp: time.Now().UTC(),
		}
		body, _ := json.Marshal(event)

		err := consumer.Handle(messaging.Header{}, body)
		require.Error(t, err)
	})

	t.Run("ValidEvent_CancelsExpiredOrderSuccessfully", func(t *testing.T) {
		called := false
		targetOrderUUID := uuid.New()
		mockService := &MockOrderService{
			CancelExpiredOrderFunc: func(ctx context.Context, orderUUID uuid.UUID) error {
				called = true
				assert.Equal(t, targetOrderUUID, orderUUID)
				return nil
			},
		}

		consumer := NewPaymentExpiredConsumer(mockService, &logger)
		event := events.NewPaymentExpiredEvent(
			uuid.New().String(),
			uuid.New().String(),
			targetOrderUUID.String(),
			"ORD-EXP-001",
			uuid.New().String(),
			"momo",
		)
		body, err := json.Marshal(event)
		require.NoError(t, err)

		err = consumer.Handle(messaging.Header{}, body)
		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("OrderServiceFails_PropagatesError", func(t *testing.T) {
		targetOrderUUID := uuid.New()
		mockService := &MockOrderService{
			CancelExpiredOrderFunc: func(ctx context.Context, orderUUID uuid.UUID) error {
				return errors.New("db connection failure")
			},
		}

		consumer := NewPaymentExpiredConsumer(mockService, &logger)
		event := events.NewPaymentExpiredEvent(
			uuid.New().String(),
			uuid.New().String(),
			targetOrderUUID.String(),
			"ORD-EXP-002",
			uuid.New().String(),
			"momo",
		)
		body, err := json.Marshal(event)
		require.NoError(t, err)

		err = consumer.Handle(messaging.Header{}, body)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db connection failure")
	})
}
