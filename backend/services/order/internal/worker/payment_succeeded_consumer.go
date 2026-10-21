package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	v1dto "goshop/services/order/internal/dto"
	orderservice "goshop/services/order/internal/service"
	"goshop/pkg/rabbitmq"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
)

// PaymentSucceededConsumer consumes payment.succeeded events and triggers order payment status updates in the Order domain.
type PaymentSucceededConsumer struct {
	orderService orderservice.OrderService
	logger       *zerolog.Logger
}

// NewPaymentSucceededConsumer creates a new PaymentSucceededConsumer instance.
func NewPaymentSucceededConsumer(orderService orderservice.OrderService, logger *zerolog.Logger) *PaymentSucceededConsumer {
	return &PaymentSucceededConsumer{
		orderService: orderService,
		logger:       logger,
	}
}

// Handle processes a payment.succeeded event idempotently.
func (c *PaymentSucceededConsumer) Handle(headers amqp.Table, body []byte) error {
	var event rabbitmq.PaymentSucceededEvent
	if err := json.Unmarshal(body, &event); err != nil {
		var rawStr string
		if unquoteErr := json.Unmarshal(body, &rawStr); unquoteErr == nil {
			if retryErr := json.Unmarshal([]byte(rawStr), &event); retryErr != nil {
				if c.logger != nil {
					c.logger.Error().Err(err).Msg("Failed to unmarshal payment succeeded event")
				}
				return err
			}
		} else {
			if c.logger != nil {
				c.logger.Error().Err(err).Msg("Failed to unmarshal payment succeeded event")
			}
			return err
		}
	}

	orderUUID, err := uuid.Parse(event.OrderUUID)
	if err != nil {
		if c.logger != nil {
			c.logger.Error().Err(err).Str("order_uuid", event.OrderUUID).Msg("Invalid order UUID in payment succeeded event")
		}
		return err
	}

	userUUID, _ := uuid.Parse(event.UserUUID)

	note := fmt.Sprintf("Payment confirmed via %s", event.Provider)
	if event.ProviderTxnID != "" {
		note = fmt.Sprintf("Payment confirmed via %s (txn: %s)", event.Provider, event.ProviderTxnID)
	}

	input := v1dto.UpdateOrderPaymentStatusInput{
		PaymentStatus:  orderservice.OrderPaymentStatusPaid,
		TransactionRef: &event.ProviderTxnID,
		Note:           &note,
	}

	_, err = c.orderService.UpdateOrderPaymentStatus(context.Background(), userUUID, orderUUID, input)
	if err != nil {
		errMsg := err.Error()
		// Idempotency: If the order was already marked as Paid, acknowledge safely without returning an error.
		if strings.Contains(errMsg, "Only Unpaid orders can be marked as Paid") ||
			strings.Contains(errMsg, "Order has already been confirmed as paid") ||
			strings.Contains(errMsg, "already been paid") {
			if c.logger != nil {
				c.logger.Info().
					Str("order_uuid", event.OrderUUID).
					Str("order_number", event.OrderNumber).
					Msg("Order was already marked as Paid, skipping idempotently")
			}
			return nil
		}

		if c.logger != nil {
			c.logger.Error().
				Err(err).
				Str("order_uuid", event.OrderUUID).
				Str("order_number", event.OrderNumber).
				Msg("Failed to update order payment status to Paid from PaymentSucceeded event")
		}
		return err
	}

	if c.logger != nil {
		c.logger.Info().
			Str("order_uuid", event.OrderUUID).
			Str("order_number", event.OrderNumber).
			Str("provider", event.Provider).
			Msg("Order payment status successfully updated to Paid via PaymentSucceeded event")
	}

	return nil
}
