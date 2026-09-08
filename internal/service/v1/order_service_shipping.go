package v1service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/utils"
	"goshop/pkg/logger"
	"goshop/pkg/shipping"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	amqp "github.com/rabbitmq/amqp091-go"
)

func (s *orderService) CreateShipping(ctx *gin.Context, orderUUID uuid.UUID, input v1dto.CreateShippingInput) (v1dto.AsyncShippingResponseDTO, error) {
	reqCtx := ctx.Request.Context()

	order, err := s.orderRepo.GetByUUID(reqCtx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v1dto.AsyncShippingResponseDTO{}, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return v1dto.AsyncShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
	}

	if order.OrderStatus != OrderStatusConfirmed {
		return v1dto.AsyncShippingResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest,
			fmt.Sprintf("Cannot create shipping for order with status %d. Order must be Confirmed.", order.OrderStatus))
	}

	if order.OrderShippingCode != nil && strings.TrimSpace(*order.OrderShippingCode) != "" {
		return v1dto.AsyncShippingResponseDTO{},
			utils.NewError(utils.ErrCodeConflict, "Shipping already created for this order")
	}

	msg := v1dto.CreateShippingQueueMessage{
		OrderUUID:  orderUUID.String(),
		Input:      input,
		EnqueuedAt: time.Now(),
	}

	if err := s.rabbitmq.Publish(reqCtx, QueueShippingCreate, msg); err != nil {
		logger.Log.Error().Err(err).Str("order_uuid", orderUUID.String()).Msg("Failed to enqueue shipping creation")
		return v1dto.AsyncShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to enqueue shipping creation", err)
	}

	return v1dto.AsyncShippingResponseDTO{
		OrderUUID: orderUUID.String(),
		Status:    "processing",
		Message:   "Shipping order creation request received and is being processed by the system",
	}, nil
}

func (s *orderService) ProcessShippingOrderMessage(headers amqp.Table, body []byte) error {
	ctx := context.Background()

	var msg v1dto.CreateShippingQueueMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to unmarshal shipping creation message")
		return err
	}

	orderUUID, err := uuid.Parse(msg.OrderUUID)
	if err != nil {
		return err
	}

	order, err := s.orderRepo.GetByUUID(ctx, orderUUID)
	if err != nil {
		return err
	}

	if order.OrderStatus != OrderStatusConfirmed {
		logger.Log.Warn().Str("order_uuid", msg.OrderUUID).Msg("Order is no longer Confirmed, skipping shipping creation")
		return nil
	}

	orderItems, err := s.orderItemRepo.GetByOrderID(ctx, order.OrderID)
	if err != nil {
		return err
	}

	shippingItems := make([]shipping.ShippingItem, 0, len(orderItems))
	for _, item := range orderItems {
		shippingItems = append(shippingItems, shipping.ShippingItem{
			Name:     item.OrderItemProductName,
			Quantity: int(item.OrderItemQuantity),
			Price:    utils.NumericToFloat(item.OrderItemPrice),
			Weight:   msg.Input.Weight / len(orderItems),
		})
	}

	var codAmount float64
	if order.OrderPaymentMethod == OrderPaymentMethodCOD && order.OrderPaymentStatus == OrderPaymentStatusUnpaid {
		codAmount = utils.NumericToFloat(order.OrderTotalAmount)
	}
	provider, err := s.shippingFactory.GetProvider(msg.Input.Provider)
	if err != nil {
		return err
	}
	ghnReq := shipping.CreateOrderRequest{
		ToName:       order.OrderShippingName,
		ToPhone:      order.OrderShippingPhone,
		ToAddress:    order.OrderShippingAddress,
		ToDistrictID: msg.Input.DistrictID,
		ToWardCode:   msg.Input.WardCode,
		Weight:       msg.Input.Weight,
		Length:       msg.Input.Length,
		Width:        msg.Input.Width,
		Height:       msg.Input.Height,
		Items:        shippingItems,
		Note:         msg.Input.Note,
		RequiredNote: msg.Input.RequiredNote,
		CODAmount:    codAmount,
		CallbackURL:  utils.GetEnv("GHN_CALLBACK_URL", ""),
	}

	ghnResp, err := provider.CreateShippingOrder(ctx, ghnReq)
	if err != nil {
		logger.Log.Error().Err(err).Str("order_uuid", msg.OrderUUID).Msg("GHN API creation failed")
		failNote := fmt.Sprintf("GHN shipping creation failed: %s", err.Error())
		_, _ = s.trackingRepo.InsertEvent(ctx, nil, sqlc.InsertTrackingEventParams{
			EventOrderID: order.OrderID,
			EventStatus:  "shipping_failed",
			EventNote:    &failNote,
			EventSource:  "system",
		})
		return err
	}

	tx, err := db.BeginTx(ctx)
	if err != nil {
		_ = provider.CancelShippingOrder(ctx, ghnResp.ShippingCode)
		return err
	}
	defer tx.Rollback(ctx)
	var expectedDelivery pgtype.Timestamptz
	if ghnResp.ExpectedDelivery != nil {
		expectedDelivery = pgtype.Timestamptz{
			Time:  *ghnResp.ExpectedDelivery,
			Valid: true,
		}
	}
	_, err = s.orderRepo.UpdateShippingAndStatus(ctx, tx, orderUUID, sqlc.UpdateOrderShippingAndStatusParams{
		ShippingProvider: &msg.Input.Provider,
		ShippingCode:     &ghnResp.ShippingCode,
		ShippingFee:      utils.FloatToNumeric(ghnResp.ShippingFee),
		ExpectedDelivery: expectedDelivery,
		OrderStatus:      OrderStatusShipped,
		OrderUuid:        orderUUID,
	})
	if err != nil {
		_ = provider.CancelShippingOrder(ctx, ghnResp.ShippingCode)
		return err
	}
	// Ghi nhận tracking timeline
	eventNote := fmt.Sprintf("Created shipping order via %s successfully. Tracking code: %s, Fee: %.0f VND",
		strings.ToUpper(msg.Input.Provider), ghnResp.ShippingCode, ghnResp.ShippingFee)
	_, err = s.trackingRepo.InsertEvent(ctx, tx, sqlc.InsertTrackingEventParams{
		EventOrderID: order.OrderID,
		EventStatus:  "shipped",
		EventNote:    &eventNote,
		EventSource:  "admin",
	})
	if err != nil {
		_ = provider.CancelShippingOrder(ctx, ghnResp.ShippingCode)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		_ = provider.CancelShippingOrder(ctx, ghnResp.ShippingCode)
		return err
	}
	_ = s.cache.Clear(fmt.Sprintf("order:%s", orderUUID.String()))
	logger.Log.Info().Str("order_uuid", msg.OrderUUID).Str("shipping_code", ghnResp.ShippingCode).Msg("Shipping order created successfully via RabbitMQ")

	go func() {
		_ = s.notificationService.Send(context.Background(), v1dto.SendNotificationInput{
			UserID: order.OrderUserID,
			Type:   "order_shipped",
			Title:  "Your Order Is On The Way",
			Body:   fmt.Sprintf("Order #%s has been shipped with tracking code: %s.", order.OrderNumber, ghnResp.ShippingCode),
			Data: map[string]any{
				"order_uuid":    orderUUID.String(),
				"shipping_code": ghnResp.ShippingCode,
			},
		})
	}()

	return nil
}

func (s *orderService) GetOrderTimeline(ctx *gin.Context, orderUUID uuid.UUID) (v1dto.OrderTimelineResponseDTO, error) {
	reqCtx := ctx.Request.Context()

	user, err := s.getUser(ctx)
	if err != nil {
		return v1dto.OrderTimelineResponseDTO{}, err
	}

	order, err := s.orderRepo.GetByUUID(reqCtx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v1dto.OrderTimelineResponseDTO{}, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return v1dto.OrderTimelineResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
	}

	// Check if the user is the owner of the order or has admin/moderator privileges
	isAdminOrMod := user.UserLevel == RoleAdmin || user.UserLevel == RoleModerator
	if order.OrderUserID != user.UserID && !isAdminOrMod {
		return v1dto.OrderTimelineResponseDTO{}, utils.NewError(utils.ErrCodeForbidden, "Access denied")
	}

	events, err := s.trackingRepo.GetByOrderID(reqCtx, order.OrderID)
	if err != nil {
		return v1dto.OrderTimelineResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get tracking events", err)
	}
	return v1dto.OrderTimelineResponseDTO{
		OrderUUID:   order.OrderUuid.String(),
		OrderNumber: order.OrderNumber,
		Events:      v1dto.MapToEventDTOs(events),
	}, nil
}

func (s *orderService) HandleShippingWebhook(ctx *gin.Context, rawBody []byte, payload v1dto.GHNWebhookPayload) error {
	reqCtx := ctx.Request.Context()

	ghnStatus := strings.ToLower(strings.TrimSpace(payload.Status))

	// Tạo externalEventID để dedup qua bảng webhook_events
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

	event, err := s.webhookRepo.InsertEvent(reqCtx, nil, eventParams)
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

	order, err := s.orderRepo.GetByShippingCode(reqCtx, payload.OrderCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Log.Warn().Str("shipping_code", payload.OrderCode).Msg("Received webhook for unknown shipping code")
			_ = s.webhookRepo.MarkProcessed(reqCtx, nil, event.WeID)
			return nil
		}
		return utils.WrapError(utils.ErrCodeInternal, "Failed to get order by shipping code", err)
	}

	// Xử lý nghiệp vụ theo trạng thái từ GHN
	switch ghnStatus {
	case "delivered":
		if order.OrderStatus == OrderStatusDelivered {
			_ = s.webhookRepo.MarkProcessed(reqCtx, nil, event.WeID)
			return nil
		}

		tx, err := db.BeginTx(reqCtx)
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to begin transaction", err)
		}
		defer tx.Rollback(reqCtx)

		updatedOrder, err := s.orderRepo.MarkDeliveredByShippingCode(reqCtx, tx, payload.OrderCode)
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to update order status to delivered", err)
		}

		eventNote := "Order delivered successfully via shipping provider."
		_, err = s.trackingRepo.InsertEvent(reqCtx, tx, sqlc.InsertTrackingEventParams{
			EventOrderID: order.OrderID,
			EventStatus:  "delivered",
			EventNote:    &eventNote,
			EventSource:  "ghn",
		})
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to record tracking event", err)
		}

		if err := tx.Commit(reqCtx); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
		}

		// Xóa cache đơn hàng
		_ = s.cache.Clear(fmt.Sprintf("order:%s", updatedOrder.OrderUuid.String()))
		logger.Log.Info().Str("order_number", order.OrderNumber).Str("code", payload.OrderCode).Msg("Order marked as DELIVERED via GHN Webhook")

		go func() {
			_ = s.notificationService.Send(context.Background(), v1dto.SendNotificationInput{
				UserID: updatedOrder.OrderUserID,
				Type:   "order_delivered",
				Title:  "Order Delivered",
				Body:   fmt.Sprintf("Your order #%s has been delivered. Please confirm receipt.", order.OrderNumber),
				Data:   map[string]any{"order_uuid": updatedOrder.OrderUuid.String()},
			})
		}()

	case "delivering":
		eventNote := "Order is currently being delivered by shipping provider."
		if payload.Warehouse != "" {
			eventNote = fmt.Sprintf("Order is being shipped from %s", payload.Warehouse)
		}
		_, _ = s.trackingRepo.InsertEvent(reqCtx, nil, sqlc.InsertTrackingEventParams{
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

		_, _ = s.trackingRepo.InsertEvent(reqCtx, nil, sqlc.InsertTrackingEventParams{
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

		_, _ = s.trackingRepo.InsertEvent(reqCtx, nil, sqlc.InsertTrackingEventParams{
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
		_, _ = s.trackingRepo.InsertEvent(reqCtx, nil, sqlc.InsertTrackingEventParams{
			EventOrderID: order.OrderID,
			EventStatus:  ghnStatus,
			EventNote:    &eventNote,
			EventSource:  "ghn",
		})
	}

	// 4. Đánh dấu webhook event đã xử lý thành công
	if err := s.webhookRepo.MarkProcessed(reqCtx, nil, event.WeID); err != nil {
		logger.Log.Error().
			Err(err).
			Int32("webhook_event_id", event.WeID).
			Msg("Failed to mark webhook event as processed")
	}

	return nil
}

func (s *orderService) CalculateShippingFee(ctx *gin.Context, input v1dto.CalculateShippingFeeInput) (v1dto.ShippingFeeResponseDTO, error) {
	reqCtx := ctx.Request.Context()

	provider, err := s.shippingFactory.GetProvider(input.Provider)
	if err != nil {
		return v1dto.ShippingFeeResponseDTO{}, utils.NewError(
			utils.ErrCodeBadRequest,
			fmt.Sprintf("Unsupported shipping provider: %s", input.Provider),
		)
	}

	req := shipping.CreateOrderRequest{
		ToDistrictID: input.DistrictID,
		ToWardCode:   input.WardCode,
		Weight:       input.Weight,
		Length:       input.Length,
		Width:        input.Width,
		Height:       input.Height,
		CODAmount:    input.InsuranceValue,
	}

	fee, err := provider.CalculateFee(reqCtx, req)
	if err != nil {
		logger.Log.Error().
			Err(err).
			Str("provider", input.Provider).
			Int("district_id", input.DistrictID).
			Str("ward_code", input.WardCode).
			Msg("Failed to calculate shipping fee from provider")
		return v1dto.ShippingFeeResponseDTO{}, utils.WrapError(
			utils.ErrCodeInternal,
			"Failed to calculate shipping fee from shipping provider",
			err,
		)
	}

	return v1dto.ShippingFeeResponseDTO{
		Provider:    input.Provider,
		ShippingFee: fee,
	}, nil
}

func (s *orderService) CancelShipping(ctx *gin.Context, orderUUID uuid.UUID, input v1dto.CancelShippingInput) (v1dto.CancelShippingResponseDTO, error) {
	reqCtx := ctx.Request.Context()

	order, err := s.orderRepo.GetByUUID(reqCtx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v1dto.CancelShippingResponseDTO{}, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return v1dto.CancelShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
	}

	if order.OrderStatus != OrderStatusShipped {
		return v1dto.CancelShippingResponseDTO{}, utils.NewError(
			utils.ErrCodeBadRequest,
			fmt.Sprintf("Cannot cancel shipping for order with status %d. Order must be in Shipped status.", order.OrderStatus),
		)
	}

	if order.OrderShippingCode == nil || strings.TrimSpace(*order.OrderShippingCode) == "" {
		return v1dto.CancelShippingResponseDTO{}, utils.NewError(
			utils.ErrCodeBadRequest,
			"Order does not have a valid shipping code",
		)
	}

	shippingCode := *order.OrderShippingCode
	providerName := "ghn"
	if order.OrderShippingProvider != nil && strings.TrimSpace(*order.OrderShippingProvider) != "" {
		providerName = *order.OrderShippingProvider
	}

	provider, err := s.shippingFactory.GetProvider(providerName)
	if err != nil {
		return v1dto.CancelShippingResponseDTO{}, utils.NewError(
			utils.ErrCodeBadRequest,
			fmt.Sprintf("Unsupported shipping provider: %s", providerName),
		)
	}

	if err := provider.CancelShippingOrder(reqCtx, shippingCode); err != nil {
		logger.Log.Error().
			Err(err).
			Str("provider", providerName).
			Str("shipping_code", shippingCode).
			Msg("Failed to cancel shipping order with provider")
		return v1dto.CancelShippingResponseDTO{}, utils.WrapError(
			utils.ErrCodeInternal,
			"Failed to cancel shipping order with provider",
			err,
		)
	}

	txCtx, cancel := context.WithTimeout(reqCtx, 10*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return v1dto.CancelShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to begin transaction", err)
	}
	defer tx.Rollback(txCtx)

	updatedOrder, err := s.orderRepo.CancelShipping(txCtx, tx, orderUUID)
	if err != nil {
		return v1dto.CancelShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update order status after shipping cancellation", err)
	}

	eventNote := fmt.Sprintf("Cancelled waybill %s (%s) successfully.", strings.ToUpper(providerName), shippingCode)
	if input.Reason != "" {
		eventNote += fmt.Sprintf(" Reason: %s", input.Reason)
	}

	_, err = s.trackingRepo.InsertEvent(reqCtx, tx, sqlc.InsertTrackingEventParams{
		EventOrderID: order.OrderID,
		EventStatus:  "shipping_cancelled",
		EventNote:    &eventNote,
		EventSource:  "admin",
	})
	if err != nil {
		return v1dto.CancelShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to record tracking event", err)
	}
	if err := tx.Commit(reqCtx); err != nil {
		return v1dto.CancelShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	_ = s.cache.Clear(fmt.Sprintf("order:%s", orderUUID.String()))

	logger.Log.Info().
		Str("order_number", order.OrderNumber).
		Str("shipping_code", shippingCode).
		Msg("Order shipping cancelled successfully")

	return v1dto.CancelShippingResponseDTO{
		OrderUUID:    orderUUID.String(),
		OrderNumber:  updatedOrder.OrderNumber,
		ShippingCode: shippingCode,
		OrderStatus:  "Confirmed",
		Message:      "Shipping waybill cancelled successfully. Order status reverted to Confirmed.",
	}, nil
}

func (s *orderService) ConfirmOrderDelivery(ctx *gin.Context, orderUUID uuid.UUID) (v1dto.ConfirmDeliveryResponseDTO, error) {
	reqCtx := ctx.Request.Context()

	user, err := s.getUser(ctx)
	if err != nil {
		return v1dto.ConfirmDeliveryResponseDTO{}, err
	}

	order, err := s.orderRepo.GetByUUID(reqCtx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v1dto.ConfirmDeliveryResponseDTO{}, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return v1dto.ConfirmDeliveryResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
	}

	isAdminOrMod := user.UserLevel == RoleAdmin || user.UserLevel == RoleModerator
	if order.OrderUserID != user.UserID && !isAdminOrMod {
		return v1dto.ConfirmDeliveryResponseDTO{}, utils.NewError(utils.ErrCodeForbidden, "Access denied")
	}

	if order.OrderStatus == OrderStatusDelivered {
		return v1dto.ConfirmDeliveryResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Order is already marked as delivered")
	}

	if order.OrderStatus != OrderStatusShipped {
		var errMsg string
		switch order.OrderStatus {
		case OrderStatusPending:
			errMsg = "Order is pending and has not been processed yet"
		case OrderStatusConfirmed:
			errMsg = "Order is confirmed but has not been shipped yet. Delivery can only be confirmed once shipped"
		case OrderStatusCancelled:
			errMsg = "Cannot confirm delivery for a cancelled order"
		case OrderStatusReturned:
			errMsg = "Cannot confirm delivery for a returned order"
		default:
			errMsg = fmt.Sprintf("Cannot confirm delivery for an order with status: %s", v1dto.MapOrderStatusToString(order.OrderStatus))
		}
		return v1dto.ConfirmDeliveryResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest, errMsg)
	}

	txCtx, cancel := context.WithTimeout(reqCtx, 10*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return v1dto.ConfirmDeliveryResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to begin transaction", err)
	}
	defer tx.Rollback(txCtx)

	updatedOrder, err := s.orderRepo.MarkDeliveredByUUID(txCtx, tx, orderUUID)
	if err != nil {
		return v1dto.ConfirmDeliveryResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update order to delivered", err)
	}

	eventNote := "Customer confirmed order receipt."
	_, err = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
		EventOrderID: order.OrderID,
		EventStatus:  "delivered",
		EventNote:    &eventNote,
		EventSource:  "user",
	})
	if err != nil {
		return v1dto.ConfirmDeliveryResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to record tracking event", err)
	}

	if err := tx.Commit(txCtx); err != nil {
		return v1dto.ConfirmDeliveryResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	_ = s.cache.Clear(fmt.Sprintf("order:%s", orderUUID.String()))
	_ = s.cache.Clear("orders:*")

	go func() {
		_ = s.notificationService.Send(context.Background(), v1dto.SendNotificationInput{
			UserID: updatedOrder.OrderUserID,
			Type:   "order_completed",
			Title:  "Order Completed",
			Body:   fmt.Sprintf("Thank you! Order #%s is completed. You can now write a review.", updatedOrder.OrderNumber),
			Data:   map[string]any{"order_uuid": updatedOrder.OrderUuid.String()},
		})
	}()

	var deliveredAt *time.Time
	if updatedOrder.OrderDeliveredAt.Valid {
		deliveredAt = &updatedOrder.OrderDeliveredAt.Time
	}

	return v1dto.ConfirmDeliveryResponseDTO{
		OrderUUID:         updatedOrder.OrderUuid.String(),
		OrderNumber:       updatedOrder.OrderNumber,
		OrderStatus:       updatedOrder.OrderStatus,
		StatusName:        v1dto.MapOrderStatusToString(updatedOrder.OrderStatus),
		StatusText:        v1dto.MapOrderStatusToVietnamese(updatedOrder.OrderStatus),
		PaymentMethod:     v1dto.MapPaymentMethodToString(updatedOrder.OrderPaymentMethod),
		PaymentStatus:     updatedOrder.OrderPaymentStatus,
		PaymentStatusName: v1dto.MapPaymentStatusToString(updatedOrder.OrderPaymentStatus),
		DeliveredAt:       deliveredAt,
		Message:           "Order delivery confirmed successfully. You can now leave a product review or request a return within 7 days.",
	}, nil
}

func (s *orderService) GetPrintBill(ctx *gin.Context, orderUUID uuid.UUID) (v1dto.PrintBillResponseDTO, error) {
	reqCtx := ctx.Request.Context()

	order, err := s.orderRepo.GetByUUID(reqCtx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v1dto.PrintBillResponseDTO{}, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return v1dto.PrintBillResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
	}

	if order.OrderShippingCode == nil || strings.TrimSpace(*order.OrderShippingCode) == "" {
		return v1dto.PrintBillResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Order does not have a valid shipping code")
	}

	shippingCode := *order.OrderShippingCode
	providerName := "ghn"
	if order.OrderShippingProvider != nil && strings.TrimSpace(*order.OrderShippingProvider) != "" {
		providerName = *order.OrderShippingProvider
	}

	provider, err := s.shippingFactory.GetProvider(providerName)
	if err != nil {
		return v1dto.PrintBillResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get shipping provider", err)
	}

	res, err := provider.GetPrintBillURL(reqCtx, shippingCode)
	if err != nil {
		logger.Log.Error().
			Err(err).
			Str("provider", providerName).
			Str("shipping_code", shippingCode).
			Msg("Failed to get print bill URL from provider")
		return v1dto.PrintBillResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get print bill URL from provider", err)
	}

	return v1dto.PrintBillResponseDTO{
		OrderUUID:    orderUUID.String(),
		OrderNumber:  order.OrderNumber,
		ShippingCode: shippingCode,
		Token:        res.Token,
		PrintA5:      res.PrintA5,
		Print80x80:   res.Print80,
		Print52x70:   res.Print52,
	}, nil
}

func (s *orderService) GetProvinces(ctx *gin.Context) ([]v1dto.ProvinceDTO, error) {
	reqCtx := ctx.Request.Context()

	cacheKey := "shipping:ghn:provinces"
	var cached []v1dto.ProvinceDTO
	if err := s.cache.Get(cacheKey, &cached); err == nil && len(cached) > 0 {
		return cached, nil
	}

	provider, err := s.shippingFactory.GetProvider("ghn")
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeInternal, "GHN provider not configured")
	}

	ghnAdapter, ok := provider.(*shipping.GHNAdapter)
	if !ok {
		return nil, utils.NewError(utils.ErrCodeInternal, "Invalid GHN provider adapter")
	}

	provinces, err := ghnAdapter.GetProvinces(reqCtx)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to get provinces from GHN", err)
	}

	_ = s.cache.Set(cacheKey, provinces, 24*time.Hour)

	return provinces, nil
}

func (s *orderService) GetDistricts(ctx *gin.Context, provinceID int) ([]v1dto.DistrictDTO, error) {
	reqCtx := ctx.Request.Context()

	cacheKey := fmt.Sprintf("shipping:ghn:districts:%d", provinceID)
	var cached []v1dto.DistrictDTO
	if err := s.cache.Get(cacheKey, &cached); err == nil && len(cached) > 0 {
		return cached, nil
	}

	provider, err := s.shippingFactory.GetProvider("ghn")
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeInternal, "GHN provider not configured")
	}

	ghnAdapter, ok := provider.(*shipping.GHNAdapter)
	if !ok {
		return nil, utils.NewError(utils.ErrCodeInternal, "Invalid GHN provider adapter")
	}

	districts, err := ghnAdapter.GetDistricts(reqCtx, provinceID)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to fetch districts from GHN", err)
	}
	_ = s.cache.Set(cacheKey, districts, addressCacheTTL)

	return districts, nil
}

func (s *orderService) GetWards(ctx *gin.Context, districtID int) ([]v1dto.WardDTO, error) {
	reqCtx := ctx.Request.Context()

	cacheKey := fmt.Sprintf("shipping:ghn:wards:%d", districtID)
	var cached []v1dto.WardDTO
	if err := s.cache.Get(cacheKey, &cached); err == nil && len(cached) > 0 {
		return cached, nil
	}

	provider, err := s.shippingFactory.GetProvider("ghn")
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeInternal, "GHN provider not configured")
	}

	ghnAdapter, ok := provider.(*shipping.GHNAdapter)
	if !ok {
		return nil, utils.NewError(utils.ErrCodeInternal, "Invalid GHN provider adapter")
	}

	wards, err := ghnAdapter.GetWards(reqCtx, districtID)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to fetch wards from GHN", err)
	}
	_ = s.cache.Set(cacheKey, wards, addressCacheTTL)

	return wards, nil
}

func (s *orderService) SyncOrderTracking(ctx *gin.Context, orderUUID uuid.UUID) (v1dto.SyncTrackingResponseDTO, error) {
	reqCtx := ctx.Request.Context()

	user, err := s.getUser(ctx)
	if err != nil {
		return v1dto.SyncTrackingResponseDTO{}, err
	}

	order, err := s.orderRepo.GetByUUID(reqCtx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v1dto.SyncTrackingResponseDTO{}, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return v1dto.SyncTrackingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
	}

	isAdminOrMod := user.UserLevel == RoleAdmin || user.UserLevel == RoleModerator
	if order.OrderUserID != user.UserID && !isAdminOrMod {
		return v1dto.SyncTrackingResponseDTO{}, utils.NewError(utils.ErrCodeForbidden, "Access denied")
	}

	if order.OrderShippingCode == nil || strings.TrimSpace(*order.OrderShippingCode) == "" {
		return v1dto.SyncTrackingResponseDTO{}, utils.NewError(
			utils.ErrCodeBadRequest,
			"Order does not have a shipping tracking code to synchronize",
		)
	}
	shippingCode := strings.TrimSpace(*order.OrderShippingCode)

	providerName := "ghn"
	if order.OrderShippingProvider != nil && strings.TrimSpace(*order.OrderShippingProvider) != "" {
		providerName = strings.TrimSpace(*order.OrderShippingProvider)
	}

	provider, err := s.shippingFactory.GetProvider(providerName)
	if err != nil {
		return v1dto.SyncTrackingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Unsupported shipping provider", err)
	}

	// Call shipping provider API to get tracking info
	trackingInfo, err := provider.GetTrackingInfo(reqCtx, shippingCode)
	if err != nil {
		logger.Log.Error().
			Err(err).
			Str("order_number", order.OrderNumber).
			Str("shipping_code", shippingCode).
			Msg("Failed to fetch tracking info from shipping provider")
		return v1dto.SyncTrackingResponseDTO{}, utils.WrapError(
			utils.ErrCodeBadGateway,
			"Failed to query tracking information from GHN gateway",
			err,
		)
	}

	ghnStatus := strings.ToLower(strings.TrimSpace(trackingInfo.Status))

	isDeliveredNow := false
	if ghnStatus == "delivered" && order.OrderStatus != OrderStatusDelivered {
		txCtx, cancel := context.WithTimeout(reqCtx, 10*time.Second)
		defer cancel()

		tx, err := db.BeginTx(txCtx)
		if err == nil {
			defer tx.Rollback(txCtx)

			_, err = s.orderRepo.MarkDeliveredByShippingCode(txCtx, tx, shippingCode)
			if err == nil {
				eventNote := fmt.Sprintf("GHN Sync: %s", trackingInfo.StatusText)
				_, _ = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
					EventOrderID: order.OrderID,
					EventStatus:  "delivered",
					EventNote:    &eventNote,
					EventSource:  "ghn_sync",
				})

				if err := tx.Commit(txCtx); err == nil {
					isDeliveredNow = true
					_ = s.cache.Clear(fmt.Sprintf("order:%s", orderUUID.String()))
					_ = s.cache.Clear("orders:*")

					// Realtime notify
					go func() {
						_ = s.notificationService.Send(context.Background(), v1dto.SendNotificationInput{
							UserID: order.OrderUserID,
							Type:   "order_delivered",
							Title:  "Order Delivered Successfully",
							Body:   fmt.Sprintf("Your order #%s has been delivered by GHN.", order.OrderNumber),
							Data: map[string]any{
								"order_uuid":    orderUUID.String(),
								"shipping_code": shippingCode,
							},
						})
					}()
				}
			}
		}
	} else {
		// Write log if this is has a new status
		existingEvents, _ := s.trackingRepo.GetByOrderID(reqCtx, order.OrderID)
		hasStatusAlready := false
		for _, ev := range existingEvents {
			if strings.EqualFold(ev.EventStatus, ghnStatus) {
				hasStatusAlready = true
				break
			}
		}

		if !hasStatusAlready && ghnStatus != "" {
			eventNote := fmt.Sprintf("GHN tracking update: %s", trackingInfo.StatusText)
			_, _ = s.trackingRepo.InsertEvent(reqCtx, nil, sqlc.InsertTrackingEventParams{
				EventOrderID: order.OrderID,
				EventStatus:  ghnStatus,
				EventNote:    &eventNote,
				EventSource:  "ghn_sync",
			})
		}
	}

	// Get all latest timeline
	updatedEvents, _ := s.trackingRepo.GetByOrderID(reqCtx, order.OrderID)

	return v1dto.SyncTrackingResponseDTO{
		OrderUUID:        order.OrderUuid.String(),
		OrderNumber:      order.OrderNumber,
		ShippingCode:     shippingCode,
		ShippingProvider: strings.ToUpper(providerName),
		CurrentStatus:    ghnStatus,
		StatusText:       trackingInfo.StatusText,
		IsDelivered:      order.OrderStatus == OrderStatusDelivered || isDeliveredNow,
		LastSyncAt:       time.Now().UTC(),
		Timeline:         v1dto.MapToEventDTOs(updatedEvents),
		Message:          "Order tracking information synchronized successfully from GHN.",
	}, nil
}

func (s *orderService) AdminSyncActiveShippingOrders(ctx *gin.Context) (v1dto.SyncActiveShippingStatsDTO, error) {
	reqCtx := ctx.Request.Context()

	user, err := s.getUser(ctx)
	if err != nil {
		return v1dto.SyncActiveShippingStatsDTO{}, err
	}

	if user.UserLevel != RoleAdmin && user.UserLevel != RoleModerator {
		return v1dto.SyncActiveShippingStatsDTO{}, utils.NewError(utils.ErrCodeForbidden, "Access denied")
	}

	return s.SyncActiveShippingOrders(reqCtx)
}

func (s *orderService) SyncActiveShippingOrders(ctx context.Context) (v1dto.SyncActiveShippingStatsDTO, error) {
	const batchLimit int32 = 50
	orders, err := s.orderRepo.GetActiveShippedOrders(ctx, batchLimit)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to query active shipped orders for sync cronjob")
		return v1dto.SyncActiveShippingStatsDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to query active shipped orders", err)
	}

	stats := v1dto.SyncActiveShippingStatsDTO{
		ScannedCount: len(orders),
		SyncedAt:     time.Now().UTC(),
	}

	if len(orders) == 0 {
		logger.Log.Debug().Msg("No active shipped orders found for cronjob synchronization")
		return stats, nil
	}

	logger.Log.Info().
		Int("orders_to_scan", len(orders)).
		Msg("Starting background shipping sync cronjob batch")

	for _, order := range orders {
		select {
		case <-ctx.Done():
			logger.Log.Warn().Msg("Shipping cronjob synchronization aborted due to context cancellation")
			return stats, ctx.Err()
		default:
		}

		if order.OrderShippingCode == nil || strings.TrimSpace(*order.OrderShippingCode) == "" {
			continue
		}
		shippingCode := strings.TrimSpace(*order.OrderShippingCode)

		providerName := "ghn"
		if order.OrderShippingProvider != nil && strings.TrimSpace(*order.OrderShippingProvider) != "" {
			providerName = strings.TrimSpace(*order.OrderShippingProvider)
		}

		provider, err := s.shippingFactory.GetProvider(providerName)
		if err != nil {
			logger.Log.Warn().
				Str("provider", providerName).
				Str("order_number", order.OrderNumber).
				Msg("Shipping provider not supported in sync cronjob")
			stats.FailedCount++
			continue
		}

		trackingInfo, err := provider.GetTrackingInfo(ctx, shippingCode)
		if err != nil {
			logger.Log.Warn().
				Err(err).
				Str("order_number", order.OrderNumber).
				Str("shipping_code", shippingCode).
				Msg("Failed to query tracking info in sync cronjob")
			stats.FailedCount++
			time.Sleep(200 * time.Millisecond)
			continue
		}

		stats.SyncedCount++
		ghnStatus := strings.ToLower(strings.TrimSpace(trackingInfo.Status))

		if ghnStatus == "delivered" && order.OrderStatus != OrderStatusDelivered {
			txCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			tx, err := db.BeginTx(txCtx)
			if err != nil {
				cancel()
				logger.Log.Error().Err(err).Str("order_number", order.OrderNumber).Msg("Failed to begin transaction for order delivery update")
				stats.FailedCount++
				continue
			}

			updatedOrder, err := s.orderRepo.MarkDeliveredByShippingCode(txCtx, tx, shippingCode)
			if err != nil {
				_ = tx.Rollback(txCtx)
				cancel()
				logger.Log.Error().Err(err).Str("order_number", order.OrderNumber).Msg("Failed to mark order delivered in sync cronjob")
				stats.FailedCount++
				continue
			}

			eventNote := fmt.Sprintf("Cronjob Sync: %s", trackingInfo.StatusText)
			_, err = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
				EventOrderID: order.OrderID,
				EventStatus:  "delivered",
				EventNote:    &eventNote,
				EventSource:  "ghn_cron",
			})
			if err != nil {
				_ = tx.Rollback(txCtx)
				cancel()
				logger.Log.Error().Err(err).Str("order_number", order.OrderNumber).Msg("Failed to record tracking event in sync cronjob")
				stats.FailedCount++
				continue
			}

			if err := tx.Commit(txCtx); err != nil {
				cancel()
				logger.Log.Error().Err(err).Str("order_number", order.OrderNumber).Msg("Failed to commit transaction in sync cronjob")
				stats.FailedCount++
				continue
			}
			cancel()

			stats.DeliveredCount++
			_ = s.cache.Clear(fmt.Sprintf("order:%s", order.OrderUuid.String()))
			_ = s.cache.Clear("orders:*")

			go func(ord sqlc.Order, code string) {
				_ = s.notificationService.Send(context.Background(), v1dto.SendNotificationInput{
					UserID: ord.OrderUserID,
					Type:   "order_delivered",
					Title:  "Order Delivered Successfully",
					Body:   fmt.Sprintf("Your order #%s has been delivered by GHN.", ord.OrderNumber),
					Data: map[string]any{
						"order_uuid":    ord.OrderUuid.String(),
						"shipping_code": code,
					},
				})
			}(updatedOrder, shippingCode)

			logger.Log.Info().
				Str("order_number", order.OrderNumber).
				Str("shipping_code", shippingCode).
				Msg("Order transitioned to DELIVERED via shipping sync cronjob")
		} else if ghnStatus != "" {
			existingEvents, err := s.trackingRepo.GetByOrderID(ctx, order.OrderID)
			if err == nil {
				hasStatusAlready := false
				for _, ev := range existingEvents {
					if strings.EqualFold(ev.EventStatus, ghnStatus) {
						hasStatusAlready = true
						break
					}
				}
				if !hasStatusAlready {
					eventNote := fmt.Sprintf("GHN update: %s", trackingInfo.StatusText)
					_, _ = s.trackingRepo.InsertEvent(ctx, nil, sqlc.InsertTrackingEventParams{
						EventOrderID: order.OrderID,
						EventStatus:  ghnStatus,
						EventNote:    &eventNote,
						EventSource:  "ghn_cron",
					})
				}
			}
		}

		time.Sleep(200 * time.Millisecond)
	}

	logger.Log.Info().
		Int("scanned", stats.ScannedCount).
		Int("synced", stats.SyncedCount).
		Int("delivered", stats.DeliveredCount).
		Int("failed", stats.FailedCount).
		Msg("Shipping sync cronjob batch completed successfully")

	return stats, nil
}
