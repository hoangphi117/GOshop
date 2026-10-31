package consumer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"goshop/contracts/events"
	"goshop/services/notification/internal/dto"
	"goshop/services/notification/internal/repository"
	"goshop/services/notification/internal/service"
	"goshop/shared/pkg/discord"
	"goshop/shared/pkg/mail"
	"goshop/shared/pkg/messaging"
)

// consumerName identifies this consumer in the processed_events ledger.
// Change it only when the consumer needs to replay events it already applied.
const consumerName = "notification.events.v3"

type TopicConsumer struct {
	notificationService service.NotificationService
	mailService         mail.EmailProviderService
	discordNotifier     discord.Notifier
	logger              *zerolog.Logger
	db                  *pgxpool.Pool
	ledger              repository.EventLedgerRepository
}

func NewTopicConsumer(
	notificationService service.NotificationService,
	mailService mail.EmailProviderService,
	discordNotifier discord.Notifier,
	logger *zerolog.Logger,
	dbPool ...*pgxpool.Pool,
) *TopicConsumer {
	return NewTopicConsumerWithLedger(notificationService, mailService, discordNotifier, logger, nil, dbPool...)
}

// NewTopicConsumerWithLedger is the explicit-construction form used by the app
// wiring and tests. A nil ledger disables processing: the consumer refuses to
// apply events it cannot deduplicate.
func NewTopicConsumerWithLedger(
	notificationService service.NotificationService,
	mailService mail.EmailProviderService,
	discordNotifier discord.Notifier,
	logger *zerolog.Logger,
	ledger repository.EventLedgerRepository,
	dbPool ...*pgxpool.Pool,
) *TopicConsumer {
	var pool *pgxpool.Pool
	if len(dbPool) > 0 {
		pool = dbPool[0]
	}

	if ledger == nil && pool != nil {
		ledger = repository.NewSqlEventLedgerRepository(pool)
	}

	return &TopicConsumer{
		notificationService: notificationService,
		mailService:         mailService,
		discordNotifier:     discordNotifier,
		logger:              logger,
		db:                  pool,
		ledger:              ledger,
	}
}

// HandleEvent dispatches events based on the x-routing-key header or payload inspection.
//
// Idempotency: every event carries an event_id. The event is applied at most once
// per consumer: if the ledger already holds (event_id, consumerName) the event is
// acknowledged without side effects. The ledger row is written only after all side
// effects succeeded, so a failed side effect leaves the event retryable (the returned
// error makes the broker redeliver it).
func (c *TopicConsumer) HandleEvent(headers messaging.Header, body []byte) error {
	routingKey := headers.Get("x-routing-key")
	traceID := headers.Get("trace_id")
	if traceID == "" {
		traceID = headers.Get("x-message-id")
	}

	eventID := extractEventID(body)

	// A nil ledger (misconfiguration) must never silently disable idempotency.
	if c.ledger == nil {
		c.logger.Error().
			Str("event_id", eventID).
			Str("trace_id", traceID).
			Str("routing_key", routingKey).
			Msg("Event ledger not configured; refusing to process event without idempotency guarantee")
		return errors.New("event ledger not configured")
	}

	ctx := context.Background()
	logger := c.logger.With().
		Str("event_id", eventID).
		Str("trace_id", traceID).
		Str("routing_key", routingKey).
		Logger()

	processed, err := c.ledger.IsEventProcessed(ctx, eventID, consumerName)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to check event ledger")
		return err
	}
	if processed {
		logger.Info().Msg("Duplicate event delivery ignored; already processed")
		return nil
	}

	logger.Info().Msg("Processing topic event in Notification Service")

	if err := c.dispatch(ctx, routingKey, body); err != nil {
		logger.Error().Err(err).Msg("Event processing failed; leaving event retryable")
		return err
	}

	if err := c.ledger.MarkEventProcessed(ctx, eventID, consumerName); err != nil {
		logger.Error().Err(err).Msg("Failed to record processed event")
		return err
	}

	logger.Info().Str("consumer", consumerName).Msg("Event processed and recorded")
	return nil
}

func (c *TopicConsumer) dispatch(ctx context.Context, routingKey string, body []byte) error {
	switch routingKey {
	case events.RoutingKeyOrderCreated:
		return c.handleOrderCreated(ctx, body)
	case events.RoutingKeyOrderStatusChanged, events.RoutingKeyOrderShipped, events.RoutingKeyOrderDelivered:
		return c.handleOrderStatusChanged(ctx, routingKey, body)
	case events.RoutingKeyPaymentSucceeded:
		return c.handlePaymentSucceeded(body)
	case events.RoutingKeyPaymentFailed:
		return c.handlePaymentFailed(body)
	case events.RoutingKeyInventoryStockLow:
		return c.handleInventoryStockLow(ctx, body)
	default:
		// Fallback: check if body is generic NotificationEvent
		var notifEvent events.NotificationEvent
		if err := json.Unmarshal(body, &notifEvent); err != nil {
			c.logger.Debug().Str("routing_key", routingKey).Err(err).Msg("Ignored unhandled routing key")
			return nil
		}
		if notifEvent.Type == "" {
			c.logger.Debug().Str("routing_key", routingKey).Msg("Ignored unhandled routing key")
			return nil
		}
		return c.handleGenericNotification(ctx, notifEvent)
	}
}

// extractEventID resolves the durable identity of an event: the envelope's
// event_id when present, otherwise a content hash. Legacy payloads without an
// event_id thus still deduplicate on redelivery (identical bytes → identical id).
func extractEventID(body []byte) string {
	var envelope struct {
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && strings.TrimSpace(envelope.EventID) != "" {
		return strings.TrimSpace(envelope.EventID)
	}
	digest := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func (c *TopicConsumer) handleOrderCreated(ctx context.Context, body []byte) error {
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

	if err := c.notifyOrderCustomer(ctx, orderRecipient{
		UserID:      payload.UserID,
		UserUUID:    payload.UserUUID,
		OrderNumber: payload.OrderNumber,
	}, "order", title, msgBody, map[string]any{"order_number": payload.OrderNumber, "total": payload.TotalAmount}); err != nil {
		return err
	}

	// Notify admins
	return c.notificationService.NotifyAdmins(ctx, dto.AdminNotifyInput{
		Type:  "order",
		Title: "New Order Received",
		Body:  fmt.Sprintf("Order #%s received for $%.2f", payload.OrderNumber, payload.TotalAmount),
		Data:  map[string]any{"order_number": payload.OrderNumber},
	})
}

func (c *TopicConsumer) handleOrderStatusChanged(ctx context.Context, routingKey string, body []byte) error {
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
			Str("routing_key", routingKey).
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

	return c.notifyOrderCustomer(ctx, orderRecipient{
		UserID:      payload.UserID,
		UserUUID:    payload.UserUUID,
		OrderNumber: payload.OrderNumber,
	}, "order_status", title, msgBody, map[string]any{"order_number": payload.OrderNumber, "status": statusStr})
}

// orderRecipient carries the ways an order event identifies its customer.
type orderRecipient struct {
	UserID      *int32
	UserUUID    string
	OrderNumber string
}

// notifyOrderCustomer creates the customer-facing in-app notification. It resolves
// the recipient in order of reliability (numeric user id, user UUID, order lookup)
// and returns every failure instead of dropping the notification silently.
func (c *TopicConsumer) notifyOrderCustomer(
	ctx context.Context,
	recipient orderRecipient,
	notifType, title, msgBody string,
	data map[string]any,
) error {
	if recipient.UserID != nil && *recipient.UserID > 0 {
		return c.notificationService.Send(ctx, dto.SendNotificationInput{
			UserID: *recipient.UserID,
			Type:   notifType,
			Title:  title,
			Body:   msgBody,
			Data:   data,
		})
	}

	if recipient.UserUUID != "" {
		userUUID, err := uuid.Parse(recipient.UserUUID)
		if err != nil {
			return fmt.Errorf("parse user_uuid %q: %w", recipient.UserUUID, err)
		}
		if userUUID == uuid.Nil {
			return errors.New("event carries a nil user_uuid")
		}
		if _, err := c.notificationService.CreateNotification(ctx, dto.CreateNotificationInput{
			UserUUID: userUUID,
			Type:     notifType,
			Title:    title,
			Body:     msgBody,
			Data:     data,
		}); err != nil {
			return err
		}
		return nil
	}

	if c.db == nil || recipient.OrderNumber == "" {
		c.logger.Warn().
			Str("order_number", recipient.OrderNumber).
			Str("type", notifType).
			Msg("Order event has no resolvable recipient; no customer notification created")
		return nil
	}

	var userUUID uuid.UUID
	err := c.db.QueryRow(ctx,
		"SELECT order_user_uuid FROM orders WHERE order_number = $1 LIMIT 1",
		recipient.OrderNumber).Scan(&userUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.logger.Warn().
			Str("order_number", recipient.OrderNumber).
			Str("type", notifType).
			Msg("Order not found; no customer notification created")
		return nil
	}
	if err != nil {
		return fmt.Errorf("lookup recipient for order %s: %w", recipient.OrderNumber, err)
	}
	if userUUID == uuid.Nil {
		return fmt.Errorf("order %s has no customer uuid", recipient.OrderNumber)
	}

	_, err = c.notificationService.CreateNotification(ctx, dto.CreateNotificationInput{
		UserUUID: userUUID,
		Type:     notifType,
		Title:    title,
		Body:     msgBody,
		Data:     data,
	})
	return err
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
		return cases.Title(language.Und).String(strings.ToLower(v))
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
	var event events.PaymentSucceededEvent
	if err := decodeEventPayload(body, &event); err != nil {
		return err
	}

	c.logger.Info().Str("event_id", event.EventID).Str("order_number", event.OrderNumber).Float64("amount", event.Amount).Msg("Payment succeeded event received (customer notification skipped)")
	return nil
}

func (c *TopicConsumer) handlePaymentFailed(body []byte) error {
	var event events.PaymentFailedEvent
	if err := decodeEventPayload(body, &event); err != nil {
		return err
	}

	c.logger.Warn().Str("event_id", event.EventID).Str("order_number", event.OrderNumber).Str("reason", event.Reason).Msg("Payment failed event received (customer notification skipped)")
	return nil
}

func (c *TopicConsumer) handleInventoryStockLow(ctx context.Context, body []byte) error {
	var payload struct {
		ProductID   int32  `json:"product_id"`
		ProductUUID string `json:"product_uuid"`
		Stock       int32  `json:"stock"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return err
	}

	return c.notificationService.NotifyAdmins(ctx, dto.AdminNotifyInput{
		Type:  "inventory",
		Title: "Low Stock Warning",
		Body:  fmt.Sprintf("Product %s (ID %d) stock is low: %d remaining", payload.ProductUUID, payload.ProductID, payload.Stock),
		Data:  map[string]any{"product_id": payload.ProductID, "product_uuid": payload.ProductUUID, "stock": payload.Stock},
	})
}

func (c *TopicConsumer) handleGenericNotification(ctx context.Context, event events.NotificationEvent) error {
	if event.UserID != nil {
		return c.notificationService.Send(ctx, dto.SendNotificationInput{
			UserID: *event.UserID,
			Type:   event.Type,
			Title:  event.Title,
			Body:   event.Body,
			Data:   event.Data,
		})
	}

	return c.notificationService.NotifyAdmins(ctx, dto.AdminNotifyInput{
		Type:  event.Type,
		Title: event.Title,
		Body:  event.Body,
		Data:  event.Data,
	})
}

// decodeEventPayload unmarshals an event body, tolerating bodies that were
// double-encoded as a JSON string by the publisher.
func decodeEventPayload(body []byte, target any) error {
	err := json.Unmarshal(body, target)
	if err == nil {
		return nil
	}

	var rawStr string
	if unquoteErr := json.Unmarshal(body, &rawStr); unquoteErr != nil {
		return err
	}
	if retryErr := json.Unmarshal([]byte(rawStr), target); retryErr != nil {
		return retryErr
	}
	return nil
}
