package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	v1dto "goshop/internal/dto/v1"
	"goshop/internal/testutils"
	"goshop/internal/worker"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShippingConsumer_Handle(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("InvalidJSON_ReturnsError", func(t *testing.T) {
		mockService := &testutils.MockOrderShippingService{}
		consumer := worker.NewShippingConsumer(mockService, &logger)

		err := consumer.Handle(amqp.Table{}, []byte("invalid-json"))
		require.Error(t, err)
	})

	t.Run("ValidMessage_DelegatesToProcessShippingOrder", func(t *testing.T) {
		called := false
		orderUUID := "11111111-1111-1111-1111-111111111111"
		msg := v1dto.CreateShippingQueueMessage{
			OrderUUID:  orderUUID,
			EnqueuedAt: time.Now(),
			Input: v1dto.CreateShippingInput{
				Provider:   "ghn",
				DistrictID: 1442,
				WardCode:   "1A0101",
				Weight:     500,
			},
		}

		mockService := &testutils.MockOrderShippingService{
			ProcessShippingOrderFunc: func(ctx context.Context, m v1dto.CreateShippingQueueMessage) error {
				called = true
				assert.Equal(t, orderUUID, m.OrderUUID)
				assert.Equal(t, "ghn", m.Input.Provider)
				assert.Equal(t, 1442, m.Input.DistrictID)
				return nil
			},
		}

		consumer := worker.NewShippingConsumer(mockService, &logger)
		body, err := json.Marshal(msg)
		require.NoError(t, err)

		err = consumer.Handle(amqp.Table{"x-retry": int32(0)}, body)
		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("ServiceError_PropagatesError", func(t *testing.T) {
		msg := v1dto.CreateShippingQueueMessage{
			OrderUUID: "11111111-1111-1111-1111-111111111111",
		}

		mockService := &testutils.MockOrderShippingService{
			ProcessShippingOrderFunc: func(ctx context.Context, m v1dto.CreateShippingQueueMessage) error {
				return errors.New("upstream provider failed")
			},
		}

		consumer := worker.NewShippingConsumer(mockService, &logger)
		body, err := json.Marshal(msg)
		require.NoError(t, err)

		err = consumer.Handle(amqp.Table{}, body)
		require.Error(t, err)
		assert.Equal(t, "upstream provider failed", err.Error())
	})
}
