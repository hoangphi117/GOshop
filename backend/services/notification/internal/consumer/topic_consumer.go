package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"goshop/pkg/discord"
	"goshop/pkg/mail"
	"goshop/pkg/rabbitmq"
	"goshop/services/notification/internal/dto"
	"goshop/services/notification/internal/service"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
)

type TopicConsumer struct {
	notificationService service.NotificationService
	mailService         mail.EmailProviderService
	discordNotifier     discord.Notifier
	logger              *zerolog.Logger
}

func NewTopicConsumer(
	notificationService service.NotificationService,
	mailService mail.EmailProviderService,
	discordNotifier discord.Notifier,
	logger *zerolog.Logger,
) *TopicConsumer {

	return &TopicConsumer{
		notificationService: notificationService,
		mailService:         mailService,
		discordNotifier:     discordNotifier,
		logger:              logger,
	}
}

// HandleEvent dispatches events based on the x-routing-key header or payload inspection
func (c *TopicConsumer) HandleEvent(headers amqp.Table, body []byte) error {
	routingKey, _ := headers["x-routing-key"].(string)
	traceID, _ := headers["trace_id"].(string)

	c.logger.Info().
		Str("trace_id", traceID).
		Str("routing_key", routingKey).
		Msg("Processing topic event in Notification Service")

	switch routingKey {
	case rabbitmq.RoutingKeyOrderCreated:
		return c.handleOrderCreated(body)
	case rabbitmq.RoutingKeyOrderStatusChanged, rabbitmq.RoutingKeyOrderShipped, rabbitmq.RoutingKeyOrderDelivered:
		return c.handleOrderStatusChanged(routingKey, body)
	case rabbitmq.RoutingKeyPaymentSucceeded:
		return c.handlePaymentSucceeded(body)
	case rabbitmq.RoutingKeyPaymentFailed:
		return c.handlePaymentFailed(body)
	case rabbitmq.RoutingKeyInventoryStockLow:
		return c.handleInventoryStockLow(body)
	default:
		// Fallback: check if body is generic NotificationEvent
		var notifEvent rabbitmq.NotificationEvent
		if err := json.Unmarshal(body, &notifEvent); err == nil && notifEvent.Type != "" {
			return c.handleGenericNotification(notifEvent)
		}
		c.logger.Debug().Str("routing_key", routingKey).Msg("Ignored unhandled routing key")
		return nil
	}
}

func (c *TopicConsumer) handleOrderCreated(body []byte) error {
	var payload struct {
		OrderID     string  `json:"order_id"`
		OrderNumber string  `json:"order_number"`
		UserUUID    string  `json:"user_uuid"`
		UserID      *int32  `json:"user_id"`
		Customer    string  `json:"customer_name"`
		TotalAmount float64 `json:"total_amount"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return err
	}

	title := "Order Placed Successfully"
	msgBody := fmt.Sprintf("Your order #%s has been placed successfully.", payload.OrderNumber)

	if payload.UserID != nil {
		_ = c.notificationService.Send(context.Background(), dto.SendNotificationInput{
			UserID: *payload.UserID,
			Type:   "order",
			Title:  title,
			Body:   msgBody,
			Data:   map[string]any{"order_number": payload.OrderNumber, "total": payload.TotalAmount},
		})
	}

	// Notify admins
	_ = c.notificationService.NotifyAdmins(context.Background(), dto.AdminNotifyInput{
		Type:  "order",
		Title: "New Order Received",
		Body:  fmt.Sprintf("Order #%s received for $%.2f", payload.OrderNumber, payload.TotalAmount),
		Data:  map[string]any{"order_number": payload.OrderNumber},
	})

	return nil
}

func (c *TopicConsumer) handleOrderStatusChanged(routingKey string, body []byte) error {
	var payload struct {
		OrderNumber string `json:"order_number"`
		UserID      *int32 `json:"user_id"`
		NewStatus   string `json:"new_status"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return err
	}

	if payload.UserID != nil {
		_ = c.notificationService.Send(context.Background(), dto.SendNotificationInput{
			UserID: *payload.UserID,
			Type:   "order_status",
			Title:  "Order Status Updated",
			Body:   fmt.Sprintf("Your order #%s status is now: %s", payload.OrderNumber, payload.NewStatus),
			Data:   map[string]any{"order_number": payload.OrderNumber, "status": payload.NewStatus},
		})
	}

	return nil
}

func (c *TopicConsumer) handlePaymentSucceeded(body []byte) error {
	var event rabbitmq.PaymentSucceededEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return err
	}

	c.logger.Info().Str("order_number", event.OrderNumber).Float64("amount", event.Amount).Msg("Payment succeeded notification recorded")
	return nil
}

func (c *TopicConsumer) handlePaymentFailed(body []byte) error {
	var event rabbitmq.PaymentFailedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return err
	}

	c.logger.Warn().Str("order_number", event.OrderNumber).Str("reason", event.Reason).Msg("Payment failed notification recorded")
	return nil
}

func (c *TopicConsumer) handleInventoryStockLow(body []byte) error {
	var payload struct {
		ProductID int32  `json:"product_id"`
		Name      string `json:"name"`
		Stock     int32  `json:"stock"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return err
	}

	return c.notificationService.NotifyAdmins(context.Background(), dto.AdminNotifyInput{
		Type:  "inventory",
		Title: "Low Stock Warning",
		Body:  fmt.Sprintf("Product %s (ID %d) stock is low: %d remaining", payload.Name, payload.ProductID, payload.Stock),
		Data:  map[string]any{"product_id": payload.ProductID, "stock": payload.Stock},
	})
}

func (c *TopicConsumer) handleGenericNotification(event rabbitmq.NotificationEvent) error {
	if event.UserID != nil {
		return c.notificationService.Send(context.Background(), dto.SendNotificationInput{
			UserID: *event.UserID,
			Type:   event.Type,
			Title:  event.Title,
			Body:   event.Body,
			Data:   event.Data,
		})
	}

	return c.notificationService.NotifyAdmins(context.Background(), dto.AdminNotifyInput{
		Type:  event.Type,
		Title: event.Title,
		Body:  event.Body,
		Data:  event.Data,
	})
}
