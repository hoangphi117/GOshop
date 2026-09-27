package worker

import (
	"context"
	"encoding/json"
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
)

type PaymentConsumer struct {
	paymentService v1service.PaymentService
	logger         *zerolog.Logger
}

func NewPaymentConsumer(paymentService v1service.PaymentService, logger *zerolog.Logger) *PaymentConsumer {
	return &PaymentConsumer{
		paymentService: paymentService,
		logger:         logger,
	}
}

func (pc *PaymentConsumer) Handle(headers amqp.Table, body []byte) error {
	var msg v1dto.PaymentWebhookMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		if pc.logger != nil {
			pc.logger.Error().Err(err).Msg("Failed to unmarshal payment webhook message")
		}
		return err
	}

	return pc.paymentService.ProcessWebhook(context.Background(), msg)
}
