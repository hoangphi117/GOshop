package worker

import (
	"context"
	"encoding/json"
	"fmt"
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/pkg/rabbitmq"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
)

type OrderConsumer struct {
	cartService         v1service.CartService
	notificationService v1service.NotificationService
	logger              *zerolog.Logger
}

func NewOrderConsumer(cartService v1service.CartService, notificationService v1service.NotificationService, logger *zerolog.Logger) *OrderConsumer {
	return &OrderConsumer{
		cartService:         cartService,
		notificationService: notificationService,
		logger:              logger,
	}
}

func (oc *OrderConsumer) Handle(headers amqp.Table, body []byte) error {
	var event rabbitmq.OrderCreatedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		if oc.logger != nil {
			oc.logger.Error().Err(err).Msg("Failed to unmarshal order created event")
		}
		return err
	}

	if oc.logger != nil {
		oc.logger.Info().
			Str("event_id", event.EventID).
			Str("order_uuid", event.OrderUUID).
			Str("order_number", event.OrderNumber).
			Int32("user_id", event.UserID).
			Msg("Processing order created event")
	}

	ctx := context.Background()

	// 1. Asynchronously remove purchased items from customer's cart
	if oc.cartService != nil && len(event.PurchasedItems) > 0 {
		if err := oc.cartService.RemovePurchasedItems(ctx, event.UserID, event.PurchasedItems); err != nil {
			if oc.logger != nil {
				oc.logger.Warn().
					Err(err).
					Int32("user_id", event.UserID).
					Str("order_uuid", event.OrderUUID).
					Msg("Failed to clean up cart for order")
			}
		}
	}

	// 2. Dispatch Customer In-App / Real-time Notification
	if oc.notificationService != nil {
		customerNotifyErr := oc.notificationService.Send(ctx, v1dto.SendNotificationInput{
			UserID: event.UserID,
			Type:   "order_created",
			Title:  "Order Placed Successfully",
			Body:   fmt.Sprintf("Your order #%s has been placed successfully.", event.OrderNumber),
			Data:   map[string]any{"order_uuid": event.OrderUUID},
		})
		if customerNotifyErr != nil && oc.logger != nil {
			oc.logger.Warn().
				Err(customerNotifyErr).
				Int32("user_id", event.UserID).
				Str("order_number", event.OrderNumber).
				Msg("Failed to send customer order notification")
		}

		// 3. Dispatch Admin Notification
		adminNotifyErr := oc.notificationService.NotifyAdmins(ctx, v1dto.AdminNotifyInput{
			Type:  "order",
			Title: "New Order Received",
			Body:  fmt.Sprintf("Order #%s placed for $%.2f", event.OrderNumber, event.TotalAmount),
			Data: map[string]any{
				"order_uuid":   event.OrderUUID,
				"order_number": event.OrderNumber,
				"link":         fmt.Sprintf("/orders/%s", event.OrderUUID),
			},
		})
		if adminNotifyErr != nil && oc.logger != nil {
			oc.logger.Warn().
				Err(adminNotifyErr).
				Str("order_uuid", event.OrderUUID).
				Msg("Failed to send admin order notification")
		}
	}

	return nil
}
