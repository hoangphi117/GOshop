package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"goshop/internal/testutils"
	"goshop/internal/worker"
	"goshop/pkg/rabbitmq"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCouponOrderCancelledConsumer_Handle(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("Returns nil when couponPort is nil", func(t *testing.T) {
		consumer := worker.NewCouponOrderCancelledConsumer(nil, &logger)
		couponID := int32(5)
		evt := rabbitmq.OrderCancelledEvent{
			OrderUUID: uuid.New().String(),
			CouponID:  &couponID,
		}
		body, _ := json.Marshal(evt)
		err := consumer.Handle(amqp.Table{}, body)
		assert.NoError(t, err)
	})

	t.Run("Returns nil when no couponID and no orderUUID", func(t *testing.T) {
		mockCoupon := &testutils.MockOrderCouponPort{}
		consumer := worker.NewCouponOrderCancelledConsumer(mockCoupon, &logger)
		evt := rabbitmq.OrderCancelledEvent{
			OrderID: 12,
		}
		body, _ := json.Marshal(evt)
		err := consumer.Handle(amqp.Table{}, body)
		assert.NoError(t, err)
	})

	t.Run("Returns error on invalid JSON payload", func(t *testing.T) {
		mockCoupon := &testutils.MockOrderCouponPort{}
		consumer := worker.NewCouponOrderCancelledConsumer(mockCoupon, &logger)
		err := consumer.Handle(amqp.Table{}, []byte("invalid-json"))
		assert.Error(t, err)
	})

	t.Run("Successfully releases coupon for cancelled order", func(t *testing.T) {
		orderUUID := uuid.New()
		couponID := int32(42)
		orderID := int32(101)

		var capturedCouponID int32
		var capturedOrderID int32
		var capturedOrderUUID *uuid.UUID

		mockCoupon := &testutils.MockOrderCouponPort{
			ReleaseCouponFunc: func(ctx context.Context, cID int32, oID int32, oUUID *uuid.UUID) error {
				capturedCouponID = cID
				capturedOrderID = oID
				capturedOrderUUID = oUUID
				return nil
			},
		}

		consumer := worker.NewCouponOrderCancelledConsumer(mockCoupon, &logger)
		evt := rabbitmq.OrderCancelledEvent{
			EventID:     "evt-456",
			OrderID:     orderID,
			OrderUUID:   orderUUID.String(),
			OrderNumber: "ORD-888",
			CouponID:    &couponID,
		}
		body, err := json.Marshal(evt)
		require.NoError(t, err)

		err = consumer.Handle(amqp.Table{}, body)
		require.NoError(t, err)

		assert.Equal(t, couponID, capturedCouponID)
		assert.Equal(t, orderID, capturedOrderID)
		require.NotNil(t, capturedOrderUUID)
		assert.Equal(t, orderUUID, *capturedOrderUUID)
	})

	t.Run("Returns error when couponPort fails", func(t *testing.T) {
		couponID := int32(10)
		mockCoupon := &testutils.MockOrderCouponPort{
			ReleaseCouponFunc: func(ctx context.Context, cID int32, oID int32, oUUID *uuid.UUID) error {
				return errors.New("coupon service rpc failure")
			},
		}

		consumer := worker.NewCouponOrderCancelledConsumer(mockCoupon, &logger)
		evt := rabbitmq.OrderCancelledEvent{
			OrderUUID: uuid.New().String(),
			CouponID:  &couponID,
		}
		body, _ := json.Marshal(evt)
		err := consumer.Handle(amqp.Table{}, body)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "coupon service rpc failure")
	})
}
