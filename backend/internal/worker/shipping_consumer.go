package worker

import (
	"context"
	"encoding/json"
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
)

type ShippingConsumer struct {
	shippingService v1service.OrderShippingService
	logger          *zerolog.Logger
}

func NewShippingConsumer(shippingService v1service.OrderShippingService, logger *zerolog.Logger) *ShippingConsumer {
	return &ShippingConsumer{
		shippingService: shippingService,
		logger:          logger,
	}
}

func (sc *ShippingConsumer) Handle(headers amqp.Table, body []byte) error {
	var msg v1dto.CreateShippingQueueMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		if sc.logger != nil {
			sc.logger.Error().Err(err).Msg("Failed to unmarshal shipping creation message")
		}
		return err
	}

	if sc.logger != nil {
		sc.logger.Info().
			Str("order_uuid", msg.OrderUUID).
			Str("provider", msg.Input.Provider).
			Msg("Processing shipping order create event from queue")
	}

	return sc.shippingService.ProcessShippingOrder(context.Background(), msg)
}
