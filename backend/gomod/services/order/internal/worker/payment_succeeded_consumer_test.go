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
	v1dto "goshop/services/order/internal/dto"
	orderservice "goshop/services/order/internal/service"
	"goshop/shared/pkg/messaging"
)

func TestPaymentSucceededConsumer_Handle(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("InvalidJSON_ReturnsError", func(t *testing.T) {
		mockService := &MockOrderService{}
		consumer := NewPaymentSucceededConsumer(mockService, &logger)

		err := consumer.Handle(messaging.Header{}, []byte("invalid-json"))
		require.Error(t, err)
	})

	t.Run("InvalidUUID_ReturnsError", func(t *testing.T) {
		mockService := &MockOrderService{}
		consumer := NewPaymentSucceededConsumer(mockService, &logger)

		event := events.PaymentSucceededEvent{
			EventID:   uuid.New().String(),
			OrderUUID: "not-a-valid-uuid",
			Timestamp: time.Now().UTC(),
		}
		body, _ := json.Marshal(event)

		err := consumer.Handle(messaging.Header{}, body)
		require.Error(t, err)
	})

	t.Run("ValidEvent_UpdatesPaymentStatusSuccessfully", func(t *testing.T) {
		called := false
		targetOrderUUID := uuid.New()
		targetUserUUID := uuid.New()
		txnID := "MOMO_TXN_123"

		mockService := &MockOrderService{
			UpdateOrderPaymentStatusFunc: func(ctx context.Context, userUUID, orderUUID uuid.UUID, input v1dto.UpdateOrderPaymentStatusInput) (*v1dto.OrderDTO, error) {
				called = true
				assert.Equal(t, targetOrderUUID, orderUUID)
				assert.Equal(t, targetUserUUID, userUUID)
				assert.Equal(t, orderservice.OrderPaymentStatusPaid, input.PaymentStatus)
				require.NotNil(t, input.TransactionRef)
				assert.Equal(t, txnID, *input.TransactionRef)
				return &v1dto.OrderDTO{}, nil
			},
		}

		consumer := NewPaymentSucceededConsumer(mockService, &logger)
		event := events.NewPaymentSucceededEvent(
			uuid.New().String(),
			uuid.New().String(),
			targetOrderUUID.String(),
			"ORD-SUCC-001",
			targetUserUUID.String(),
			"momo",
			txnID,
			500000,
		)
		body, err := json.Marshal(event)
		require.NoError(t, err)

		err = consumer.Handle(messaging.Header{}, body)
		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("IdempotentSkip_WhenAlreadyPaid", func(t *testing.T) {
		targetOrderUUID := uuid.New()
		targetUserUUID := uuid.New()

		mockService := &MockOrderService{
			UpdateOrderPaymentStatusFunc: func(ctx context.Context, userUUID, orderUUID uuid.UUID, input v1dto.UpdateOrderPaymentStatusInput) (*v1dto.OrderDTO, error) {
				return nil, errors.New("Only Unpaid orders can be marked as Paid")
			},
		}

		consumer := NewPaymentSucceededConsumer(mockService, &logger)
		event := events.NewPaymentSucceededEvent(
			uuid.New().String(),
			uuid.New().String(),
			targetOrderUUID.String(),
			"ORD-SUCC-002",
			targetUserUUID.String(),
			"momo",
			"MOMO_DUP",
			500000,
		)
		body, err := json.Marshal(event)
		require.NoError(t, err)

		err = consumer.Handle(messaging.Header{}, body)
		require.NoError(t, err, "Should acknowledge and return nil on idempotent duplicate event")
	})

	t.Run("OrderServiceFails_PropagatesError", func(t *testing.T) {
		targetOrderUUID := uuid.New()
		mockService := &MockOrderService{
			UpdateOrderPaymentStatusFunc: func(ctx context.Context, userUUID, orderUUID uuid.UUID, input v1dto.UpdateOrderPaymentStatusInput) (*v1dto.OrderDTO, error) {
				return nil, errors.New("db connection failure")
			},
		}

		consumer := NewPaymentSucceededConsumer(mockService, &logger)
		event := events.NewPaymentSucceededEvent(
			uuid.New().String(),
			uuid.New().String(),
			targetOrderUUID.String(),
			"ORD-SUCC-003",
			uuid.New().String(),
			"momo",
			"MOMO_FAIL",
			500000,
		)
		body, err := json.Marshal(event)
		require.NoError(t, err)

		err = consumer.Handle(messaging.Header{}, body)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db connection failure")
	})
}
