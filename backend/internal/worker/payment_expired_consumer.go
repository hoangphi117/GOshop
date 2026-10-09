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

// PaymentExpiredConsumer consumes payment.expired events and triggers order cancellation in the Order domain.
type PaymentExpiredConsumer struct {
	orderService v1service.OrderService
	logger       *zerolog.Logger
}

func NewPaymentExpiredConsumer(orderService v1service.OrderService, logger *zerolog.Logger) *PaymentExpiredConsumer {
	return &PaymentExpiredConsumer{
		orderService: orderService,
		logger:       logger,
	}
}

func (c *PaymentExpiredConsumer) Handle(headers amqp.Table, body []byte) error {
	var event rabbitmq.PaymentExpiredEvent
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
