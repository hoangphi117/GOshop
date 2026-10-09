package v1service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type orderLifecycleService struct {
	txManager           db.TxManager
	orderStore          OrderStorePort
	orderItemStore      OrderItemStorePort
	trackingStore       OrderTrackingPort
	userReader          OrderCustomerReader
	stockPort           OrderStockPort
	notificationService NotificationService
	cache               cache.RedisCacheService
}

func NewOrderLifecycleService(
	txManager db.TxManager,
	orderStore OrderStorePort,
	orderItemStore OrderItemStorePort,
	trackingStore OrderTrackingPort,
	userReader OrderCustomerReader,
	stockPort OrderStockPort,
	notificationService NotificationService,
	cacheService cache.RedisCacheService,
) OrderLifecycleService {
	return &orderLifecycleService{
		txManager:           txManager,
		orderStore:          orderStore,
		orderItemStore:      orderItemStore,
		trackingStore:       trackingStore,
		userReader:          userReader,
		stockPort:           stockPort,
		notificationService: notificationService,
		cache:               cacheService,
	}
}

func (s *orderLifecycleService) UpdateOrderStatus(ctx context.Context, input sqlc.UpdateOrderStatusParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {
	existingOrder, err := s.orderStore.GetByUUID(ctx, input.OrderUuid)
	if err != nil {
		return sqlc.Order{}, nil, handleOrderNotFoundError(err)
	}

	if !isValidStatusTransition(existingOrder.OrderStatus, input.OrderStatus) {
		errMsg := fmt.Sprintf("Cannot change order status from '%s' to '%s'",
			v1dto.MapOrderStatusToString(existingOrder.OrderStatus),
			v1dto.MapOrderStatusToString(input.OrderStatus),
		)
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, errMsg)
	}

	updatedOrder, err := s.orderStore.UpdateStatus(ctx, nil, input)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeNotFound, "order not found")
		}
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "failed to update order status", err)
	}

	var statusNote string
	switch input.OrderStatus {
	case OrderStatusConfirmed:
		statusNote = "Order was confirmed by shop admin."
	case OrderStatusDelivered:
		statusNote = "Order was marked as delivered."
	}

	if statusNote != "" {
		_, _ = s.trackingStore.InsertEvent(ctx, nil, sqlc.InsertTrackingEventParams{
			EventOrderID: updatedOrder.OrderID,
			EventStatus:  v1dto.MapOrderStatusToString(input.OrderStatus),
			EventNote:    &statusNote,
			EventSource:  "admin",
		})
	}

	orderItems, err := s.orderItemStore.GetByOrderID(ctx, updatedOrder.OrderID)
	if err != nil {
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "failed to get order items", err)
	}

	if s.cache != nil {
		if err := s.cache.Clear("orders:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear order cache")
		}
	}

	if input.OrderStatus == OrderStatusConfirmed {
		go func() {
			logger.Log.Info().
				Int32("target_customer_id", updatedOrder.OrderUserID).
				Str("order_number", updatedOrder.OrderNumber).
				Msg("Dispatching order_confirmed notification")

			if s.notificationService != nil {
				err := s.notificationService.Send(context.Background(), v1dto.SendNotificationInput{
					UserID: updatedOrder.OrderUserID,
					Type:   "order_confirmed",
					Title:  "Order Confirmed",
					Body:   fmt.Sprintf("Your order #%s has been confirmed and is being packed.", updatedOrder.OrderNumber),
					Data:   map[string]any{"order_uuid": updatedOrder.OrderUuid.String()},
				})
				if err != nil {
					logger.Log.Error().Err(err).
						Int32("target_customer_id", updatedOrder.OrderUserID).
						Msg("Failed to send order_confirmed notification")
				}
			}
		}()
	}

	return updatedOrder, orderItems, nil
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

func (s *orderLifecycleService) CancelOrder(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, reason *string) (sqlc.Order, error) {
	// 1. Lấy thông tin User & Order
	user, err := s.userReader.GetByUUID(ctx, userUUID)
	if err != nil {
		return sqlc.Order{}, handleUserNotFoundError(err)
	}

	orderRow, err := s.orderStore.GetByUUID(ctx, orderUUID)
	if err != nil {
		return sqlc.Order{}, handleOrderNotFoundError(err)
	}

	// 2. Validate điều kiện hủy đơn (Domain Rule)
	if err := s.validateOrderCancellation(user, orderRow); err != nil {
		return sqlc.Order{}, err
	}

	// 3. Lấy danh sách sản phẩm để chuẩn bị hoàn kho
	orderItems, err := s.orderItemStore.GetByOrderID(ctx, orderRow.OrderID)
	if err != nil {
		return sqlc.Order{}, utils.WrapError(utils.ErrCodeInternal, "failed to get order items", err)
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

		return nil
	})

	if err != nil {
		return sqlc.Order{}, err
	}

	// 5. Side-effects hậu commit (Cache & Notification)
	InvalidateOrderCachesWithCache(s.cache, user.UserUuid)
	s.sendCancellationNotification(ctx, orderRow)

	return cancelledOrder, nil
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

		return nil
	})

	if err != nil {
		return err
	}

	// 4. Invalidate caches & send cancellation notification
	InvalidateOrderCachesWithCache(s.cache, orderRow.UserUuid)
	s.sendCancellationNotification(ctx, orderRow)

	return nil
}

func (s *orderLifecycleService) validateOrderCancellation(user sqlc.User, order sqlc.GetOrderByUUIDRow) error {
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

func (s *orderLifecycleService) recordCancellationTracking(ctx context.Context, tx pgx.Tx, user sqlc.User, order sqlc.GetOrderByUUIDRow, reason *string) error {
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

func (s *orderLifecycleService) sendCancellationNotification(ctx context.Context, order sqlc.GetOrderByUUIDRow) {
	if s.notificationService == nil {
		return
	}
	err := s.notificationService.Send(ctx, v1dto.SendNotificationInput{
		UserID: order.OrderUserID,
		Type:   "order_cancelled",
		Title:  "Order Cancelled",
		Body:   fmt.Sprintf("Your order #%s has been cancelled.", order.OrderNumber),
		Data:   map[string]any{"order_uuid": order.OrderUuid.String()},
	})
	if err != nil {
		logger.Log.Warn().
			Err(err).
			Int32("user_id", order.OrderUserID).
			Str("order_number", order.OrderNumber).
			Msg("Failed to send order cancellation notification")
	}
}

func (s *orderLifecycleService) UpdateOrderShippingInfo(ctx context.Context, userUUID uuid.UUID, input sqlc.UpdateOrderShippingInfoParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {
	user, err := s.userReader.GetByUUID(ctx, userUUID)
	if err != nil {
		return sqlc.Order{}, nil, handleUserNotFoundError(err)
	}

	existingOrder, err := s.orderStore.GetByUUID(ctx, input.OrderUuid)
	if err != nil {
		return sqlc.Order{}, nil, handleOrderNotFoundError(err)
	}

	isAdmin := user.UserLevel == RoleAdmin
	isModerator := user.UserLevel == RoleModerator

	if existingOrder.OrderUserID != user.UserID && !isAdmin && !isModerator {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeForbidden, "You do not have permission to update this order")
	}

	if isAdmin || isModerator {
		if existingOrder.OrderStatus != OrderStatusPending && existingOrder.OrderStatus != OrderStatusConfirmed {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Admin or moderator can only update shipping info for Pending or Confirmed orders")
		}
	} else {
		if existingOrder.OrderStatus != OrderStatusPending {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "You can only update shipping info for Pending orders")
		}
	}

	allowedStatuses := []int32{OrderStatusPending}
	if isAdmin || isModerator {
		allowedStatuses = append(allowedStatuses, OrderStatusConfirmed)
	}

	isStatusValid := slices.Contains(allowedStatuses, existingOrder.OrderStatus)
	if !isStatusValid {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Order status does not allow shipping info update")
	}

	input.AllowedStatuses = allowedStatuses

	updatedOrder, err := s.orderStore.UpdateOrderShippingInfo(ctx, input)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeConflict, "Order status has changed or order not found, please refresh and try again")
		}
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "Failed to update shipping info", err)
	}

	orderItems, err := s.orderItemStore.GetByOrderID(ctx, updatedOrder.OrderID)
	if err != nil {
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "Failed to get order items", err)
	}

	InvalidateOrderCachesWithCache(s.cache, existingOrder.UserUuid)
	return updatedOrder, orderItems, nil
}

func (s *orderLifecycleService) UpdateOrderPaymentStatus(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateOrderPaymentStatusInput) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {
	existingOrder, err := s.orderStore.GetByUUID(ctx, orderUUID)
	if err != nil {
		return sqlc.Order{}, nil, handleOrderNotFoundError(err)
	}

	var allowedFromStatuses []int32
	var targetPaymentStatus int32 = input.PaymentStatus
	newTotalRefunded := utils.NumericToFloat(existingOrder.OrderRefundedAmount)

	switch input.PaymentStatus {
	case OrderPaymentStatusPaid:
		if existingOrder.OrderPaymentStatus != OrderPaymentStatusUnpaid {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Only Unpaid orders can be marked as Paid")
		}
		if existingOrder.OrderStatus == OrderStatusCancelled || existingOrder.OrderStatus == OrderStatusReturned {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Cannot mark a cancelled or returned order as Paid")
		}
		allowedFromStatuses = []int32{OrderPaymentStatusUnpaid}

	case OrderPaymentStatusPartiallyRefunded, OrderPaymentStatusFullyRefunded:
		if existingOrder.OrderPaymentStatus != OrderPaymentStatusPaid && existingOrder.OrderPaymentStatus != OrderPaymentStatusPartiallyRefunded {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Only Paid or Partially Refunded orders can be refunded")
		}
		if existingOrder.OrderStatus != OrderStatusCancelled && existingOrder.OrderStatus != OrderStatusReturned {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Can only refund orders that are Cancelled (5) or Returned (6)")
		}

		totalAmount := utils.NumericToFloat(existingOrder.OrderTotalAmount)
		currentRefunded := utils.NumericToFloat(existingOrder.OrderRefundedAmount)

		refundAmountVal := totalAmount - currentRefunded
		if input.RefundAmount != nil && *input.RefundAmount > 0 {
			refundAmountVal = *input.RefundAmount
		}

		newTotalRefunded = utils.RoundFloat(currentRefunded+refundAmountVal, 2)
		if newTotalRefunded > totalAmount {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Total refunded amount (%.2f) cannot exceed order total (%.2f)", newTotalRefunded, totalAmount))
		}

		if newTotalRefunded >= totalAmount {
			targetPaymentStatus = OrderPaymentStatusFullyRefunded
		} else {
			targetPaymentStatus = OrderPaymentStatusPartiallyRefunded
		}

		allowedFromStatuses = []int32{OrderPaymentStatusPaid, OrderPaymentStatusPartiallyRefunded}

	default:
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Invalid target payment status")
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
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeConflict, "Payment status has changed by another request, please refresh")
		}
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "Failed to update payment status", err)
	}

	orderItems, err := s.orderItemStore.GetByOrderID(ctx, updatedOrder.OrderID)
	if err != nil {
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "Failed to get order items", err)
	}

	trxRef := ""
	if input.TransactionRef != nil {
		trxRef = *input.TransactionRef
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

	return updatedOrder, orderItems, nil
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

		_, _, err = s.UpdateOrderStatus(ctx, sqlc.UpdateOrderStatusParams{
			OrderUuid:   parsedUUID,
			OrderStatus: input.Status,
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
