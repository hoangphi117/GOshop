package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goshop/pkg/messaging"
	v1dto "goshop/services/order/internal/dto"
)

func TestShippingConsumer_Handle(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("InvalidJSON_ReturnsError", func(t *testing.T) {
		mockService := &MockOrderShippingService{}
		consumer := NewShippingConsumer(mockService, &logger)

		err := consumer.Handle(messaging.Header{}, []byte("invalid-json"))
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

		mockService := &MockOrderShippingService{
			ProcessShippingOrderFunc: func(ctx context.Context, m v1dto.CreateShippingQueueMessage) error {
				called = true
				assert.Equal(t, orderUUID, m.OrderUUID)
				assert.Equal(t, "ghn", m.Input.Provider)
				assert.Equal(t, 1442, m.Input.DistrictID)
				return nil
			},
		}

		consumer := NewShippingConsumer(mockService, &logger)
		body, err := json.Marshal(msg)
		require.NoError(t, err)

		err = consumer.Handle(messaging.Header{"x-retry": "0"}, body)
		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("ServiceError_PropagatesError", func(t *testing.T) {
		msg := v1dto.CreateShippingQueueMessage{
			OrderUUID: "11111111-1111-1111-1111-111111111111",
		}

		mockService := &MockOrderShippingService{
			ProcessShippingOrderFunc: func(ctx context.Context, m v1dto.CreateShippingQueueMessage) error {
				return errors.New("upstream provider failed")
			},
		}

		consumer := NewShippingConsumer(mockService, &logger)
		body, err := json.Marshal(msg)
		require.NoError(t, err)

		err = consumer.Handle(messaging.Header{}, body)
		require.Error(t, err)
		assert.Equal(t, "upstream provider failed", err.Error())
	})
}
