package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	v1dto "goshop/internal/dto/v1"
	"goshop/internal/testutils"
	"goshop/internal/worker"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentConsumer_Handle(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("InvalidJSON_ReturnsError", func(t *testing.T) {
		mockService := &testutils.MockPaymentService{}
		consumer := worker.NewPaymentConsumer(mockService, &logger)

		err := consumer.Handle(amqp.Table{}, []byte("invalid-json"))
		require.Error(t, err)
	})

	t.Run("ValidMessage_DelegatesToProcessWebhook", func(t *testing.T) {
		called := false
		msg := v1dto.PaymentWebhookMessage{
			WebhookEventID: 101,
			OrderUUID:      "11111111-1111-1111-1111-111111111111",
			Provider:       "momo",
			EventType:      "payment.success",
			ProviderTxnID:  "txn-12345",
			OrderNumber:    "ORD-2026-001",
			Amount:         150000,
			Status:         "SUCCESS",
			Metadata:       map[string]string{"partnerCode": "MOMO"},
		}

		mockService := &testutils.MockPaymentService{
			ProcessWebhookFunc: func(ctx context.Context, m v1dto.PaymentWebhookMessage) error {
				called = true
				assert.Equal(t, "momo", m.Provider)
				assert.Equal(t, int32(101), m.WebhookEventID)
				assert.Equal(t, "ORD-2026-001", m.OrderNumber)
				assert.Equal(t, "MOMO", m.Metadata["partnerCode"])
				return nil
			},
		}

		consumer := worker.NewPaymentConsumer(mockService, &logger)
		body, err := json.Marshal(msg)
		require.NoError(t, err)

		err = consumer.Handle(amqp.Table{}, body)
		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("ServiceError_PropagatesError", func(t *testing.T) {
		msg := v1dto.PaymentWebhookMessage{
			Provider: "stripe",
		}

		mockService := &testutils.MockPaymentService{
			ProcessWebhookFunc: func(ctx context.Context, m v1dto.PaymentWebhookMessage) error {
				return errors.New("webhook processing failed")
			},
		}

		consumer := worker.NewPaymentConsumer(mockService, &logger)
		body, err := json.Marshal(msg)
		require.NoError(t, err)

		err = consumer.Handle(amqp.Table{}, body)
		require.Error(t, err)
		assert.Equal(t, "webhook processing failed", err.Error())
	})
}
