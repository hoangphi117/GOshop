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

	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
)

type TopicConsumer struct {
	notificationService service.NotificationService
	mailService         mail.EmailProviderService
	discordNotifier     discord.Notifier
	logger              *zerolog.Logger
	db                  *pgxpool.Pool
}

func NewTopicConsumer(
	notificationService service.NotificationService,
	mailService mail.EmailProviderService,
	discordNotifier discord.Notifier,
	logger *zerolog.Logger,
	dbPool ...*pgxpool.Pool,
) *TopicConsumer {
	var pool *pgxpool.Pool
	if len(dbPool) > 0 {
		pool = dbPool[0]
	}

	return &TopicConsumer{
		notificationService: notificationService,
		mailService:         mailService,
		discordNotifier:     discordNotifier,
		logger:              logger,
		db:                  pool,
	}
}

// HandleEvent dispatches events based on the x-routing-key header or payload inspection
func (c *TopicConsumer) HandleEvent(headers amqp.Table, body []byte) error {
	routingKey, _ := headers["x-routing-key"].(string)
	traceID, _ := headers["trace_id"].(string)
	if traceID == "" {
		traceID, _ = headers["x-message-id"].(string)
	}

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
		OrderID     any     `json:"order_id"`
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

	if payload.UserID != nil && *payload.UserID > 0 {
		_ = c.notificationService.Send(context.Background(), dto.SendNotificationInput{
			UserID: *payload.UserID,
			Type:   "order",
			Title:  title,
			Body:   msgBody,
			Data:   map[string]any{"order_number": payload.OrderNumber, "total": payload.TotalAmount},
		})
	} else if payload.UserUUID != "" {
		if uUUID, err := uuid.Parse(payload.UserUUID); err == nil && uUUID != uuid.Nil {
			_, _ = c.notificationService.CreateNotification(context.Background(), dto.CreateNotificationInput{
				UserUUID: uUUID,
				Type:     "order",
				Title:    title,
				Body:     msgBody,
				Data:     map[string]any{"order_number": payload.OrderNumber, "total": payload.TotalAmount},
			})
		}
	} else if c.db != nil && payload.OrderNumber != "" {
		var uUUID uuid.UUID
		_ = c.db.QueryRow(context.Background(),
			"SELECT order_user_uuid FROM orders WHERE order_number = $1 LIMIT 1",
			payload.OrderNumber).Scan(&uUUID)
		if uUUID != uuid.Nil {
			_, _ = c.notificationService.CreateNotification(context.Background(), dto.CreateNotificationInput{
				UserUUID: uUUID,
				Type:     "order",
				Title:    title,
				Body:     msgBody,
				Data:     map[string]any{"order_number": payload.OrderNumber, "total": payload.TotalAmount},
			})
		}
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
		UserUUID    string `json:"user_uuid"`
		UserID      *int32 `json:"user_id"`
		NewStatus   any    `json:"new_status"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return err
	}

	statusStr := formatOrderStatus(payload.NewStatus)
	if statusStr == "Returned" {
		c.logger.Info().
			Str("order_number", payload.OrderNumber).
			Msg("Skipping generic returned notification; return event owns this notification")
		return nil
	}
	var title, msgBody string
	switch statusStr {
	case "Confirmed":
		title = "Order Confirmed"
		msgBody = fmt.Sprintf("Your order #%s has been confirmed.", payload.OrderNumber)
	case "Shipped":
		title = "Order Shipped"
		msgBody = fmt.Sprintf("Your order #%s has been shipped out.", payload.OrderNumber)
	case "Delivered":
		title = "Order Delivered"
		msgBody = fmt.Sprintf("Your order #%s has been delivered successfully.", payload.OrderNumber)
	case "Cancelled":
		title = "Order Cancelled"
		msgBody = fmt.Sprintf("Your order #%s has been cancelled.", payload.OrderNumber)
	default:
		title = "Order Status Updated"
		msgBody = fmt.Sprintf("Your order #%s status is now: %s", payload.OrderNumber, statusStr)
	}

	if payload.UserID != nil && *payload.UserID > 0 {
		_ = c.notificationService.Send(context.Background(), dto.SendNotificationInput{
			UserID: *payload.UserID,
			Type:   "order_status",
			Title:  title,
			Body:   msgBody,
			Data:   map[string]any{"order_number": payload.OrderNumber, "status": statusStr},
		})
	} else if payload.UserUUID != "" {
		if uUUID, err := uuid.Parse(payload.UserUUID); err == nil && uUUID != uuid.Nil {
			_, _ = c.notificationService.CreateNotification(context.Background(), dto.CreateNotificationInput{
				UserUUID: uUUID,
				Type:     "order_status",
				Title:    title,
				Body:     msgBody,
				Data:     map[string]any{"order_number": payload.OrderNumber, "status": statusStr},
			})
		}
	} else if c.db != nil && payload.OrderNumber != "" {
		var uUUID uuid.UUID
		_ = c.db.QueryRow(context.Background(),
			"SELECT order_user_uuid FROM orders WHERE order_number = $1 LIMIT 1",
			payload.OrderNumber).Scan(&uUUID)
		if uUUID != uuid.Nil {
			_, _ = c.notificationService.CreateNotification(context.Background(), dto.CreateNotificationInput{
				UserUUID: uUUID,
				Type:     "order_status",
				Title:    title,
				Body:     msgBody,
				Data:     map[string]any{"order_number": payload.OrderNumber, "status": statusStr},
			})
		}
	}

	return nil
}

func formatOrderStatus(status any) string {
	if status == nil {
		return "Updated"
	}
	switch v := status.(type) {
	case float64:
		return orderStatusIntToString(int32(v))
	case int32:
		return orderStatusIntToString(v)
	case int:
		return orderStatusIntToString(int32(v))
	case string:
		if num, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return orderStatusIntToString(int32(num))
		}
		if v == "" {
			return "Updated"
		}
		return strings.Title(strings.ToLower(v))
	default:
		return fmt.Sprintf("%v", v)
	}
}

func orderStatusIntToString(status int32) string {
	switch status {
	case 1:
		return "Pending"
	case 2:
		return "Confirmed"
	case 3:
		return "Shipped"
	case 4:
		return "Delivered"
	case 5:
		return "Cancelled"
	case 6:
		return "Returned"
	default:
		return fmt.Sprintf("Status %d", status)
	}
}

func (c *TopicConsumer) handlePaymentSucceeded(body []byte) error {
	var event rabbitmq.PaymentSucceededEvent
	if err := json.Unmarshal(body, &event); err != nil {
		var rawStr string
		if unquoteErr := json.Unmarshal(body, &rawStr); unquoteErr == nil {
			if retryErr := json.Unmarshal([]byte(rawStr), &event); retryErr != nil {
				return err
			}
		} else {
			return err
		}
	}

	c.logger.Info().Str("order_number", event.OrderNumber).Float64("amount", event.Amount).Msg("Payment succeeded event received (customer notification skipped)")
	return nil
}

func (c *TopicConsumer) handlePaymentFailed(body []byte) error {
	var event rabbitmq.PaymentFailedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		var rawStr string
		if unquoteErr := json.Unmarshal(body, &rawStr); unquoteErr == nil {
			if retryErr := json.Unmarshal([]byte(rawStr), &event); retryErr != nil {
				return err
			}
		} else {
			return err
		}
	}

	c.logger.Warn().Str("order_number", event.OrderNumber).Str("reason", event.Reason).Msg("Payment failed event received (customer notification skipped)")
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
