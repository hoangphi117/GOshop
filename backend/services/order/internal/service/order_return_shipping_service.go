package service

import (
	"context"
	"fmt"
	"strings"

	"goshop/internal/db"
	"goshop/pkg/utils"
	"goshop/pkg/rabbitmq"
	"goshop/services/order/internal/db/sqlc"
	v1dto "goshop/services/order/internal/dto"

	"github.com/google/uuid"
)

// SubmitReturnShipping submits customer shipping courier and tracking code for an approved return.
func (s *orderReturnService) SubmitReturnShipping(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.SubmitReturnShippingInput) (v1dto.SubmitReturnShippingResponseDTO, error) {
	user, err := s.getCustomer(ctx, userUUID)
	if err != nil {
		return v1dto.SubmitReturnShippingResponseDTO{}, err
	}

	resolved, err := s.resolveOrderReturn(ctx, orderUUID)
	if err != nil {
		return v1dto.SubmitReturnShippingResponseDTO{}, err
	}

	if !isReturnAccessibleByUser(user, resolved) {
		return v1dto.SubmitReturnShippingResponseDTO{}, utils.NewError(utils.ErrCodeForbidden, "Access denied")
	}

	returnRequest := resolved.Return
	if returnRequest.ReturnStatus != OrderReturnStatusApproved {
		var errMsg string
		switch returnRequest.ReturnStatus {
		case OrderReturnStatusPending:
			errMsg = "Return request is pending approval. Please do not ship the item without store approval."
		case OrderReturnStatusRejected:
			errMsg = "Return request has been rejected. You cannot update shipping information."
		case OrderReturnStatusCompleted:
			errMsg = "Return request has been completed."
		case OrderReturnStatusCancelled:
			errMsg = "Return request has been cancelled."
		default:
			errMsg = "Invalid return request status for updating shipping information."
		}
		return v1dto.SubmitReturnShippingResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest, errMsg)
	}

	provider := strings.TrimSpace(input.ShippingProvider)
	shippingCode := strings.ToUpper(strings.TrimSpace(input.ShippingCode))
	var note *string
	if strings.TrimSpace(input.Note) != "" {
		n := strings.TrimSpace(input.Note)
		note = &n
	}

	var updatedReturn sqlc.OrderReturn
	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		tx, _ := db.GetTx(txCtx)
		var err error
		updatedReturn, err = s.orderReturnRepo.UpdateReturnShippingInfo(txCtx, tx, sqlc.UpdateReturnShippingInfoParams{
			ReturnShippingProvider: &provider,
			ReturnShippingCode:     &shippingCode,
			ReturnShippingNote:     note,
			ReturnUuid:             returnRequest.ReturnUuid,
		})
		if err != nil {
			return err
		}

		if s.trackingRepo != nil {
			eventNote := fmt.Sprintf("Return waybill submitted for %s (Tracking Code: %s).", strings.ToUpper(provider), shippingCode)
			if note != nil {
				eventNote += fmt.Sprintf(" Note: %s", *note)
			}
			_, err = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
				EventOrderID: returnRequest.ReturnOrderID,
				EventStatus:  "return_shipping_submitted",
				EventNote:    &eventNote,
				EventSource:  "user",
			})
			if err != nil {
				return err
			}
		}

		if s.outboxStore != nil {
			event := rabbitmq.NewOrderReturnUpdatedEvent(
				uuid.NewString(),
				updatedReturn.ReturnUuid.String(),
				resolved.OrderUUID.String(),
				resolved.OrderNumber,
				updatedReturn.ReturnID,
				resolved.OrderID,
				resolved.OrderUserID,
				updatedReturn.ReturnStatus,
				fmt.Sprintf("Return shipping submitted: %s (%s)", shippingCode, provider),
			)
			event.ChangeType = rabbitmq.ReturnChangeTypeShippingSent
			_, _ = s.outboxStore.Insert(ctx, tx, "order", resolved.OrderUUID.String(), rabbitmq.QueueOrderReturnUpdated, event)
		}
		return nil
	})
	if err != nil {
		return v1dto.SubmitReturnShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update return shipping info", err)
	}

	if s.cache != nil {
		_ = s.cache.Clear(fmt.Sprintf("order:%s", resolved.OrderUUID.String()))
	}

	return v1dto.SubmitReturnShippingResponseDTO{
		ReturnUUID:       updatedReturn.ReturnUuid.String(),
		OrderUUID:        resolved.OrderUUID.String(),
		OrderNumber:      resolved.OrderNumber,
		ShippingProvider: provider,
		ShippingCode:     shippingCode,
		Note:             note,
		Status:           updatedReturn.ReturnStatus,
		StatusName:       v1dto.MapReturnStatusToString(updatedReturn.ReturnStatus),
		StatusText:       v1dto.MapReturnStatusToVietnamese(updatedReturn.ReturnStatus),
		UpdatedAt:        updatedReturn.ReturnUpdatedAt,
		Message:          "Return shipping information submitted successfully. Please wait for the store to process your return request.",
	}, nil
}

// UpdateReturnBankAccount updates customer refund bank details for pending or approved return requests.
func (s *orderReturnService) UpdateReturnBankAccount(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateReturnBankAccountInput) (v1dto.UpdateReturnBankAccountResponseDTO, error) {
	user, err := s.getCustomer(ctx, userUUID)
	if err != nil {
		return v1dto.UpdateReturnBankAccountResponseDTO{}, err
	}

	resolved, err := s.resolveOrderReturn(ctx, orderUUID)
	if err != nil {
		return v1dto.UpdateReturnBankAccountResponseDTO{}, err
	}

	if !isReturnAccessibleByUser(user, resolved) {
		return v1dto.UpdateReturnBankAccountResponseDTO{}, utils.NewError(utils.ErrCodeForbidden, "Access denied")
	}

	returnRequest := resolved.Return
	if returnRequest.ReturnStatus != OrderReturnStatusPending && returnRequest.ReturnStatus != OrderReturnStatusApproved {
		var errMsg string
		switch returnRequest.ReturnStatus {
		case OrderReturnStatusRejected:
			errMsg = "Return request has been rejected. You cannot update bank account information."
		case OrderReturnStatusCompleted:
			errMsg = "Return request has been completed and refunded. You cannot update bank account information."
		case OrderReturnStatusCancelled:
			errMsg = "Return request has been cancelled. You cannot update bank account information."
		default:
			errMsg = "Invalid return request status for updating bank account information."
		}
		return v1dto.UpdateReturnBankAccountResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest, errMsg)
	}

	bankName := strings.TrimSpace(input.BankName)
	bankAccount := strings.TrimSpace(input.BankAccount)
	bankAccountName := strings.ToUpper(strings.TrimSpace(input.BankAccountName))

	var updatedReturn sqlc.OrderReturn
	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		tx, _ := db.GetTx(txCtx)
		var err error
		updatedReturn, err = s.orderReturnRepo.UpdateReturnBankAccount(txCtx, tx, sqlc.UpdateReturnBankAccountParams{
			ReturnBankName:        &bankName,
			ReturnBankAccount:     &bankAccount,
			ReturnBankAccountName: &bankAccountName,
			ReturnUuid:            returnRequest.ReturnUuid,
		})
		if err != nil {
			return err
		}

		if s.trackingRepo != nil {
			eventNote := fmt.Sprintf("Customer updated refund bank account: %s - %s (%s)", bankName, bankAccount, bankAccountName)
			_, _ = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
				EventOrderID: returnRequest.ReturnOrderID,
				EventStatus:  "return_bank_updated",
				EventNote:    &eventNote,
				EventSource:  "user",
			})
		}
		return nil
	})
	if err != nil {
		return v1dto.UpdateReturnBankAccountResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update return bank account", err)
	}

	if s.cache != nil {
		_ = s.cache.Clear(fmt.Sprintf("order:%s", resolved.OrderUUID.String()))
		_ = s.cache.Clear("orders:*")
	}

	return v1dto.UpdateReturnBankAccountResponseDTO{
		ReturnUUID:      returnRequest.ReturnUuid.String(),
		OrderUUID:       resolved.OrderUUID.String(),
		OrderNumber:     resolved.OrderNumber,
		BankName:        bankName,
		BankAccount:     bankAccount,
		BankAccountName: bankAccountName,
		Status:          updatedReturn.ReturnStatus,
		StatusName:      v1dto.MapReturnStatusToString(updatedReturn.ReturnStatus),
		StatusText:      v1dto.MapReturnStatusToVietnamese(updatedReturn.ReturnStatus),
		UpdatedAt:       updatedReturn.ReturnUpdatedAt,
		Message:         "Bank account information updated successfully.",
	}, nil
}
