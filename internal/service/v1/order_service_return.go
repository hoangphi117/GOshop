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
	"goshop/pkg/logger"
	"goshop/pkg/payment"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *orderService) RequestOrderReturn(ctx *gin.Context, orderUUID uuid.UUID, input v1dto.CreateOrderReturnInput, imageURLs []string, videoURL *string) (v1dto.OrderReturnDTO, error) {
	reqCtx := ctx.Request.Context()

	user, err := s.getUser(ctx)
	if err != nil {
		return v1dto.OrderReturnDTO{}, err
	}

	order, err := s.orderRepo.GetByUUID(reqCtx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v1dto.OrderReturnDTO{}, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return v1dto.OrderReturnDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
	}

	if order.OrderUserID != user.UserID {
		return v1dto.OrderReturnDTO{}, utils.NewError(utils.ErrCodeForbidden, "Access denied")
	}

	if order.OrderStatus != OrderStatusDelivered {
		return v1dto.OrderReturnDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Order is not eligible for return. Only delivered orders can be returned.")
	}

	if order.OrderDeliveredAt.Valid {
		deliveredTime := order.OrderDeliveredAt.Time
		if time.Since(deliveredTime) > 7*24*time.Hour {
			return v1dto.OrderReturnDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Return request period has expired. Returns can only be requested within 7 days of delivery.")
		}
	}

	existingReturn, err := s.orderReturnRepo.GetActiveByOrderID(reqCtx, order.OrderID)
	if err == nil && existingReturn.ReturnID > 0 {
		return v1dto.OrderReturnDTO{}, utils.NewError(utils.ErrCodeConflict, "An active return request already exists for this order")
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return v1dto.OrderReturnDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to check existing return request", err)
	}

	tx, err := db.BeginTx(reqCtx)
	if err != nil {
		return v1dto.OrderReturnDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to begin transaction", err)
	}
	defer tx.Rollback(reqCtx)

	var bankName *string
	if strings.TrimSpace(input.BankName) != "" {
		bn := strings.TrimSpace(input.BankName)
		bankName = &bn
	}
	var bankAccount *string
	if strings.TrimSpace(input.BankAccount) != "" {
		ba := strings.TrimSpace(input.BankAccount)
		bankAccount = &ba
	}
	var bankAccountName *string
	if strings.TrimSpace(input.BankAccountName) != "" {
		ban := strings.ToUpper(strings.TrimSpace(input.BankAccountName))
		bankAccountName = &ban
	}

	createdReturn, err := s.orderReturnRepo.Create(reqCtx, tx, sqlc.CreateOrderReturnParams{
		ReturnOrderID:         order.OrderID,
		ReturnReason:          input.Reason,
		ReturnUserID:          user.UserID,
		ReturnImages:          imageURLs,
		ReturnVideoUrl:        videoURL,
		ReturnBankName:        bankName,
		ReturnBankAccount:     bankAccount,
		ReturnBankAccountName: bankAccountName,
	})
	if err != nil {
		return v1dto.OrderReturnDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to create return request", err)
	}

	eventNote := fmt.Sprintf("Return request created by user. Reason: %s", input.Reason)
	if bankName != nil && bankAccount != nil {
		eventNote += fmt.Sprintf(" (Refund Bank: %s - %s)", *bankName, *bankAccount)
	}
	_, err = s.trackingRepo.InsertEvent(reqCtx, tx, sqlc.InsertTrackingEventParams{
		EventOrderID: order.OrderID,
		EventStatus:  "return_requested",
		EventNote:    &eventNote,
		EventSource:  "user",
	})
	if err != nil {
		return v1dto.OrderReturnDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to record return tracking event", err)
	}

	if err := tx.Commit(reqCtx); err != nil {
		return v1dto.OrderReturnDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	_ = s.cache.Clear(fmt.Sprintf("order:%s", orderUUID.String()))

	go func() {
		_ = s.notificationService.Send(context.Background(), v1dto.SendNotificationInput{
			UserID: order.OrderUserID,
			Type:   "return_requested",
			Title:  "Return Request Received",
			Body:   fmt.Sprintf("Your return request for order #%s is under review.", order.OrderNumber),
			Data: map[string]any{
				"order_uuid":  orderUUID.String(),
				"return_uuid": createdReturn.ReturnUuid.String(),
			},
		})
	}()

	return v1dto.OrderReturnDTO{
		ReturnUUID:      createdReturn.ReturnUuid.String(),
		OrderUUID:       orderUUID.String(),
		OrderNumber:     order.OrderNumber,
		Reason:          createdReturn.ReturnReason,
		Images:          createdReturn.ReturnImages,
		VideoURL:        createdReturn.ReturnVideoUrl,
		Status:          v1dto.MapReturnStatusToString(createdReturn.ReturnStatus),
		BankName:        createdReturn.ReturnBankName,
		BankAccount:     createdReturn.ReturnBankAccount,
		BankAccountName: createdReturn.ReturnBankAccountName,
		CreatedAt:       createdReturn.ReturnCreatedAt,
	}, nil
}

func (s *orderService) AdminGetReturns(ctx *gin.Context, query v1dto.AdminGetReturnsQuery) ([]v1dto.AdminReturnSummaryDTO, int64, error) {
	reqCtx := ctx.Request.Context()

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

	total, err := s.orderReturnRepo.CountReturns(reqCtx, sqlc.AdminCountReturnsParams{
		Status: query.Status,
		Search: search,
	})
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to count returns requests", err)
	}

	rows, err := s.orderReturnRepo.GetReturns(reqCtx, sqlc.AdminGetReturnsParams{
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
			ReturnUUID:    r.ReturnUuid.String(),
			OrderUUID:     r.OrderUuid.String(),
			OrderNumber:   r.OrderNumber,
			CustomerName:  r.OrderShippingName,
			CustomerPhone: r.OrderShippingPhone,
			CustomerEmail: r.UserEmail,
			TotalAmount:   utils.NumericToFloat(r.OrderTotalAmount),
			PaymentMethod: v1dto.MapPaymentMethodToString(r.OrderPaymentMethod),
			PaymentStatus: v1dto.MapPaymentStatusToString(r.OrderPaymentStatus),
			Reason:        r.ReturnReason,
			Images:        r.ReturnImages,
			VideoURL:      r.ReturnVideoUrl,
			Status:        r.ReturnStatus,
			StatusText:    v1dto.MapReturnStatusToVietnamese(r.ReturnStatus),
			AdminNote:     r.ReturnAdminNote,
			CreatedAt:     r.ReturnCreatedAt,
			UpdatedAt:     r.ReturnUpdatedAt,
		}
	}
	return result, total, nil
}

func (s *orderService) AdminGetReturnDetail(ctx *gin.Context, returnUUID uuid.UUID) (v1dto.AdminReturnDetailDTO, error) {
	reqCtx := ctx.Request.Context()

	user, err := s.getUser(ctx)
	if err != nil {
		return v1dto.AdminReturnDetailDTO{}, err
	}

	if user.UserLevel != RoleAdmin && user.UserLevel != RoleModerator {
		return v1dto.AdminReturnDetailDTO{}, utils.NewError(utils.ErrCodeForbidden, "Admin or Moderator access required")
	}

	var returnRow sqlc.AdminGetReturnByUUIDRow
	returnByUUID, err := s.orderReturnRepo.AdminGetReturnByUUID(reqCtx, returnUUID)
	if err == nil {
		returnRow = returnByUUID
	} else if errors.Is(err, pgx.ErrNoRows) {
		// Fallback: check if the provided UUID is an order UUID
		orderByUUID, oErr := s.orderRepo.GetByUUID(reqCtx, returnUUID)
		if oErr != nil {
			if errors.Is(oErr, pgx.ErrNoRows) {
				return v1dto.AdminReturnDetailDTO{}, utils.NewError(utils.ErrCodeNotFound, "Return request or order not found")
			}
			return v1dto.AdminReturnDetailDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order", oErr)
		}
		latestReturn, lErr := s.orderReturnRepo.AdminGetLatestReturnByOrderID(reqCtx, orderByUUID.OrderID)
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

	items, err := s.orderItemRepo.GetByOrderID(reqCtx, returnRow.OrderID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return v1dto.AdminReturnDetailDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order items", err)
	}

	itemsDTO := make([]v1dto.OrderItemDTO, len(items))
	for i, item := range items {
		itemsDTO[i] = v1dto.MapToOrderItemRowDTO(item)
	}

	events, err := s.trackingRepo.GetByOrderID(reqCtx, returnRow.OrderID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return v1dto.AdminReturnDetailDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get tracking events", err)
	}

	returnEvents := make([]v1dto.TrackingEventDTO, 0)
	for _, ev := range events {
		if strings.HasPrefix(ev.EventStatus, "return_") || ev.EventStatus == "returned" || ev.EventStatus == "refunded" {
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
		Status:           returnRow.ReturnStatus,
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
			FullName: returnRow.UserFullname,
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

func (s *orderService) AdminUpdateReturnStatus(ctx *gin.Context, returnUUID uuid.UUID, input v1dto.AdminUpdateReturnInput) (v1dto.AdminReturnSummaryDTO, error) {
	reqCtx := ctx.Request.Context()

	returnRequest, err := s.orderReturnRepo.GetByUUID(reqCtx, returnUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v1dto.AdminReturnSummaryDTO{}, utils.NewError(utils.ErrCodeNotFound, "Return request not found")
		}
		return v1dto.AdminReturnSummaryDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get return request", err)
	}

	if returnRequest.ReturnStatus == OrderReturnStatusCompleted || returnRequest.ReturnStatus == OrderReturnStatusRejected {
		return v1dto.AdminReturnSummaryDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Cannot update a completed or rejected return request")
	}

	if input.Status == OrderReturnStatusCompleted && returnRequest.ReturnStatus != 2 {
		return v1dto.AdminReturnSummaryDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Return request must be in 'Approved' status before marking as 'Completed'")
	}

	order, err := s.orderRepo.GetByID(reqCtx, returnRequest.ReturnOrderID)
	if err != nil {
		return v1dto.AdminReturnSummaryDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order for return request", err)
	}

	txCtx, cancel := context.WithTimeout(reqCtx, 10*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return v1dto.AdminReturnSummaryDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to begin transaction", err)
	}
	defer tx.Rollback(txCtx)

	updatedReturn, err := s.orderReturnRepo.UpdateStatus(reqCtx, tx, sqlc.UpdateReturnStatusParams{
		ReturnUuid: returnUUID,
		Status:     input.Status,
		AdminNote:  input.AdminNote,
	})
	if err != nil {
		return v1dto.AdminReturnSummaryDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update return request status", err)
	}

	noteStr := ""
	if input.AdminNote != nil && strings.TrimSpace(*input.AdminNote) != "" {
		noteStr = *input.AdminNote
	}

	switch input.Status {
	case OrderReturnStatusApproved:
		eventNote := fmt.Sprintf("Request approved. Please ship the package back to the warehouse. Note: %s", noteStr)
		_, _ = s.trackingRepo.InsertEvent(reqCtx, tx, sqlc.InsertTrackingEventParams{
			EventOrderID: returnRequest.ReturnOrderID,
			EventStatus:  "return_approved",
			EventNote:    &eventNote,
			EventSource:  "admin",
		})
	case OrderReturnStatusRejected:
		eventNote := fmt.Sprintf("Request rejected. Reason: %s", noteStr)
		_, _ = s.trackingRepo.InsertEvent(reqCtx, tx, sqlc.InsertTrackingEventParams{
			EventOrderID: returnRequest.ReturnOrderID,
			EventStatus:  "return_rejected",
			EventNote:    &eventNote,
			EventSource:  "admin",
		})
	case OrderReturnStatusCompleted:
		_, err := s.orderRepo.UpdateStatus(txCtx, sqlc.UpdateOrderStatusParams{
			OrderUuid:   order.OrderUuid,
			OrderStatus: OrderStatusReturned,
		})
		if err != nil {
			return v1dto.AdminReturnSummaryDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update order status to Returned", err)
		}

		// Restock
		orderItems, err := s.orderItemRepo.GetByOrderID(txCtx, order.OrderID)
		if err == nil {
			for _, item := range orderItems {
				_ = s.productRepo.RestoreStock(txCtx, tx, item.OrderItemProductID, item.OrderItemQuantity)
			}
		}

		eventNote := fmt.Sprintf("Warehouse received returned goods successfully and restored inventory. Awaiting refund processing. Note: %s", noteStr)
		_, _ = s.trackingRepo.InsertEvent(reqCtx, tx, sqlc.InsertTrackingEventParams{
			EventOrderID: returnRequest.ReturnOrderID,
			EventStatus:  "returned",
			EventNote:    &eventNote,
			EventSource:  "admin",
		})
	}

	if err := tx.Commit(txCtx); err != nil {
		return v1dto.AdminReturnSummaryDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	_ = s.cache.Clear(fmt.Sprintf("order:%s", order.OrderUuid.String()))

	go func() {
		var notifType, notifTitle, notifBody string
		switch updatedReturn.ReturnStatus {
		case OrderReturnStatusApproved:
			notifType = "return_approved"
			notifTitle = "Return Request Approved"
			notifBody = fmt.Sprintf("Your return request for order #%s has been approved. Please submit the return tracking code.", order.OrderNumber)
		case OrderReturnStatusRejected:
			notifType = "return_rejected"
			notifTitle = "Return Request Rejected"
			notifBody = fmt.Sprintf("Your return request for order #%s was rejected. Reason: %s", order.OrderNumber, noteStr)
		default:
			return
		}

		_ = s.notificationService.Send(context.Background(), v1dto.SendNotificationInput{
			UserID: order.OrderUserID,
			Type:   notifType,
			Title:  notifTitle,
			Body:   notifBody,
			Data: map[string]any{
				"order_uuid":  order.OrderUuid.String(),
				"return_uuid": updatedReturn.ReturnUuid.String(),
			},
		})
	}()

	return v1dto.AdminReturnSummaryDTO{
		ReturnUUID: updatedReturn.ReturnUuid.String(),
		Status:     updatedReturn.ReturnStatus,
		StatusText: v1dto.MapReturnStatusToVietnamese(updatedReturn.ReturnStatus),
		AdminNote:  updatedReturn.ReturnAdminNote,
		UpdatedAt:  updatedReturn.ReturnUpdatedAt,
	}, nil
}

type resolvedOrderReturn struct {
	Return           sqlc.OrderReturn
	OrderID          int32
	OrderUUID        uuid.UUID
	OrderNumber      string
	OrderUserID      int32
	OrderTotalAmount float64
}

// resolveOrderReturn resolves the associated return request and order info given either an order UUID or a return request UUID.
func (s *orderService) resolveOrderReturn(ctx context.Context, identifier uuid.UUID) (*resolvedOrderReturn, error) {
	var res resolvedOrderReturn

	// 1. Check if identifier is an Order UUID
	orderByUUID, err := s.orderRepo.GetByUUID(ctx, identifier)
	if err == nil {
		res.OrderID = orderByUUID.OrderID
		res.OrderUUID = orderByUUID.OrderUuid
		res.OrderNumber = orderByUUID.OrderNumber
		res.OrderUserID = orderByUUID.OrderUserID
		res.OrderTotalAmount = utils.NumericToFloat(orderByUUID.OrderTotalAmount)

		returnReq, rErr := s.orderReturnRepo.GetLatestByOrderID(ctx, res.OrderID)
		if rErr != nil {
			if errors.Is(rErr, pgx.ErrNoRows) {
				return nil, utils.NewError(utils.ErrCodeNotFound, "No return request found for this order")
			}
			return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to get return request", rErr)
		}
		res.Return = returnReq
		return &res, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
	}

	// 2. Fallback: Check if identifier is a Return Request UUID
	returnByUUID, rErr := s.orderReturnRepo.GetByUUID(ctx, identifier)
	if rErr != nil {
		if errors.Is(rErr, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "Order or return request not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to get return request", rErr)
	}
	res.Return = returnByUUID

	orderRow, oErr := s.orderRepo.GetByID(ctx, returnByUUID.ReturnOrderID)
	if oErr != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to get order for return request", oErr)
	}
	res.OrderID = orderRow.OrderID
	res.OrderUUID = orderRow.OrderUuid
	res.OrderNumber = orderRow.OrderNumber
	res.OrderUserID = orderRow.OrderUserID
	res.OrderTotalAmount = utils.NumericToFloat(orderRow.OrderTotalAmount)

	return &res, nil
}

func isReturnAccessibleByUser(user sqlc.User, res *resolvedOrderReturn) bool {
	if user.UserLevel == RoleAdmin || user.UserLevel == RoleModerator {
		return true
	}
	return user.UserID == res.OrderUserID || user.UserID == res.Return.ReturnUserID
}

func (s *orderService) GetOrderReturnDetails(ctx *gin.Context, orderUUID uuid.UUID) (v1dto.OrderReturnDetailDTO, error) {
	reqCtx := ctx.Request.Context()

	user, err := s.getUser(ctx)
	if err != nil {
		return v1dto.OrderReturnDetailDTO{}, err
	}

	resolved, err := s.resolveOrderReturn(reqCtx, orderUUID)
	if err != nil {
		return v1dto.OrderReturnDetailDTO{}, err
	}

	if !isReturnAccessibleByUser(user, resolved) {
		return v1dto.OrderReturnDetailDTO{}, utils.NewError(utils.ErrCodeForbidden, "Access denied")
	}

	events, err := s.trackingRepo.GetByOrderID(reqCtx, resolved.OrderID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return v1dto.OrderReturnDetailDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get tracking events", err)
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

	return v1dto.OrderReturnDetailDTO{
		ReturnUUID:       resolved.Return.ReturnUuid.String(),
		OrderUUID:        resolved.OrderUUID.String(),
		OrderNumber:      resolved.OrderNumber,
		Reason:           resolved.Return.ReturnReason,
		Images:           resolved.Return.ReturnImages,
		VideoURL:         resolved.Return.ReturnVideoUrl,
		Status:           resolved.Return.ReturnStatus,
		StatusName:       v1dto.MapReturnStatusToString(resolved.Return.ReturnStatus),
		StatusText:       v1dto.MapReturnStatusToVietnamese(resolved.Return.ReturnStatus),
		AdminNote:        resolved.Return.ReturnAdminNote,
		ShippingProvider: resolved.Return.ReturnShippingProvider,
		ShippingCode:     resolved.Return.ReturnShippingCode,
		ShippingNote:     resolved.Return.ReturnShippingNote,
		BankName:         resolved.Return.ReturnBankName,
		BankAccount:      resolved.Return.ReturnBankAccount,
		BankAccountName:  resolved.Return.ReturnBankAccountName,
		TotalAmount:      resolved.OrderTotalAmount,
		CreatedAt:        resolved.Return.ReturnCreatedAt,
		UpdatedAt:        resolved.Return.ReturnUpdatedAt,
		Events:           returnEvents,
	}, nil
}

func (s *orderService) GetMyOrderReturns(ctx *gin.Context, query v1dto.GetMyOrderReturnsQuery) ([]v1dto.UserReturnSummaryDTO, int64, error) {
	reqCtx := ctx.Request.Context()

	user, err := s.getUser(ctx)
	if err != nil {
		return nil, 0, err
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

	total, err := s.orderReturnRepo.CountReturnsByUserID(reqCtx, sqlc.CountReturnsByUserIDParams{
		UserID: user.UserID,
		Status: query.Status,
		Search: search,
	})
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to count return requests", err)
	}

	if total == 0 {
		return []v1dto.UserReturnSummaryDTO{}, 0, nil
	}

	rows, err := s.orderReturnRepo.GetReturnsByUserID(reqCtx, sqlc.GetReturnsByUserIDParams{
		UserID: user.UserID,
		Status: query.Status,
		Search: search,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to get return requests", err)
	}

	result := make([]v1dto.UserReturnSummaryDTO, len(rows))
	for i, r := range rows {
		result[i] = v1dto.UserReturnSummaryDTO{
			ReturnUUID:       r.ReturnUuid.String(),
			OrderUUID:        r.OrderUuid.String(),
			OrderNumber:      r.OrderNumber,
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

func (s *orderService) CancelOrderReturn(ctx *gin.Context, orderUUID uuid.UUID, input v1dto.CancelOrderReturnInput) (v1dto.CancelOrderReturnResponseDTO, error) {
	reqCtx := ctx.Request.Context()

	user, err := s.getUser(ctx)
	if err != nil {
		return v1dto.CancelOrderReturnResponseDTO{}, err
	}

	resolved, err := s.resolveOrderReturn(reqCtx, orderUUID)
	if err != nil {
		return v1dto.CancelOrderReturnResponseDTO{}, err
	}

	if !isReturnAccessibleByUser(user, resolved) {
		return v1dto.CancelOrderReturnResponseDTO{}, utils.NewError(utils.ErrCodeForbidden, "Access denied")
	}

	returnRequest := resolved.Return
	if returnRequest.ReturnStatus == OrderReturnStatusCancelled {
		return v1dto.CancelOrderReturnResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Return request is already cancelled")
	}

	if returnRequest.ReturnStatus != OrderReturnStatusPending {
		return v1dto.CancelOrderReturnResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Only pending return requests can be cancelled, current: %s", v1dto.MapReturnStatusToVietnamese(returnRequest.ReturnStatus)))
	}

	txCtx, cancel := context.WithTimeout(reqCtx, 10*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return v1dto.CancelOrderReturnResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to begin transaction", err)
	}
	defer tx.Rollback(txCtx)

	note := "User cancelled the return request."
	if strings.TrimSpace(input.Reason) != "" {
		note += fmt.Sprintf(" Reason: %s", strings.TrimSpace(input.Reason))
	}

	updatedReturn, err := s.orderReturnRepo.UpdateStatus(reqCtx, tx, sqlc.UpdateReturnStatusParams{
		ReturnUuid: returnRequest.ReturnUuid,
		Status:     OrderReturnStatusCancelled,
		AdminNote:  &note,
	})
	if err != nil {
		return v1dto.CancelOrderReturnResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update return request status", err)
	}

	_, err = s.trackingRepo.InsertEvent(reqCtx, tx, sqlc.InsertTrackingEventParams{
		EventOrderID: returnRequest.ReturnOrderID,
		EventStatus:  "return_cancelled",
		EventNote:    &note,
		EventSource:  "user",
	})
	if err != nil {
		return v1dto.CancelOrderReturnResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to record return cancellation tracking event", err)
	}

	if err := tx.Commit(txCtx); err != nil {
		return v1dto.CancelOrderReturnResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	_ = s.cache.Clear(fmt.Sprintf("order:%s", resolved.OrderUUID.String()))

	return v1dto.CancelOrderReturnResponseDTO{
		ReturnUUID:   updatedReturn.ReturnUuid.String(),
		OrderUUID:    resolved.OrderUUID.String(),
		OrderNumber:  resolved.OrderNumber,
		Status:       updatedReturn.ReturnStatus,
		StatusName:   v1dto.MapReturnStatusToString(updatedReturn.ReturnStatus),
		StatusText:   v1dto.MapReturnStatusToVietnamese(updatedReturn.ReturnStatus),
		CancelReason: input.Reason,
		UpdatedAt:    updatedReturn.ReturnUpdatedAt,
		Message:      "Return request cancelled successfully.",
	}, nil
}

func (s *orderService) SubmitReturnShipping(ctx *gin.Context, orderUUID uuid.UUID, input v1dto.SubmitReturnShippingInput) (v1dto.SubmitReturnShippingResponseDTO, error) {
	reqCtx := ctx.Request.Context()

	user, err := s.getUser(ctx)
	if err != nil {
		return v1dto.SubmitReturnShippingResponseDTO{}, err
	}

	resolved, err := s.resolveOrderReturn(reqCtx, orderUUID)
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

	txCtx, cancel := context.WithTimeout(reqCtx, 10*time.Second)
	defer cancel()
	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return v1dto.SubmitReturnShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to begin transaction", err)
	}
	defer tx.Rollback(txCtx)
	updatedReturn, err := s.orderReturnRepo.UpdateReturnShippingInfo(txCtx, tx, sqlc.UpdateReturnShippingInfoParams{
		ReturnShippingProvider: &provider,
		ReturnShippingCode:     &shippingCode,
		ReturnShippingNote:     note,
		ReturnUuid:             returnRequest.ReturnUuid,
	})
	if err != nil {
		return v1dto.SubmitReturnShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update return shipping info", err)
	}

	eventNote := fmt.Sprintf("User submitted return shipping info for %s (Tracking Code: %s).", strings.ToUpper(provider), shippingCode)
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
		return v1dto.SubmitReturnShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to record tracking event", err)
	}
	if err := tx.Commit(txCtx); err != nil {
		return v1dto.SubmitReturnShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	_ = s.cache.Clear(fmt.Sprintf("order:%s", resolved.OrderUUID.String()))
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

func (s *orderService) UpdateReturnBankAccount(ctx *gin.Context, orderUUID uuid.UUID, input v1dto.UpdateReturnBankAccountInput) (v1dto.UpdateReturnBankAccountResponseDTO, error) {
	reqCtx := ctx.Request.Context()

	user, err := s.getUser(ctx)
	if err != nil {
		return v1dto.UpdateReturnBankAccountResponseDTO{}, err
	}

	resolved, err := s.resolveOrderReturn(reqCtx, orderUUID)
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

	txCtx, cancel := context.WithTimeout(reqCtx, 10*time.Second)
	defer cancel()
	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return v1dto.UpdateReturnBankAccountResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to begin transaction", err)
	}
	defer tx.Rollback(txCtx)

	updatedReturn, err := s.orderReturnRepo.UpdateReturnBankAccount(txCtx, tx, sqlc.UpdateReturnBankAccountParams{
		ReturnBankName:        &bankName,
		ReturnBankAccount:     &bankAccount,
		ReturnBankAccountName: &bankAccountName,
		ReturnUuid:            returnRequest.ReturnUuid,
	})
	if err != nil {
		return v1dto.UpdateReturnBankAccountResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update return bank account", err)
	}

	eventNote := fmt.Sprintf("Customer updated refund bank account: %s - %s (%s)", bankName, bankAccount, bankAccountName)
	_, _ = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
		EventOrderID: returnRequest.ReturnOrderID,
		EventStatus:  "return_bank_updated",
		EventNote:    &eventNote,
		EventSource:  "user",
	})

	if err := tx.Commit(txCtx); err != nil {
		return v1dto.UpdateReturnBankAccountResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	_ = s.cache.Clear(fmt.Sprintf("order:%s", resolved.OrderUUID.String()))
	_ = s.cache.Clear("orders:*")

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

func (s *orderService) AdminProcessReturnRefund(ctx *gin.Context, returnUUID uuid.UUID, input v1dto.AdminReturnRefundInput) (v1dto.AdminReturnRefundResponseDTO, error) {
	reqCtx := ctx.Request.Context()

	user, err := s.getUser(ctx)
	if err != nil {
		return v1dto.AdminReturnRefundResponseDTO{}, err
	}

	if user.UserLevel != RoleAdmin && user.UserLevel != RoleModerator {
		return v1dto.AdminReturnRefundResponseDTO{}, utils.NewError(utils.ErrCodeForbidden, "Admin or Moderator access required")
	}

	resolved, err := s.resolveOrderReturn(reqCtx, returnUUID)
	if err != nil {
		return v1dto.AdminReturnRefundResponseDTO{}, err
	}

	returnRequest := resolved.Return

	if returnRequest.ReturnStatus != OrderReturnStatusCompleted && returnRequest.ReturnStatus != OrderReturnStatusApproved {
		return v1dto.AdminReturnRefundResponseDTO{}, utils.NewError(
			utils.ErrCodeBadRequest,
			fmt.Sprintf("Refund can only be processed when return request is Approved or Completed. Current status: %s",
				v1dto.MapReturnStatusToString(returnRequest.ReturnStatus)),
		)
	}

	txCtx, cancel := context.WithTimeout(reqCtx, 15*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return v1dto.AdminReturnRefundResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to begin transaction", err)
	}
	defer tx.Rollback(txCtx)

	order, err := s.orderRepo.GetByUUIDForUpdate(txCtx, tx, resolved.OrderUUID)
	if err != nil {
		return v1dto.AdminReturnRefundResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to lock order for refund", err)
	}

	if order.OrderPaymentStatus != OrderPaymentStatusPaid && order.OrderPaymentStatus != OrderPaymentStatusPartiallyRefunded {
		return v1dto.AdminReturnRefundResponseDTO{}, utils.NewError(
			utils.ErrCodeBadRequest,
			fmt.Sprintf("Order is unpaid or not in an eligible state for refund. Current payment status: %s",
				v1dto.MapPaymentStatusToString(order.OrderPaymentStatus)),
		)
	}

	totalAmount := utils.NumericToFloat(order.OrderTotalAmount)
	currentRefunded := utils.NumericToFloat(order.OrderRefundedAmount)
	remainingRefundable := totalAmount - currentRefunded

	if remainingRefundable <= 0 {
		return v1dto.AdminReturnRefundResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest, "This order has already been fully refunded")
	}

	refundAmount := remainingRefundable
	if input.Amount != nil && *input.Amount > 0 {
		if *input.Amount > remainingRefundable {
			return v1dto.AdminReturnRefundResponseDTO{}, utils.NewError(
				utils.ErrCodeBadRequest,
				fmt.Sprintf("Refund amount (%.2f) exceeds maximum refundable balance (%.2f)", *input.Amount, remainingRefundable),
			)
		}
		refundAmount = *input.Amount
	}

	reason := input.Reason
	if strings.TrimSpace(reason) == "" {
		reason = fmt.Sprintf("Refund for return request: %s", returnRequest.ReturnReason)
	}

	var refundTxnID string
	providerName := "cod"

	txns, err := s.paymentRepo.GetByOrderID(txCtx, tx, order.OrderID)
	if err != nil {
		return v1dto.AdminReturnRefundResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get payment history", err)
	}

	var originalTxn *sqlc.PaymentTransaction
	for _, t := range txns {
		if t.PtStatus == PaymentStatusPaid && t.PtRefundOf == nil {
			originalTxn = &t
			break
		}
	}

	if order.OrderPaymentMethod != OrderPaymentMethodCOD && originalTxn != nil && originalTxn.PtProvider != "cod" {
		providerName = originalTxn.PtProvider
		provider, exists := s.paymentProviders[providerName]
		if !exists {
			return v1dto.AdminReturnRefundResponseDTO{}, utils.NewError(
				utils.ErrCodeBadRequest,
				fmt.Sprintf("Payment provider '%s' does not support automated refund API. Please process refund via manual bank transfer.", providerName),
			)
		}

		origProviderTxnID := ""
		if originalTxn.PtProviderTxnID != nil {
			origProviderTxnID = *originalTxn.PtProviderTxnID
		}

		refundResp, gErr := provider.Refund(txCtx, payment.RefundRequest{
			OrderNumber:   order.OrderNumber,
			ProviderTxnID: origProviderTxnID,
			Amount:        refundAmount,
			TotalAmount:   totalAmount,
			Reason:        reason,
		})
		if gErr != nil {
			logger.Log.Error().Err(gErr).Str("provider", providerName).Msg("Gateway refund failed")
			return v1dto.AdminReturnRefundResponseDTO{}, utils.WrapError(
				utils.ErrCodeBadGateway,
				fmt.Sprintf("Payment gateway %s rejected refund request: %v", strings.ToUpper(providerName), gErr),
				gErr,
			)
		}
		refundTxnID = refundResp.RefundTxnID
	} else {
		providerName = "bank_transfer"
		if strings.TrimSpace(input.BankTxnRef) != "" {
			refundTxnID = strings.TrimSpace(input.BankTxnRef)
		} else {
			refundTxnID = fmt.Sprintf("REFUND_BANK_%s_%d", order.OrderNumber, time.Now().Unix())
		}
	}

	idempotencyKey := fmt.Sprintf("refund:return:%s:%.0f:%d", returnRequest.ReturnUuid.String(), refundAmount, time.Now().Unix())
	refundNumeric := utils.FloatToNumeric(refundAmount)
	var origPtID *int32
	if originalTxn != nil {
		origPtID = &originalTxn.PtID
	}

	_, err = s.paymentRepo.CreateRefund(txCtx, tx, sqlc.CreateRefundTransactionParams{
		OrderID:        order.OrderID,
		IdempotencyKey: idempotencyKey,
		Provider:       providerName,
		ProviderTxnID:  &refundTxnID,
		Amount:         refundNumeric,
		Currency:       "VND",
		Status:         PaymentStatusPaid,
		RefundOf:       origPtID,
		RefundAmount:   refundNumeric,
	})
	if err != nil {
		return v1dto.AdminReturnRefundResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to create refund transaction", err)
	}

	newTotalRefunded := currentRefunded + refundAmount
	var newOrderStatus int32 = OrderPaymentStatusPartiallyRefunded
	if newTotalRefunded >= totalAmount {
		newOrderStatus = OrderPaymentStatusFullyRefunded
	}

	_, err = s.orderRepo.UpdateOrderPaymentStatus(txCtx, tx, sqlc.UpdateOrderPaymentStatusParams{
		OrderUuid:           order.OrderUuid,
		PaymentStatus:       newOrderStatus,
		RefundedAmount:      utils.FloatToNumeric(newTotalRefunded),
		AllowedFromStatuses: []int32{OrderPaymentStatusPaid, OrderPaymentStatusPartiallyRefunded},
	})
	if err != nil {
		return v1dto.AdminReturnRefundResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update order payment status", err)
	}

	eventNote := fmt.Sprintf("Refund processed successfully: %.0f VND via %s. Txn ID: %s.",
		refundAmount, strings.ToUpper(providerName), refundTxnID)
	if strings.TrimSpace(input.AccountingNote) != "" {
		eventNote += fmt.Sprintf(" Note: %s", strings.TrimSpace(input.AccountingNote))
	}

	_, err = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
		EventOrderID: order.OrderID,
		EventStatus:  "refunded",
		EventNote:    &eventNote,
		EventSource:  "admin",
	})
	if err != nil {
		return v1dto.AdminReturnRefundResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to record refund tracking event", err)
	}

	if err := tx.Commit(txCtx); err != nil {
		return v1dto.AdminReturnRefundResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	_ = s.cache.Clear(fmt.Sprintf("order:%s", order.OrderUuid.String()))
	_ = s.cache.Clear("orders:*")

	go func() {
		_ = s.notificationService.Send(context.Background(), v1dto.SendNotificationInput{
			UserID: order.OrderUserID,
			Type:   "return_refunded",
			Title:  "Refund Completed",
			Body:   fmt.Sprintf("Refund of %.0f VND for order #%s has been processed successfully.", refundAmount, order.OrderNumber),
			Data: map[string]any{
				"order_uuid":  order.OrderUuid.String(),
				"return_uuid": returnRequest.ReturnUuid.String(),
			},
		})
	}()

	logger.Log.Info().
		Str("return_uuid", returnRequest.ReturnUuid.String()).
		Str("order_number", order.OrderNumber).
		Float64("refund_amount", refundAmount).
		Str("refund_txn_id", refundTxnID).
		Msg("Admin processed return refund successfully")

	return v1dto.AdminReturnRefundResponseDTO{
		ReturnUUID:       returnRequest.ReturnUuid.String(),
		OrderUUID:        order.OrderUuid.String(),
		OrderNumber:      order.OrderNumber,
		RefundAmount:     refundAmount,
		TotalRefunded:    newTotalRefunded,
		OrderTotalAmount: totalAmount,
		PaymentMethod:    v1dto.MapPaymentMethodToString(order.OrderPaymentMethod),
		PaymentStatus:    v1dto.MapPaymentStatusToString(newOrderStatus),
		RefundTxnID:      refundTxnID,
		RefundProvider:   providerName,
		RefundedAt:       time.Now(),
		Message:          "Return refund processed successfully.",
	}, nil
}
