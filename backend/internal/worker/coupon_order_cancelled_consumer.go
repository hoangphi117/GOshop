package worker

import (
	"context"
	"encoding/json"
	v1service "goshop/internal/service/v1"
	"goshop/pkg/rabbitmq"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
)

// CouponOrderCancelledConsumer consumes order.cancelled events on topic queue coupon.order_events
// to idempotently release coupon reservations and decrement coupon usage.
type CouponOrderCancelledConsumer struct {
	couponPort v1service.OrderCouponPort
	logger     *zerolog.Logger
}

func NewCouponOrderCancelledConsumer(
	couponPort v1service.OrderCouponPort,
	logger *zerolog.Logger,
) *CouponOrderCancelledConsumer {
	return &CouponOrderCancelledConsumer{
		couponPort: couponPort,
		logger:     logger,
	}
}

func (c *CouponOrderCancelledConsumer) Handle(headers amqp.Table, body []byte) error {
	var event rabbitmq.OrderCancelledEvent
	if err := json.Unmarshal(body, &event); err != nil {
		if c.logger != nil {
			c.logger.Error().Err(err).Msg("Failed to unmarshal order cancelled event in coupon consumer")
		}
		return err
	}

	if c.couponPort == nil {
		return nil
	}

	var couponID int32
	if event.CouponID != nil {
		couponID = *event.CouponID
	}

	var orderUUID *uuid.UUID
	if event.OrderUUID != "" {
		if u, err := uuid.Parse(event.OrderUUID); err == nil {
			orderUUID = &u
		}
	}

	// Only release coupon if a coupon was used or an orderUUID is present
	if couponID == 0 && orderUUID == nil {
		return nil
	}

	ctx := context.Background()
	if err := c.couponPort.ReleaseCoupon(ctx, couponID, event.OrderID, orderUUID); err != nil {
		if c.logger != nil {
			c.logger.Error().
				Err(err).
				Str("order_uuid", event.OrderUUID).
				Int32("coupon_id", couponID).
				Msg("Failed to release coupon via coupon order cancelled consumer")
		}
		return err
	}

	if c.logger != nil {
		c.logger.Info().
			Str("order_uuid", event.OrderUUID).
			Int32("coupon_id", couponID).
			Msg("Coupon released successfully from topic event order.cancelled")
	}

	return nil
}
