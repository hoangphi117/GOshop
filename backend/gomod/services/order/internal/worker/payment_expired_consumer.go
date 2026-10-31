package worker

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"goshop/contracts/events"
	orderservice "goshop/services/order/internal/service"
	"goshop/shared/pkg/messaging"
)

// PaymentExpiredConsumer consumes payment.expired events and triggers order cancellation in the Order domain.
type PaymentExpiredConsumer struct {
	orderService orderservice.OrderService
	logger       *zerolog.Logger
}

func NewPaymentExpiredConsumer(orderService orderservice.OrderService, logger *zerolog.Logger) *PaymentExpiredConsumer {
	return &PaymentExpiredConsumer{
		orderService: orderService,
		logger:       logger,
	}
}

func (c *PaymentExpiredConsumer) Handle(headers messaging.Header, body []byte) error {
	var event events.PaymentExpiredEvent
	if err := json.Unmarshal(body, &event); err != nil {
		if c.logger != nil {
			c.logger.Error().Err(err).Msg("Failed to unmarshal payment expired event")
		}
		return err
	}

	orderUUID, err := uuid.Parse(event.OrderUUID)
	if err != nil {
		if c.logger != nil {
			c.logger.Error().Err(err).Str("order_uuid", event.OrderUUID).Msg("Invalid order UUID in payment expired event")
		}
		return err
	}

	if err := c.orderService.CancelExpiredOrder(context.Background(), orderUUID); err != nil {
		if c.logger != nil {
			c.logger.Error().Err(err).Str("order_uuid", event.OrderUUID).Msg("Failed to cancel expired order")
		}
		return err
	}

	if c.logger != nil {
		c.logger.Info().
			Str("order_uuid", event.OrderUUID).
			Str("order_number", event.OrderNumber).
			Msg("Expired order cancelled and inventory restored successfully via PaymentExpired event")
	}

	return nil
}
