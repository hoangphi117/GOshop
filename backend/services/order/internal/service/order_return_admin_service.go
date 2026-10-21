package service

import (
	"context"
	"errors"
	"strings"

	"goshop/services/order/internal/db/sqlc"
	v1dto "goshop/services/order/internal/dto"
	"goshop/internal/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AdminGetReturns retrieves paginated list of all customer return requests for backoffice moderation.
func (s *orderReturnService) AdminGetReturns(ctx context.Context, userUUID uuid.UUID, query v1dto.AdminGetReturnsQuery) ([]v1dto.AdminReturnSummaryDTO, int64, error) {
	user, err := s.getCustomer(ctx, userUUID)
	if err != nil {
		return nil, 0, err
	}

	if user.UserLevel != RoleAdmin && user.UserLevel != RoleModerator {
		return nil, 0, utils.NewError(utils.ErrCodeForbidden, "Admin or Moderator access required")
	}

	page := query.Page
	if page <= 0 {
		page = 1
	}

	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	offset := (page - 1) * limit

	var search *string
	if strings.TrimSpace(query.Search) != "" {
		trimmed := strings.TrimSpace(query.Search)
		search = &trimmed
	}

	total, err := s.orderReturnRepo.CountReturns(ctx, sqlc.AdminCountReturnsParams{
		Status: query.Status,
		Search: search,
	})
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to count returns requests", err)
	}

	rows, err := s.orderReturnRepo.GetReturns(ctx, sqlc.AdminGetReturnsParams{
		Status: query.Status,
		Search: search,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to get returns requests", err)
	}

	result := make([]v1dto.AdminReturnSummaryDTO, len(rows))
	for i, r := range rows {
		result[i] = v1dto.AdminReturnSummaryDTO{
			ReturnUUID:       r.ReturnUuid.String(),
			OrderUUID:        r.OrderUuid.String(),
			OrderNumber:      r.OrderNumber,
			CustomerName:     r.OrderShippingName,
			CustomerPhone:    r.OrderShippingPhone,
			CustomerEmail:    r.UserEmail,
			UserFullname:     r.UserFullname,
			TotalAmount:      utils.NumericToFloat(r.OrderTotalAmount),
			PaymentMethod:    v1dto.MapPaymentMethodToString(r.OrderPaymentMethod),
			PaymentStatus:    v1dto.MapPaymentStatusToString(r.OrderPaymentStatus),
			Reason:           r.ReturnReason,
			Images:           r.ReturnImages,
			VideoURL:         r.ReturnVideoUrl,
			Status:           r.ReturnStatus,
			StatusName:       v1dto.MapReturnStatusToString(r.ReturnStatus),
			StatusText:       v1dto.MapReturnStatusToVietnamese(r.ReturnStatus),
			AdminNote:        r.ReturnAdminNote,
			ShippingProvider: r.ReturnShippingProvider,
			ShippingCode:     r.ReturnShippingCode,
			ShippingNote:     r.ReturnShippingNote,
			CreatedAt:        r.ReturnCreatedAt,
			UpdatedAt:        r.ReturnUpdatedAt,
		}
	}
	return result, total, nil
}

// AdminGetReturnDetail retrieves comprehensive metadata of a single return ticket including timeline and line items.
func (s *orderReturnService) AdminGetReturnDetail(ctx context.Context, userUUID uuid.UUID, returnUUID uuid.UUID) (v1dto.AdminReturnDetailDTO, error) {
	user, err := s.getCustomer(ctx, userUUID)
	if err != nil {
		return v1dto.AdminReturnDetailDTO{}, err
	}

	if user.UserLevel != RoleAdmin && user.UserLevel != RoleModerator {
		return v1dto.AdminReturnDetailDTO{}, utils.NewError(utils.ErrCodeForbidden, "Admin or Moderator access required")
	}

	var returnRow sqlc.AdminGetReturnByUUIDRow
	returnByUUID, err := s.orderReturnRepo.AdminGetReturnByUUID(ctx, returnUUID)
	if err == nil {
		returnRow = returnByUUID
	} else if errors.Is(err, pgx.ErrNoRows) {
		// Fallback: check if the provided UUID is an order UUID
		orderByUUID, oErr := s.orderRepo.GetByUUID(ctx, returnUUID)
		if oErr != nil {
			if errors.Is(oErr, pgx.ErrNoRows) {
				return v1dto.AdminReturnDetailDTO{}, utils.NewError(utils.ErrCodeNotFound, "Return request or order not found")
			}
			return v1dto.AdminReturnDetailDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order", oErr)
		}
		latestReturn, lErr := s.orderReturnRepo.AdminGetLatestReturnByOrderID(ctx, orderByUUID.OrderID)
		if lErr != nil {
			if errors.Is(lErr, pgx.ErrNoRows) {
				return v1dto.AdminReturnDetailDTO{}, utils.NewError(utils.ErrCodeNotFound, "No return request found for this order")
			}
			return v1dto.AdminReturnDetailDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get return request", lErr)
		}
		returnRow = sqlc.AdminGetReturnByUUIDRow(latestReturn)
	} else {
		return v1dto.AdminReturnDetailDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get return request", err)
	}

	items, err := s.orderItemRepo.GetByOrderID(ctx, returnRow.OrderID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return v1dto.AdminReturnDetailDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order items", err)
	}

	itemsDTO := make([]v1dto.OrderItemDTO, len(items))
	for i, item := range items {
		var imgURL string
		if item.ProductImageUrl != nil {
			imgURL = *item.ProductImageUrl
		}
		itemsDTO[i] = v1dto.OrderItemDTO{
			UUID:        item.OrderItemUuid.String(),
			ProductUUID: item.ProductUuid.String(),
			ProductName: item.OrderItemProductName,
			ProductSlug: item.ProductSlug,
			ImageURL:    imgURL,
			Price:       utils.NumericToFloat(item.OrderItemPrice),
			Quantity:    item.OrderItemQuantity,
			Subtotal:    utils.NumericToFloat(item.OrderItemSubtotal),
		}
	}

	events, err := s.trackingRepo.GetByOrderID(ctx, returnRow.OrderID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return v1dto.AdminReturnDetailDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get tracking events", err)
	}

	returnEvents := make([]v1dto.TrackingEventDTO, 0)
	for _, ev := range events {
		if strings.HasPrefix(ev.EventStatus, "return_") || ev.EventStatus == "returned" {
			var note *string
			if ev.EventNote != nil && strings.TrimSpace(*ev.EventNote) != "" {
				note = ev.EventNote
			}
			returnEvents = append(returnEvents, v1dto.TrackingEventDTO{
				EventID:    ev.EventID,
				Status:     ev.EventStatus,
				StatusText: v1dto.MapTrackingStatusToText(ev.EventStatus),
				Note:       note,
				Source:     ev.EventSource,
				CreatedAt:  ev.EventCreatedAt,
			})
		}
	}

	return v1dto.AdminReturnDetailDTO{
		ReturnUUID:       returnRow.ReturnUuid.String(),
		Reason:           returnRow.ReturnReason,
		Images:           returnRow.ReturnImages,
		VideoURL:         returnRow.ReturnVideoUrl,
		Status:           int16(returnRow.ReturnStatus),
		StatusName:       v1dto.MapReturnStatusToString(returnRow.ReturnStatus),
		StatusText:       v1dto.MapReturnStatusToVietnamese(returnRow.ReturnStatus),
		AdminNote:        returnRow.ReturnAdminNote,
		ShippingProvider: returnRow.ReturnShippingProvider,
		ShippingCode:     returnRow.ReturnShippingCode,
		ShippingNote:     returnRow.ReturnShippingNote,
		BankName:         returnRow.ReturnBankName,
		BankAccount:      returnRow.ReturnBankAccount,
		BankAccountName:  returnRow.ReturnBankAccountName,
		CreatedAt:        returnRow.ReturnCreatedAt,
		UpdatedAt:        returnRow.ReturnUpdatedAt,
		Customer: v1dto.ReturnCustomerDTO{
			UserUUID: returnRow.UserUuid.String(),
			FullName: returnRow.OrderShippingName,
			Email:    returnRow.UserEmail,
		},
		Order: v1dto.ReturnOrderInfoDTO{
			OrderUUID:       returnRow.OrderUuid.String(),
			OrderNumber:     returnRow.OrderNumber,
			TotalAmount:     utils.NumericToFloat(returnRow.OrderTotalAmount),
			RefundedAmount:  utils.NumericToFloat(returnRow.OrderRefundedAmount),
			PaymentMethod:   v1dto.MapPaymentMethodToString(returnRow.OrderPaymentMethod),
			PaymentStatus:   v1dto.MapPaymentStatusToString(returnRow.OrderPaymentStatus),
			OrderStatus:     v1dto.MapOrderStatusToString(returnRow.OrderStatus),
			ShippingName:    returnRow.OrderShippingName,
			ShippingPhone:   returnRow.OrderShippingPhone,
			ShippingAddress: returnRow.OrderShippingAddress,
			CreatedAt:       returnRow.OrderCreatedAt,
		},
		Items:  itemsDTO,
		Events: returnEvents,
	}, nil
}
