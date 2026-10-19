package worker

import (
	"context"
	"encoding/json"
	"fmt"
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/pkg/rabbitmq"

	"github.com/google/uuid"
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
	return oc.HandleOrderCreated(headers, body)
}

func (oc *OrderConsumer) HandleOrderCreated(headers amqp.Table, body []byte) error {
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

	// 1. Asynchronously remove purchased items from customer's cart (G8 FIX: Single source of truth for cart cleanup)
	if oc.cartService != nil {
		cartCleaned := false
		if len(event.PurchasedItemUUIDs) > 0 && event.UserUUID != "" {
			if userUUID, uErr := uuid.Parse(event.UserUUID); uErr == nil {
				itemUUIDs := make([]uuid.UUID, 0, len(event.PurchasedItemUUIDs))
				for _, idStr := range event.PurchasedItemUUIDs {
					if itemUUID, pErr := uuid.Parse(idStr); pErr == nil {
						itemUUIDs = append(itemUUIDs, itemUUID)
					}
				}
				if len(itemUUIDs) > 0 {
					if uuidPort, ok := oc.cartService.(v1service.OrderCartUUIDPort); ok {
						if err := uuidPort.RemovePurchasedItemsByUUID(ctx, userUUID, itemUUIDs); err != nil {
							if oc.logger != nil {
								oc.logger.Warn().
									Err(err).
									Str("user_uuid", event.UserUUID).
									Str("order_uuid", event.OrderUUID).
									Msg("Failed to clean up cart by UUID, falling back to ID")
							}
						} else {
							cartCleaned = true
						}
					}
				}
			}
		}

		if !cartCleaned && len(event.PurchasedItems) > 0 {
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

func (oc *OrderConsumer) HandleOrderCancelled(headers amqp.Table, body []byte) error {
	var event rabbitmq.OrderCancelledEvent
	if err := json.Unmarshal(body, &event); err != nil {
		if oc.logger != nil {
			oc.logger.Error().Err(err).Msg("Failed to unmarshal order cancelled event")
		}
		return err
	}

	if oc.logger != nil {
		oc.logger.Info().
			Str("event_id", event.EventID).
			Str("order_uuid", event.OrderUUID).
			Str("order_number", event.OrderNumber).
			Int32("user_id", event.UserID).
			Msg("Processing order cancelled event")
	}

	ctx := context.Background()
	if oc.notificationService != nil {
		bodyText := fmt.Sprintf("Your order #%s has been cancelled.", event.OrderNumber)
		if event.Reason != "" {
			bodyText = fmt.Sprintf("Your order #%s has been cancelled. Reason: %s", event.OrderNumber, event.Reason)
		}

		_ = oc.notificationService.Send(ctx, v1dto.SendNotificationInput{
			UserID: event.UserID,
			Type:   "order_cancelled",
			Title:  "Order Cancelled",
			Body:   bodyText,
			Data:   map[string]any{"order_uuid": event.OrderUUID},
		})

		_ = oc.notificationService.NotifyAdmins(ctx, v1dto.AdminNotifyInput{
			Type:  "order",
			Title: "Order Cancelled",
			Body:  fmt.Sprintf("Order #%s has been cancelled. Reason: %s", event.OrderNumber, event.Reason),
			Data: map[string]any{
				"order_uuid":   event.OrderUUID,
				"order_number": event.OrderNumber,
				"link":         fmt.Sprintf("/orders/%s", event.OrderUUID),
			},
		})
	}
	return nil
}

func (oc *OrderConsumer) HandleOrderStatusChanged(headers amqp.Table, body []byte) error {
	var event rabbitmq.OrderStatusChangedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		if oc.logger != nil {
			oc.logger.Error().Err(err).Msg("Failed to unmarshal order status changed event")
		}
		return err
	}

	if oc.logger != nil {
		oc.logger.Info().
			Str("event_id", event.EventID).
			Str("order_uuid", event.OrderUUID).
			Int32("new_status", event.NewStatus).
			Msg("Processing order status changed event")
	}

	ctx := context.Background()
	if oc.notificationService != nil {
		var title, bodyText string
		switch event.NewStatus {
		case v1service.OrderStatusConfirmed:
			title = "Order Confirmed"
			bodyText = fmt.Sprintf("Your order #%s has been confirmed.", event.OrderNumber)
		case v1service.OrderStatusShipped:
			title = "Order Shipped"
			if event.ShippingCode != "" {
				bodyText = fmt.Sprintf("Your order #%s has been shipped. Tracking code: %s", event.OrderNumber, event.ShippingCode)
			} else {
				bodyText = fmt.Sprintf("Your order #%s is now on its way.", event.OrderNumber)
			}
		case v1service.OrderStatusDelivered:
			title = "Order Delivered"
			bodyText = fmt.Sprintf("Your order #%s has been delivered successfully.", event.OrderNumber)
		default:
			title = "Order Status Updated"
			bodyText = fmt.Sprintf("Your order #%s status has been updated.", event.OrderNumber)
		}

		_ = oc.notificationService.Send(ctx, v1dto.SendNotificationInput{
			UserID: event.UserID,
			Type:   "order_status",
			Title:  title,
			Body:   bodyText,
			Data: map[string]any{
				"order_uuid":     event.OrderUUID,
				"new_status":     event.NewStatus,
				"shipping_code":  event.ShippingCode,
			},
		})
	}
	return nil
}

func (oc *OrderConsumer) HandleOrderReturnRequested(headers amqp.Table, body []byte) error {
	var event rabbitmq.OrderReturnRequestedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		if oc.logger != nil {
			oc.logger.Error().Err(err).Msg("Failed to unmarshal order return requested event")
		}
		return err
	}

	if oc.logger != nil {
		oc.logger.Info().
			Str("event_id", event.EventID).
			Str("return_uuid", event.ReturnUUID).
			Str("order_uuid", event.OrderUUID).
			Msg("Processing order return requested event")
	}

	ctx := context.Background()
	if oc.notificationService != nil {
		_ = oc.notificationService.Send(ctx, v1dto.SendNotificationInput{
			UserID: event.UserID,
			Type:   "return_requested",
			Title:  "Return Request Received",
			Body:   fmt.Sprintf("Your return request for order #%s is under review.", event.OrderNumber),
			Data: map[string]any{
				"order_uuid":  event.OrderUUID,
				"return_uuid": event.ReturnUUID,
			},
		})

		_ = oc.notificationService.NotifyAdmins(ctx, v1dto.AdminNotifyInput{
			Type:  "return",
			Title: "New Return Requested",
			Body:  fmt.Sprintf("Return requested for order #%s: %s", event.OrderNumber, event.ReturnReason),
			Data: map[string]any{
				"order_uuid":  event.OrderUUID,
				"return_uuid": event.ReturnUUID,
				"link":        "/returns",
			},
		})
	}
	return nil
}

func (oc *OrderConsumer) HandleOrderReturnUpdated(headers amqp.Table, body []byte) error {
	var event rabbitmq.OrderReturnUpdatedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		if oc.logger != nil {
			oc.logger.Error().Err(err).Msg("Failed to unmarshal order return updated event")
		}
		return err
	}

	if oc.logger != nil {
		oc.logger.Info().
			Str("event_id", event.EventID).
			Str("return_uuid", event.ReturnUUID).
			Int16("return_status", event.ReturnStatus).
			Msg("Processing order return updated event")
	}

	ctx := context.Background()
	if oc.notificationService != nil {
		switch event.ReturnStatus {
		case v1service.OrderReturnStatusApproved:
			_ = oc.notificationService.Send(ctx, v1dto.SendNotificationInput{
				UserID: event.UserID,
				Type:   "return_approved",
				Title:  "Return Request Approved",
				Body:   fmt.Sprintf("Your return request for order #%s has been approved. Please submit the return tracking code.", event.OrderNumber),
				Data: map[string]any{
					"order_uuid":  event.OrderUUID,
					"return_uuid": event.ReturnUUID,
				},
			})
		case v1service.OrderReturnStatusRejected:
			_ = oc.notificationService.Send(ctx, v1dto.SendNotificationInput{
				UserID: event.UserID,
				Type:   "return_rejected",
				Title:  "Return Request Rejected",
				Body:   fmt.Sprintf("Your return request for order #%s was rejected. Reason: %s", event.OrderNumber, event.AdminNote),
				Data: map[string]any{
					"order_uuid":  event.OrderUUID,
					"return_uuid": event.ReturnUUID,
				},
			})
		case v1service.OrderReturnStatusCompleted:
			_ = oc.notificationService.Send(ctx, v1dto.SendNotificationInput{
				UserID: event.UserID,
				Type:   "return_refunded",
				Title:  "Refund Completed",
				Body:   fmt.Sprintf("Return request for order #%s has been completed and refunded.", event.OrderNumber),
				Data: map[string]any{
					"order_uuid":  event.OrderUUID,
					"return_uuid": event.ReturnUUID,
				},
			})
		case v1service.OrderReturnStatusCancelled:
			_ = oc.notificationService.NotifyAdmins(ctx, v1dto.AdminNotifyInput{
				Type:  "return",
				Title: "Return Request Cancelled",
				Body:  fmt.Sprintf("Customer cancelled return request for order #%s.", event.OrderNumber),
				Data: map[string]any{
					"order_uuid":  event.OrderUUID,
					"return_uuid": event.ReturnUUID,
					"link":        "/returns",
				},
			})
		}
	}
	return nil
}
