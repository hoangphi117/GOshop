package v1service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/utils"
	"goshop/pkg/rabbitmq"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AdminUpdateReturnStatus handles Admin inspection, approval, rejection, and receiving of returned goods with automatic inventory restock.
func (s *orderReturnService) AdminUpdateReturnStatus(ctx context.Context, userUUID uuid.UUID, returnUUID uuid.UUID, input v1dto.AdminUpdateReturnInput) (v1dto.AdminReturnSummaryDTO, error) {
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		return v1dto.AdminReturnSummaryDTO{}, s.handleUserNotFoundError(err)
	}

	if user.UserLevel != RoleAdmin && user.UserLevel != RoleModerator {
		return v1dto.AdminReturnSummaryDTO{}, utils.NewError(utils.ErrCodeForbidden, "Admin or Moderator access required")
	}

	returnRequest, err := s.orderReturnRepo.GetByUUID(ctx, returnUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v1dto.AdminReturnSummaryDTO{}, utils.NewError(utils.ErrCodeNotFound, "Return request not found")
		}
		return v1dto.AdminReturnSummaryDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get return request", err)
	}

	if returnRequest.ReturnStatus == OrderReturnStatusCompleted || returnRequest.ReturnStatus == OrderReturnStatusRejected {
		return v1dto.AdminReturnSummaryDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Cannot update a completed or rejected return request")
	}

	if input.Status == OrderReturnStatusCompleted && returnRequest.ReturnStatus != OrderReturnStatusApproved {
		return v1dto.AdminReturnSummaryDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Return request must be in 'Approved' status before marking as 'Completed'")
	}

	order, err := s.orderRepo.GetByID(ctx, returnRequest.ReturnOrderID)
	if err != nil {
		return v1dto.AdminReturnSummaryDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order for return request", err)
	}

	noteStr := ""
	if input.AdminNote != nil && strings.TrimSpace(*input.AdminNote) != "" {
		noteStr = *input.AdminNote
	}

	var updatedReturn sqlc.OrderReturn
	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		tx, _ := db.GetTx(txCtx)
		var err error
		updatedReturn, err = s.orderReturnRepo.UpdateStatus(txCtx, tx, sqlc.UpdateReturnStatusParams{
			ReturnUuid: returnUUID,
			Status:     input.Status,
			AdminNote:  input.AdminNote,
		})
		if err != nil {
			return err
		}

		switch input.Status {
		case OrderReturnStatusApproved:
			if s.trackingRepo != nil {
				eventNote := fmt.Sprintf("Request approved. Please ship the package back to the warehouse and update shipping information in \"Returns & Refunds\" tab. Note: %s", noteStr)
				_, _ = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
					EventOrderID: returnRequest.ReturnOrderID,
					EventStatus:  "return_approved",
					EventNote:    &eventNote,
					EventSource:  "admin",
				})
			}
		case OrderReturnStatusRejected:
			if s.trackingRepo != nil {
				eventNote := fmt.Sprintf("Request rejected. Reason: %s", noteStr)
				_, _ = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
					EventOrderID: returnRequest.ReturnOrderID,
					EventStatus:  "return_rejected",
					EventNote:    &eventNote,
					EventSource:  "admin",
				})
			}
		case OrderReturnStatusCompleted:
			_, err := s.orderRepo.UpdateStatus(txCtx, tx, sqlc.UpdateOrderStatusParams{
				OrderUuid:   order.OrderUuid,
				OrderStatus: OrderStatusReturned,
			})
			if err != nil {
				return err
			}

			// Automatically restock returned items in warehouse inventory via StockPort
			if err := s.restockReturnedOrderItems(txCtx, order, returnRequest); err != nil {
				return err
			}

			if s.trackingRepo != nil {
				eventNote := fmt.Sprintf("Warehouse received returned goods successfully and restored inventory. Awaiting refund processing. Note: %s", noteStr)
				_, _ = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
					EventOrderID: returnRequest.ReturnOrderID,
					EventStatus:  "returned",
					EventNote:    &eventNote,
					EventSource:  "admin",
				})
			}
		}

		if s.outboxStore != nil {
			event := rabbitmq.NewOrderReturnUpdatedEvent(
				uuid.NewString(),
				updatedReturn.ReturnUuid.String(),
				order.OrderUuid.String(),
				order.OrderNumber,
				updatedReturn.ReturnID,
				order.OrderID,
				order.OrderUserID,
				updatedReturn.ReturnStatus,
				noteStr,
			)
			_, _ = s.outboxStore.Insert(ctx, tx, "order", order.OrderUuid.String(), rabbitmq.QueueOrderReturnUpdated, event)
		}
		return nil
	})
	if err != nil {
		return v1dto.AdminReturnSummaryDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update return request status", err)
	}

	if s.cache != nil {
		_ = s.cache.Clear(fmt.Sprintf("order:%s", order.OrderUuid.String()))
		_ = s.cache.Clear("products:*")
		_ = s.cache.Clear("product:*")
	}

	return v1dto.AdminReturnSummaryDTO{
		ReturnUUID: updatedReturn.ReturnUuid.String(),
		Status:     updatedReturn.ReturnStatus,
		StatusText: v1dto.MapReturnStatusToVietnamese(updatedReturn.ReturnStatus),
		AdminNote:  updatedReturn.ReturnAdminNote,
		UpdatedAt:  updatedReturn.ReturnUpdatedAt,
	}, nil
}

// restockReturnedOrderItems restores warehouse inventory for all items in the returned order via OrderStockPort.
func (s *orderReturnService) restockReturnedOrderItems(ctx context.Context, order sqlc.GetOrderByIDRow, returnRequest sqlc.OrderReturn) error {
	orderItems, err := s.orderItemRepo.GetByOrderID(ctx, returnRequest.ReturnOrderID)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "failed to get order items for return restock", err)
	}
	if len(orderItems) == 0 {
		return nil
	}

	restoreItems := make([]RestoreItem, len(orderItems))
	for i, item := range orderItems {
		restoreItems[i] = RestoreItem{
			ProductID: item.OrderItemProductID,
			Quantity:  item.OrderItemQuantity,
		}
	}

	refID := order.OrderNumber
	note := fmt.Sprintf("Restock from completed return (Order #%s, Return ID #%d)", order.OrderNumber, returnRequest.ReturnID)
	if s.stockPort != nil {
		if err := s.stockPort.RestoreStockWithRef(ctx, restoreItems, refID, "order_return", nil, &note); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "failed to restore stock for returned order", err)
		}
	}
	return nil
}

// AdminProcessReturnRefund executes refund payout (either online payment gateway refund or manual bank transfer).
func (s *orderReturnService) AdminProcessReturnRefund(ctx context.Context, userUUID uuid.UUID, returnUUID uuid.UUID, input v1dto.AdminReturnRefundInput) (v1dto.AdminReturnRefundResponseDTO, error) {
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		return v1dto.AdminReturnRefundResponseDTO{}, s.handleUserNotFoundError(err)
	}

	if user.UserLevel != RoleAdmin && user.UserLevel != RoleModerator {
		return v1dto.AdminReturnRefundResponseDTO{}, utils.NewError(utils.ErrCodeForbidden, "Admin or Moderator access required")
	}

	resolved, err := s.resolveOrderReturn(ctx, returnUUID)
	if err != nil {
		return v1dto.AdminReturnRefundResponseDTO{}, err
	}

	returnRequest := resolved.Return
	if returnRequest.ReturnStatus != OrderReturnStatusCompleted && returnRequest.ReturnStatus != OrderReturnStatusApproved {
		return v1dto.AdminReturnRefundResponseDTO{}, utils.NewError(
			utils.ErrCodeBadRequest,
			"Refund can only be processed when return request is Approved or Completed. Current status: "+v1dto.MapReturnStatusToString(returnRequest.ReturnStatus),
		)
	}

	orderRow, err := s.orderRepo.GetByID(ctx, returnRequest.ReturnOrderID)
	if err != nil {
		return v1dto.AdminReturnRefundResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order for return request", err)
	}

	orderTotalAmount := utils.NumericToFloat(orderRow.OrderTotalAmount)
	currentRefunded := utils.NumericToFloat(orderRow.OrderRefundedAmount)
	remainingRefundable := orderTotalAmount - currentRefunded

	refundAmt := remainingRefundable
	if input.Amount != nil && *input.Amount > 0 {
		refundAmt = *input.Amount
	}
	if refundAmt > remainingRefundable {
		return v1dto.AdminReturnRefundResponseDTO{}, utils.NewError(
			utils.ErrCodeBadRequest,
			fmt.Sprintf("Refund amount exceeds refundable balance. Maximum refundable: %.0f VND", remainingRefundable),
		)
	}

	refundReason := input.Reason
	if strings.TrimSpace(refundReason) == "" {
		refundReason = fmt.Sprintf("Refund for return request: %s", returnRequest.ReturnReason)
	}

	var refundProvider string
	var refundTxnID string
	var totalRefunded float64

	isCOD := orderRow.OrderPaymentMethod == OrderPaymentMethodCOD
	isManualBankRefund := strings.TrimSpace(input.BankTxnRef) != "" || isCOD

	if !isManualBankRefund {
		// Try online payment gateway refund via payment service
		idempotencyKey := fmt.Sprintf("refund:return:%s:%.0f", returnRequest.ReturnUuid, refundAmt)
		txnDTO, err := s.paymentService.RefundPayment(ctx, idempotencyKey, v1dto.RefundPaymentInput{
			OrderUUID: resolved.OrderUUID.String(),
			Amount:    refundAmt,
			Reason:    refundReason,
		})
		if err == nil {
			refundProvider = txnDTO.Provider
			if txnDTO.ProviderTxnID != nil {
				refundTxnID = *txnDTO.ProviderTxnID
			}
			if txnDTO.RefundAmount != nil {
				totalRefunded = *txnDTO.RefundAmount
			} else {
				totalRefunded = currentRefunded + refundAmt
			}
		} else if strings.Contains(err.Error(), "No successful original payment transaction found") || strings.Contains(err.Error(), "not supported") {
			// Seamlessly fallback to manual bank transfer refund
			isManualBankRefund = true
		} else {
			return v1dto.AdminReturnRefundResponseDTO{}, err
		}
	}

	if isManualBankRefund {
		refundProvider = "bank_transfer"
		if strings.TrimSpace(input.BankTxnRef) != "" {
			refundTxnID = strings.TrimSpace(input.BankTxnRef)
		} else {
			refundTxnID = fmt.Sprintf("BANK_REFUND_%s_%d", resolved.OrderNumber, time.Now().Unix())
		}
		totalRefunded = currentRefunded + refundAmt
	}

	refundCompletedNote := fmt.Sprintf("Return completed and refunded %.0f VND via %s (Txn ID: %s)", refundAmt, strings.ToUpper(refundProvider), refundTxnID)
	if strings.TrimSpace(input.AccountingNote) != "" {
		refundCompletedNote += fmt.Sprintf(". Note: %s", strings.TrimSpace(input.AccountingNote))
	}

	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		tx, _ := db.GetTx(txCtx)

		// 1. Update Return Status to Completed
		_, err = s.orderReturnRepo.UpdateStatus(txCtx, tx, sqlc.UpdateReturnStatusParams{
			ReturnUuid: returnRequest.ReturnUuid,
			Status:     OrderReturnStatusCompleted,
			AdminNote:  &refundCompletedNote,
		})
		if err != nil {
			return err
		}

		// 2. Update order payment status and refunded amount for manual bank refund
		if isManualBankRefund {
			var newPaymentStatus int32 = OrderPaymentStatusPartiallyRefunded
			if totalRefunded >= orderTotalAmount {
				newPaymentStatus = OrderPaymentStatusFullyRefunded
			}
			_, _ = s.orderRepo.UpdateOrderPaymentStatus(txCtx, tx, sqlc.UpdateOrderPaymentStatusParams{
				OrderUuid:           resolved.OrderUUID,
				PaymentStatus:       newPaymentStatus,
				RefundedAmount:      utils.FloatToNumeric(totalRefunded),
				AllowedFromStatuses: []int32{OrderPaymentStatusUnpaid, OrderPaymentStatusPaid, OrderPaymentStatusPartiallyRefunded},
			})
		}

		// 3. Update Order Status to Returned if fully refunded
		if totalRefunded >= orderTotalAmount {
			_, _ = s.orderRepo.UpdateStatus(txCtx, tx, sqlc.UpdateOrderStatusParams{
				OrderUuid:   resolved.OrderUUID,
				OrderStatus: OrderStatusReturned,
			})
		}

		// 4. Insert tracking event
		if s.trackingRepo != nil {
			eventNote := fmt.Sprintf("Refund processed successfully: %.0f VND via %s. Txn ID: %s.",
				refundAmt, strings.ToUpper(refundProvider), refundTxnID)
			if strings.TrimSpace(input.AccountingNote) != "" {
				eventNote += fmt.Sprintf(" Note: %s", strings.TrimSpace(input.AccountingNote))
			}

			_, err = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
				EventOrderID: resolved.OrderID,
				EventStatus:  "refunded",
				EventNote:    &eventNote,
				EventSource:  "admin",
			})
			if err != nil {
				return err
			}
		}

		if s.outboxStore != nil {
			event := rabbitmq.NewOrderReturnUpdatedEvent(
				uuid.NewString(),
				returnRequest.ReturnUuid.String(),
				resolved.OrderUUID.String(),
				resolved.OrderNumber,
				returnRequest.ReturnID,
				resolved.OrderID,
				resolved.OrderUserID,
				OrderReturnStatusCompleted,
				fmt.Sprintf("Refund of %.0f VND processed", refundAmt),
			)
			_, _ = s.outboxStore.Insert(ctx, tx, "order", resolved.OrderUUID.String(), rabbitmq.QueueOrderReturnUpdated, event)
		}
		return nil
	})
	if err != nil {
		return v1dto.AdminReturnRefundResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to finalize return status", err)
	}

	if s.cache != nil {
		_ = s.cache.Clear(fmt.Sprintf("order:%s", resolved.OrderUUID.String()))
		_ = s.cache.Clear("orders:*")
		_ = s.cache.Clear("products:*")
		_ = s.cache.Clear("product:*")
	}

	paymentStatusStr := "REFUNDED"
	if totalRefunded < orderTotalAmount {
		paymentStatusStr = "PARTIALLY_REFUNDED"
	}

	return v1dto.AdminReturnRefundResponseDTO{
		ReturnUUID:       returnRequest.ReturnUuid.String(),
		OrderUUID:        resolved.OrderUUID.String(),
		OrderNumber:      resolved.OrderNumber,
		RefundAmount:     refundAmt,
		TotalRefunded:    totalRefunded,
		OrderTotalAmount: orderTotalAmount,
		PaymentMethod:    v1dto.MapPaymentMethodToString(orderRow.OrderPaymentMethod),
		PaymentStatus:    paymentStatusStr,
		RefundTxnID:      refundTxnID,
		RefundProvider:   refundProvider,
		RefundedAt:       time.Now(),
		Message:          "Return refund processed successfully.",
	}, nil
}
