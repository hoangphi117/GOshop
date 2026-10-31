package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"goshop/contracts/events"
	"goshop/pkg/db"
	"goshop/pkg/logger"
	"goshop/pkg/utils"
	"goshop/services/order/internal/db/sqlc"
	v1dto "goshop/services/order/internal/dto"
)

// HandleShippingWebhook processes incoming status updates from GHN shipping gateway.
func (s *orderShippingService) HandleShippingWebhook(ctx context.Context, rawBody []byte, payload v1dto.GHNWebhookPayload) error {
	ghnStatus := strings.ToLower(strings.TrimSpace(payload.Status))

	var eventTime int64
	if !payload.Time.IsZero() {
		eventTime = payload.Time.Unix()
	}
	externalEventID := fmt.Sprintf("%s_%s_%d", payload.OrderCode, ghnStatus, eventTime)

	eventParams := sqlc.InsertWebhookEventParams{
		WeProvider:        "ghn",
		WeExternalEventID: externalEventID,
		WeEventType:       ghnStatus,
		WeRawBody:         string(rawBody),
	}

	event, err := s.webhookRepo.InsertEvent(ctx, nil, eventParams)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Log.Info().
				Str("provider", "ghn").
				Str("event_id", externalEventID).
				Msg("Duplicate GHN webhook event received (ON CONFLICT), acknowledging gateway")
			return nil
		}
		logger.Log.Error().
			Err(err).
			Str("provider", "ghn").
			Str("event_id", externalEventID).
			Msg("Database failure while inserting GHN webhook event")
		return utils.WrapError(utils.ErrCodeInternal, "failed to persist webhook event", err)
	}

	if event.WeID == 0 {
		logger.Log.Info().
			Str("provider", "ghn").
			Str("event_id", externalEventID).
			Msg("Duplicate GHN webhook event received (WeID=0), acknowledging gateway")
		return nil
	}

	order, err := s.orderRepo.GetByShippingCode(ctx, payload.OrderCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Log.Warn().Str("shipping_code", payload.OrderCode).Msg("Received webhook for unknown shipping code")
			_ = s.webhookRepo.MarkProcessed(ctx, nil, event.WeID)
			return nil
		}
		return utils.WrapError(utils.ErrCodeInternal, "Failed to get order by shipping code", err)
	}

	switch ghnStatus {
	case "delivered":
		if order.OrderStatus == OrderStatusDelivered {
			_ = s.webhookRepo.MarkProcessed(ctx, nil, event.WeID)
			return nil
		}

		var updatedOrder sqlc.Order
		err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
			tx, _ := db.GetTx(txCtx)
			var err error
			updatedOrder, err = s.orderRepo.MarkDeliveredByShippingCode(txCtx, tx, payload.OrderCode)
			if err != nil {
				return err
			}
			if s.trackingRepo != nil {
				eventNote := "Order delivered successfully via shipping provider."
				_, err = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
					EventOrderID: order.OrderID,
					EventStatus:  "delivered",
					EventNote:    &eventNote,
					EventSource:  "ghn",
				})
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to update order status to delivered", err)
		}

		if s.cache != nil {
			_ = s.cache.Clear(fmt.Sprintf("order:%s", updatedOrder.OrderUuid.String()))
		}
		logger.Log.Info().Str("order_number", order.OrderNumber).Str("code", payload.OrderCode).Msg("Order marked as DELIVERED via GHN Webhook")

		if s.rabbitmq != nil {
			evt := events.NewOrderStatusChangedEvent(
				uuid.New().String(),
				updatedOrder.OrderUuid.String(),
				order.OrderNumber,
				"",
				updatedOrder.OrderID,
				updatedOrder.OrderUserID,
				order.OrderStatus,
				OrderStatusDelivered,
				payload.OrderCode,
			)
			_ = s.rabbitmq.Publish(context.Background(), events.QueueOrderStatusChanged, evt)
			_ = s.rabbitmq.PublishTopic(context.Background(), events.RoutingKeyOrderStatusChanged, evt)
		}

	case "delivering":
		eventNote := "Order is currently being delivered by shipping provider."
		if payload.Warehouse != "" {
			eventNote = fmt.Sprintf("Order is being shipped from %s", payload.Warehouse)
		}
		_, _ = s.trackingRepo.InsertEvent(ctx, nil, sqlc.InsertTrackingEventParams{
			EventOrderID: order.OrderID,
			EventStatus:  "delivering",
			EventNote:    &eventNote,
			EventSource:  "ghn",
		})

	case "delivery_fail":
		eventNote := "Delivery attempt failed."
		if payload.Description != "" {
			eventNote = fmt.Sprintf("Delivery failed: %s", payload.Description)
		}

		_, _ = s.trackingRepo.InsertEvent(ctx, nil, sqlc.InsertTrackingEventParams{
			EventOrderID: order.OrderID,
			EventStatus:  "delivery_fail",
			EventNote:    &eventNote,
			EventSource:  "ghn",
		})

	case "return", "returned":
		eventNote := "Order is being returned by shipping provider."
		if payload.Description != "" {
			eventNote = fmt.Sprintf("Order returned: %s", payload.Description)
		}

		_, _ = s.trackingRepo.InsertEvent(ctx, nil, sqlc.InsertTrackingEventParams{
			EventOrderID: order.OrderID,
			EventStatus:  "returned",
			EventNote:    &eventNote,
			EventSource:  "ghn",
		})

	default:
		eventNote := fmt.Sprintf("GHN update status: %s", ghnStatus)
		if payload.Description != "" {
			eventNote = payload.Description
		}
		_, _ = s.trackingRepo.InsertEvent(ctx, nil, sqlc.InsertTrackingEventParams{
			EventOrderID: order.OrderID,
			EventStatus:  ghnStatus,
			EventNote:    &eventNote,
			EventSource:  "ghn",
		})
	}

	if err := s.webhookRepo.MarkProcessed(ctx, nil, event.WeID); err != nil {
		logger.Log.Error().
			Int32("webhook_event_id", event.WeID).
			Msg("Failed to mark webhook event as processed")
	}

	return nil
}
