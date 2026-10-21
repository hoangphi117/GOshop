package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"goshop/internal/db"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/rabbitmq"
	"goshop/services/order/internal/db/sqlc"
	v1dto "goshop/services/order/internal/dto"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type orderLifecycleService struct {
	txManager      db.TxManager
	orderStore     OrderStorePort
	orderItemStore OrderItemStorePort
	trackingStore  OrderTrackingPort
	userReader     OrderCustomerReader
	stockPort      OrderStockPort
	outboxStore    OrderOutboxPort
	cache          cache.RedisCacheService
}

func NewOrderLifecycleService(
	txManager db.TxManager,
	orderStore OrderStorePort,
	orderItemStore OrderItemStorePort,
	trackingStore OrderTrackingPort,
	userReader OrderCustomerReader,
	stockPort OrderStockPort,
	cacheService cache.RedisCacheService,
	outboxStore ...OrderOutboxPort,
) OrderLifecycleService {
	var outboxPort OrderOutboxPort
	if len(outboxStore) > 0 {
		outboxPort = outboxStore[0]
	}
	return &orderLifecycleService{
		txManager:      txManager,
		orderStore:     orderStore,
		orderItemStore: orderItemStore,
		trackingStore:  trackingStore,
		userReader:     userReader,
		stockPort:      stockPort,
		outboxStore:    outboxPort,
		cache:          cacheService,
	}
}

func (s *orderLifecycleService) UpdateOrderStatus(ctx context.Context, orderUUID uuid.UUID, input v1dto.UpdateOrderStatusInput) (*v1dto.OrderDTO, error) {
	existingOrder, err := s.orderStore.GetByUUID(ctx, orderUUID)
	if err != nil {
		return nil, handleOrderNotFoundError(err)
	}

	if !isValidStatusTransition(existingOrder.OrderStatus, input.Status) {
		errMsg := fmt.Sprintf("Cannot change order status from '%s' to '%s'",
			v1dto.MapOrderStatusToString(existingOrder.OrderStatus),
			v1dto.MapOrderStatusToString(input.Status),
		)
		return nil, utils.NewError(utils.ErrCodeBadRequest, errMsg)
	}

	var statusNote string
	switch input.Status {
	case OrderStatusConfirmed:
		statusNote = "Order was confirmed by shop admin."
	case OrderStatusDelivered:
		statusNote = "Order was marked as delivered."
	}

	updateParams := input.MapUpdateStatusInputToModel(orderUUID)
	var updatedOrder sqlc.Order

	executeUpdates := func(txCtx context.Context, tx pgx.Tx) error {
		var updateErr error
		updatedOrder, updateErr = s.orderStore.UpdateStatus(txCtx, tx, updateParams)
		if updateErr != nil {
			if errors.Is(updateErr, sql.ErrNoRows) || errors.Is(updateErr, pgx.ErrNoRows) {
				return utils.NewError(utils.ErrCodeNotFound, "order not found")
			}
			return utils.WrapError(utils.ErrCodeInternal, "failed to update order status", updateErr)
		}

		if statusNote != "" {
			if _, trackErr := s.trackingStore.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
				EventOrderID: updatedOrder.OrderID,
				EventStatus:  v1dto.MapOrderStatusToString(input.Status),
				EventNote:    &statusNote,
				EventSource:  "admin",
			}); trackErr != nil {
				logger.Log.Warn().Err(trackErr).Int32("order_id", updatedOrder.OrderID).Msg("Failed to insert tracking event for order status change")
			}
		}

		if s.outboxStore != nil {
			evt := rabbitmq.NewOrderStatusChangedEvent(
				uuid.New().String(),
				updatedOrder.OrderUuid.String(),
				updatedOrder.OrderNumber,
				existingOrder.UserUuid.String(),
				updatedOrder.OrderID,
				updatedOrder.OrderUserID,
				existingOrder.OrderStatus,
				updatedOrder.OrderStatus,
				"",
			)
			if _, outboxErr := s.outboxStore.Insert(txCtx, tx, "order", updatedOrder.OrderUuid.String(), rabbitmq.QueueOrderStatusChanged, evt); outboxErr != nil {
				return utils.WrapError(utils.ErrCodeInternal, "failed to insert outbox event for order status change", outboxErr)
			}
		}
		return nil
	}

	if s.txManager != nil {
		if err := s.txManager.WithTx(ctx, func(txCtx context.Context) error {
			tx, _ := db.GetTx(txCtx)
			return executeUpdates(txCtx, tx)
		}); err != nil {
			return nil, err
		}
	} else {
		if err := executeUpdates(ctx, nil); err != nil {
			return nil, err
		}
	}

	orderItems, err := s.orderItemStore.GetByOrderID(ctx, updatedOrder.OrderID)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get order items", err)
	}

	if s.cache != nil {
		if err := s.cache.Clear("orders:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear order cache")
		}
	}

	dto := v1dto.MapToOrderDTO(updatedOrder, orderItems)
	return &dto, nil
}

func isValidStatusTransition(currentStatus, newStatus int32) bool {
	switch currentStatus {
	case OrderStatusPending:
		return newStatus == OrderStatusConfirmed
	case OrderStatusConfirmed:
		return newStatus == OrderStatusShipped
	case OrderStatusShipped:
		return newStatus == OrderStatusDelivered || newStatus == OrderStatusReturned
	case OrderStatusDelivered:
		return newStatus == OrderStatusReturned
	default:
		return false
	}
}

func (s *orderLifecycleService) getCustomer(ctx context.Context, userUUID uuid.UUID) (*OrderCustomer, error) {
	if cust, ok := GetOrderCustomerFromContext(ctx); ok && cust.UserUUID == userUUID && cust.UserID > 0 {
		return cust, nil
	}
	if s.userReader == nil {
		return nil, utils.NewError(utils.ErrCodeNotFound, "User not found")
	}
	user, err := s.userReader.GetByUUID(ctx, userUUID)
	if err != nil {
		return nil, handleUserNotFoundError(err)
	}
	return user, nil
}

func (s *orderLifecycleService) CancelOrder(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, reason *string) (*v1dto.OrderSummaryDTO, error) {
	// 1. Lấy thông tin User & Order
	user, err := s.getCustomer(ctx, userUUID)
	if err != nil {
		return nil, err
	}

	orderRow, err := s.orderStore.GetByUUID(ctx, orderUUID)
	if err != nil {
		return nil, handleOrderNotFoundError(err)
	}

	// 2. Validate điều kiện hủy đơn (Domain Rule)
	if err := s.validateOrderCancellation(user, orderRow); err != nil {
		return nil, err
	}

	// 3. Lấy danh sách sản phẩm để chuẩn bị hoàn kho
	orderItems, err := s.orderItemStore.GetByOrderID(ctx, orderRow.OrderID)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get order items", err)
	}

	var cancelledOrder sqlc.Order

	// 4. Thực thi Transaction điều phối các Sub-services (Orchestration)
	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		tx, _ := db.GetTx(txCtx)

		// 4.1. Hủy trạng thái đơn
		updatedOrder, err := s.orderStore.CancelOrder(txCtx, tx, orderUUID)
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "failed to cancel order", err)
		}
		cancelledOrder = updatedOrder

		// 4.2. Hoàn tồn kho qua StockPort
		if err := s.stockPort.RestoreStockWithRef(txCtx, toRestoreItems(orderItems), orderRow.OrderNumber, "order_cancel", &user.UserID, reason); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "failed to restore product stock", err)
		}

		// 4.3. Ghi nhận Tracking Event
		if err := s.recordCancellationTracking(txCtx, tx, user, orderRow, reason); err != nil {
			return err
		}

		// 4.4. Dispatch OrderCancelled event via Outbox
		if s.outboxStore != nil {
			reasonStr := ""
			if reason != nil {
				reasonStr = strings.TrimSpace(*reason)
			}
			evt := rabbitmq.NewOrderCancelledEventWithDetails(
				uuid.New().String(),
				orderRow.OrderUuid.String(),
				orderRow.OrderNumber,
				user.UserUuid.String(),
				orderRow.OrderID,
				orderRow.OrderUserID,
				reasonStr,
				orderRow.OrderCouponID,
				toOrderCancelledItems(orderItems),
			)
			if _, err := s.outboxStore.Insert(txCtx, tx, "order", orderRow.OrderUuid.String(), rabbitmq.QueueOrderCancelled, evt); err != nil {
				return utils.WrapError(utils.ErrCodeInternal, "failed to insert order cancelled outbox event", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// 5. Invalidate caches
	InvalidateOrderCachesWithCache(s.cache, user.UserUuid)

	dto := v1dto.MapToOrderSummaryDTO(cancelledOrder)
	return &dto, nil
}

func (s *orderLifecycleService) CancelExpiredOrder(ctx context.Context, orderUUID uuid.UUID) error {
	orderRow, err := s.orderStore.GetByUUID(ctx, orderUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return handleOrderNotFoundError(err)
	}

	// Only cancel if order is still Pending and Unpaid
	if orderRow.OrderStatus != OrderStatusPending || orderRow.OrderPaymentStatus == OrderPaymentStatusPaid {
		return nil
	}

	orderItems, err := s.orderItemStore.GetByOrderID(ctx, orderRow.OrderID)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "failed to get order items", err)
	}

	cancelReason := "Payment expired after 15 minutes"
	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		tx, _ := db.GetTx(txCtx)

		// 1. Atomically cancel order
		_, err := s.orderStore.CancelOrder(txCtx, tx, orderUUID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return utils.WrapError(utils.ErrCodeInternal, "failed to cancel order", err)
		}

		// 2. Restore inventory if stockPort is available
		if s.stockPort != nil && len(orderItems) > 0 {
			if err := s.stockPort.RestoreStockWithRef(txCtx, toRestoreItems(orderItems), orderRow.OrderNumber, "order_cancel", nil, &cancelReason); err != nil {
				return utils.WrapError(utils.ErrCodeInternal, "failed to restore product stock", err)
			}
		}

		// 3. Record tracking event
		if s.trackingStore != nil {
			note := "Order was automatically cancelled due to payment expiration (15 minutes timeout)."
			_, err = s.trackingStore.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
				EventOrderID: orderRow.OrderID,
				EventStatus:  "cancelled",
				EventNote:    &note,
				EventSource:  "system",
			})
			if err != nil {
				return utils.WrapError(utils.ErrCodeInternal, "failed to insert tracking event", err)
			}
		}

		// 4. Dispatch OrderCancelled event via Outbox
		if s.outboxStore != nil {
			evt := rabbitmq.NewOrderCancelledEventWithDetails(
				uuid.New().String(),
				orderRow.OrderUuid.String(),
				orderRow.OrderNumber,
				orderRow.UserUuid.String(),
				orderRow.OrderID,
				orderRow.OrderUserID,
				cancelReason,
				orderRow.OrderCouponID,
				toOrderCancelledItems(orderItems),
			)
			if _, err := s.outboxStore.Insert(txCtx, tx, "order", orderRow.OrderUuid.String(), rabbitmq.QueueOrderCancelled, evt); err != nil {
				return utils.WrapError(utils.ErrCodeInternal, "failed to insert order cancelled outbox event", err)
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	// 4. Invalidate caches
	InvalidateOrderCachesWithCache(s.cache, orderRow.UserUuid)

	return nil
}

func (s *orderLifecycleService) validateOrderCancellation(user *OrderCustomer, order sqlc.GetOrderByUUIDRow) error {
	isAdminOrMod := user.UserLevel == RoleAdmin || user.UserLevel == RoleModerator
	if order.OrderUserID != user.UserID && !isAdminOrMod {
		return utils.NewError(utils.ErrCodeForbidden, "You are not authorized to cancel this order")
	}

	if order.OrderStatus != OrderStatusPending && order.OrderStatus != OrderStatusConfirmed {
		return utils.NewError(utils.ErrCodeBadRequest, "Only pending or confirmed orders can be cancelled")
	}

	return nil
}

func toRestoreItems(orderItems []sqlc.GetOrderItemsByOrderIDRow) []RestoreItem {
	items := make([]RestoreItem, len(orderItems))
	for i, item := range orderItems {
		items[i] = RestoreItem{
			ProductID: item.OrderItemProductID,
			Quantity:  item.OrderItemQuantity,
		}
	}
	return items
}

func toOrderCancelledItems(orderItems []sqlc.GetOrderItemsByOrderIDRow) []rabbitmq.OrderCancelledItem {
	items := make([]rabbitmq.OrderCancelledItem, len(orderItems))
	for i, item := range orderItems {
		items[i] = rabbitmq.OrderCancelledItem{
			ProductID:   item.OrderItemProductID,
			ProductUUID: item.OrderItemProductUuid.String(),
			Quantity:    item.OrderItemQuantity,
		}
	}
	return items
}

func (s *orderLifecycleService) recordCancellationTracking(ctx context.Context, tx pgx.Tx, user *OrderCustomer, order sqlc.GetOrderByUUIDRow, reason *string) error {
	note := "Order cancelled."
	if reason != nil && strings.TrimSpace(*reason) != "" {
		note = fmt.Sprintf("Order cancelled. Reason: %s", strings.TrimSpace(*reason))
	}

	eventSource := "user"
	if (user.UserLevel == RoleAdmin || user.UserLevel == RoleModerator) && order.OrderUserID != user.UserID {
		eventSource = "admin"
	}

	_, err := s.trackingStore.InsertEvent(ctx, tx, sqlc.InsertTrackingEventParams{
		EventOrderID: order.OrderID,
		EventStatus:  "cancelled",
		EventNote:    &note,
		EventSource:  eventSource,
	})
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "failed to insert tracking event", err)
	}
	return nil
}

func (s *orderLifecycleService) UpdateOrderShippingInfo(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateOrderShippingInfoInput) (*v1dto.OrderDTO, error) {
	user, err := s.getCustomer(ctx, userUUID)
	if err != nil {
		return nil, err
	}

	existingOrder, err := s.orderStore.GetByUUID(ctx, orderUUID)
	if err != nil {
		return nil, handleOrderNotFoundError(err)
	}

	isAdmin := user.UserLevel == RoleAdmin
	isModerator := user.UserLevel == RoleModerator

	if existingOrder.OrderUserID != user.UserID && !isAdmin && !isModerator {
		return nil, utils.NewError(utils.ErrCodeForbidden, "You do not have permission to update this order")
	}

	if isAdmin || isModerator {
		if existingOrder.OrderStatus != OrderStatusPending && existingOrder.OrderStatus != OrderStatusConfirmed {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "Admin or moderator can only update shipping info for Pending or Confirmed orders")
		}
	} else {
		if existingOrder.OrderStatus != OrderStatusPending {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "You can only update shipping info for Pending orders")
		}
	}

	allowedStatuses := []int32{OrderStatusPending}
	if isAdmin || isModerator {
		allowedStatuses = append(allowedStatuses, OrderStatusConfirmed)
	}

	isStatusValid := slices.Contains(allowedStatuses, existingOrder.OrderStatus)
	if !isStatusValid {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "Order status does not allow shipping info update")
	}

	updateParams := input.MapUpdateShippingInputToModel(orderUUID)
	updateParams.AllowedStatuses = allowedStatuses

	updatedOrder, err := s.orderStore.UpdateOrderShippingInfo(ctx, updateParams)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeConflict, "Order status has changed or order not found, please refresh and try again")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to update shipping info", err)
	}

	orderItems, err := s.orderItemStore.GetByOrderID(ctx, updatedOrder.OrderID)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to get order items", err)
	}

	InvalidateOrderCachesWithCache(s.cache, existingOrder.UserUuid)
	dto := v1dto.MapToOrderDTO(updatedOrder, orderItems)
	return &dto, nil
}

func (s *orderLifecycleService) UpdateOrderPaymentStatus(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateOrderPaymentStatusInput) (*v1dto.OrderDTO, error) {
	existingOrder, err := s.orderStore.GetByUUID(ctx, orderUUID)
	if err != nil {
		return nil, handleOrderNotFoundError(err)
	}

	var allowedFromStatuses []int32
	var targetPaymentStatus int32 = input.PaymentStatus
	newTotalRefunded := utils.NumericToFloat(existingOrder.OrderRefundedAmount)

	switch input.PaymentStatus {
	case OrderPaymentStatusPaid:
		if existingOrder.OrderPaymentStatus != OrderPaymentStatusUnpaid {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "Only Unpaid orders can be marked as Paid")
		}
		if existingOrder.OrderStatus == OrderStatusCancelled || existingOrder.OrderStatus == OrderStatusReturned {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "Cannot mark a cancelled or returned order as Paid")
		}
		allowedFromStatuses = []int32{OrderPaymentStatusUnpaid}

	case OrderPaymentStatusPartiallyRefunded, OrderPaymentStatusFullyRefunded:
		if existingOrder.OrderPaymentStatus != OrderPaymentStatusPaid && existingOrder.OrderPaymentStatus != OrderPaymentStatusPartiallyRefunded {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "Only Paid or Partially Refunded orders can be refunded")
		}
		if existingOrder.OrderStatus != OrderStatusCancelled && existingOrder.OrderStatus != OrderStatusReturned {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "Can only refund orders that are Cancelled (5) or Returned (6)")
		}

		totalAmount := utils.NumericToFloat(existingOrder.OrderTotalAmount)
		currentRefunded := utils.NumericToFloat(existingOrder.OrderRefundedAmount)

		refundAmountVal := totalAmount - currentRefunded
		if input.RefundAmount != nil && *input.RefundAmount > 0 {
			refundAmountVal = *input.RefundAmount
		}

		newTotalRefunded = utils.RoundFloat(currentRefunded+refundAmountVal, 2)
		if newTotalRefunded > totalAmount {
			return nil, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Total refunded amount (%.2f) cannot exceed order total (%.2f)", newTotalRefunded, totalAmount))
		}

		if newTotalRefunded >= totalAmount {
			targetPaymentStatus = OrderPaymentStatusFullyRefunded
		} else {
			targetPaymentStatus = OrderPaymentStatusPartiallyRefunded
		}

		allowedFromStatuses = []int32{OrderPaymentStatusPaid, OrderPaymentStatusPartiallyRefunded}

	default:
		return nil, utils.NewError(utils.ErrCodeBadRequest, "Invalid target payment status")
	}

	updateParams := sqlc.UpdateOrderPaymentStatusParams{
		OrderUuid:           orderUUID,
		PaymentStatus:       targetPaymentStatus,
		RefundedAmount:      utils.FloatToNumeric(newTotalRefunded),
		AllowedFromStatuses: allowedFromStatuses,
	}

	updatedOrder, err := s.orderStore.UpdateOrderPaymentStatus(ctx, nil, updateParams)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeConflict, "Payment status has changed by another request, please refresh")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to update payment status", err)
	}

	trxRef := ""
	if input.TransactionRef != nil {
		trxRef = *input.TransactionRef
	}

	if s.outboxStore != nil && targetPaymentStatus == OrderPaymentStatusPaid {
		event := rabbitmq.NewPaymentSucceededEvent(
			uuid.New().String(),
			"",
			updatedOrder.OrderUuid.String(),
			updatedOrder.OrderNumber,
			existingOrder.UserUuid.String(),
			"admin",
			trxRef,
			utils.NumericToFloat(updatedOrder.OrderTotalAmount),
		)
		if _, outboxErr := s.outboxStore.Insert(
			ctx,
			nil,
			"order",
			updatedOrder.OrderUuid.String(),
			rabbitmq.QueuePaymentSucceeded,
			event,
		); outboxErr != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to publish payment status update", outboxErr)
		}
	}

	orderItems, err := s.orderItemStore.GetByOrderID(ctx, updatedOrder.OrderID)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to get order items", err)
	}

	noteStr := ""
	if input.Note != nil {
		noteStr = *input.Note
	}

	logger.Log.Info().
		Str("order_uuid", orderUUID.String()).
		Int32("from_status", existingOrder.OrderPaymentStatus).
		Int32("to_status", targetPaymentStatus).
		Float64("refunded_amount", newTotalRefunded).
		Str("transaction_ref", trxRef).
		Str("note", noteStr).
		Str("changed_by", userUUID.String()).
		Msg("Order payment status changed with audit info")

	if s.cache != nil {
		userKey := fmt.Sprintf("orders:user:%s:*", existingOrder.UserUuid.String())
		_ = s.cache.Clear(userKey)
		_ = s.cache.Clear("orders:admin:*")
		_ = s.cache.Del(fmt.Sprintf("invoice:pdf:%s", existingOrder.OrderUuid.String()))
	}

	dto := v1dto.MapToOrderDTO(updatedOrder, orderItems)
	return &dto, nil
}

func (s *orderLifecycleService) BulkUpdateStatus(ctx context.Context, input v1dto.BulkUpdateOrderStatusInput) (*v1dto.BulkOperationResultDTO, error) {
	result := &v1dto.BulkOperationResultDTO{
		Total:  len(input.OrderUUIDs),
		Errors: make([]v1dto.BulkItemError, 0),
	}

	for _, rawUUID := range input.OrderUUIDs {
		parsedUUID, err := uuid.Parse(rawUUID)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, v1dto.BulkItemError{
				OrderUUID: rawUUID,
				Error:     "invalid UUID format",
			})
			continue
		}

		_, err = s.UpdateOrderStatus(ctx, parsedUUID, v1dto.UpdateOrderStatusInput{
			Status: input.Status,
		})
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, v1dto.BulkItemError{
				OrderUUID: rawUUID,
				Error:     err.Error(),
			})
			continue
		}

		result.Succeeded++
	}

	return result, nil
}

func (s *orderLifecycleService) BulkCancel(ctx context.Context, userUUID uuid.UUID, input v1dto.BulkCancelOrdersInput) (*v1dto.BulkOperationResultDTO, error) {
	result := &v1dto.BulkOperationResultDTO{
		Total:  len(input.OrderUUIDs),
		Errors: make([]v1dto.BulkItemError, 0),
	}

	for _, rawUUID := range input.OrderUUIDs {
		parsedUUID, err := uuid.Parse(rawUUID)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, v1dto.BulkItemError{
				OrderUUID: rawUUID,
				Error:     "invalid UUID format",
			})
			continue
		}

		_, err = s.CancelOrder(ctx, userUUID, parsedUUID, input.Reason)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, v1dto.BulkItemError{
				OrderUUID: rawUUID,
				Error:     err.Error(),
			})
			continue
		}

		result.Succeeded++
	}

	return result, nil
}
