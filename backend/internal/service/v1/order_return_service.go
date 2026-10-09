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
	"goshop/pkg/cache"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

type orderReturnService struct {
	txManager           db.TxManager
	orderRepo           OrderStorePort
	orderItemRepo       OrderItemStorePort
	orderReturnRepo     OrderReturnStorePort
	productRepo         OrderStockRestorer
	userRepo            OrderCustomerReader
	trackingRepo        OrderTrackingPort
	paymentService      OrderPaymentRefundPort
	notificationService NotificationService
	cache               cache.RedisCacheService
}

func NewOrderReturnService(
	txManager db.TxManager,
	orderRepo OrderStorePort,
	orderItemRepo OrderItemStorePort,
	orderReturnRepo OrderReturnStorePort,
	productRepo OrderStockRestorer,
	userRepo OrderCustomerReader,
	trackingRepo OrderTrackingPort,
	paymentService OrderPaymentRefundPort,
	notificationService NotificationService,
	redisClient *redis.Client,
) OrderReturnService {
	var cacheService cache.RedisCacheService
	if redisClient != nil {
		cacheService = cache.NewRedisCacheService(redisClient)
	}
	return &orderReturnService{
		txManager:           txManager,
		orderRepo:           orderRepo,
		orderItemRepo:       orderItemRepo,
		orderReturnRepo:     orderReturnRepo,
		productRepo:         productRepo,
		userRepo:            userRepo,
		trackingRepo:        trackingRepo,
		paymentService:      paymentService,
		notificationService: notificationService,
		cache:               cacheService,
	}
}

func (s *orderReturnService) handleUserNotFoundError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return utils.NewError(utils.ErrCodeNotFound, "User not found")
	}
	return utils.WrapError(utils.ErrCodeInternal, "Failed to get user", err)
}

func (s *orderReturnService) handleOrderNotFoundError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return utils.NewError(utils.ErrCodeNotFound, "Order not found")
	}
	return utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
}

func (s *orderReturnService) RequestOrderReturn(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.CreateOrderReturnInput, imageURLs []string, videoURL *string) (v1dto.OrderReturnDTO, error) {
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		return v1dto.OrderReturnDTO{}, s.handleUserNotFoundError(err)
	}

	order, err := s.orderRepo.GetByUUID(ctx, orderUUID)
	if err != nil {
		return v1dto.OrderReturnDTO{}, s.handleOrderNotFoundError(err)
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

	existingReturn, err := s.orderReturnRepo.GetActiveByOrderID(ctx, order.OrderID)
	if err == nil && existingReturn.ReturnID > 0 {
		return v1dto.OrderReturnDTO{}, utils.NewError(utils.ErrCodeConflict, "An active return request already exists for this order")
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return v1dto.OrderReturnDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to check existing return request", err)
	}

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

	var createdReturn sqlc.OrderReturn
	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		tx, _ := db.GetTx(txCtx)
		var err error
		createdReturn, err = s.orderReturnRepo.Create(txCtx, tx, sqlc.CreateOrderReturnParams{
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
			return err
		}

		if s.trackingRepo != nil {
			eventNote := fmt.Sprintf("Return request created by user. Reason: %s", input.Reason)
			if bankName != nil && bankAccount != nil {
				eventNote += fmt.Sprintf(" (Refund Bank: %s - %s)", *bankName, *bankAccount)
			}
			_, err = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
				EventOrderID: order.OrderID,
				EventStatus:  "return_requested",
				EventNote:    &eventNote,
				EventSource:  "user",
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return v1dto.OrderReturnDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to create return request", err)
	}

	if s.cache != nil {
		_ = s.cache.Clear(fmt.Sprintf("order:%s", orderUUID.String()))
	}

	if s.notificationService != nil {
		go func() {
			_ = s.notificationService.SendAsync(context.Background(), v1dto.SendNotificationInput{
				UserID: order.OrderUserID,
				Type:   "return_requested",
				Title:  "Return Request Received",
				Body:   fmt.Sprintf("Your return request for order #%s is under review.", order.OrderNumber),
				Data: map[string]any{
					"order_uuid":  orderUUID.String(),
					"return_uuid": createdReturn.ReturnUuid.String(),
				},
			})

			_ = s.notificationService.NotifyAdminsAsync(context.Background(), v1dto.AdminNotifyInput{
				Type:  "return",
				Title: "New Return Requested",
				Body:  fmt.Sprintf("Return requested for order #%s: %s", order.OrderNumber, createdReturn.ReturnReason),
				Data: map[string]any{
					"order_uuid":  orderUUID.String(),
					"return_uuid": createdReturn.ReturnUuid.String(),
					"link":        "/returns",
				},
			})
		}()
	}

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

func (s *orderReturnService) AdminGetReturns(ctx context.Context, userUUID uuid.UUID, query v1dto.AdminGetReturnsQuery) ([]v1dto.AdminReturnSummaryDTO, int64, error) {
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		return nil, 0, s.handleUserNotFoundError(err)
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

func (s *orderReturnService) AdminGetReturnDetail(ctx context.Context, userUUID uuid.UUID, returnUUID uuid.UUID) (v1dto.AdminReturnDetailDTO, error) {
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		return v1dto.AdminReturnDetailDTO{}, s.handleUserNotFoundError(err)
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

type resolvedOrderReturn struct {
	Return           sqlc.OrderReturn
	OrderID          int32
	OrderUUID        uuid.UUID
	OrderNumber      string
	OrderUserID      int32
	OrderTotalAmount float64
}

// resolveOrderReturn resolves the associated return request and order info given either an order UUID or a return request UUID.
func (s *orderReturnService) resolveOrderReturn(ctx context.Context, identifier uuid.UUID) (*resolvedOrderReturn, error) {
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

func (s *orderReturnService) GetOrderReturnDetails(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.OrderReturnDetailDTO, error) {
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		return v1dto.OrderReturnDetailDTO{}, s.handleUserNotFoundError(err)
	}

	resolved, err := s.resolveOrderReturn(ctx, orderUUID)
	if err != nil {
		return v1dto.OrderReturnDetailDTO{}, err
	}

	if !isReturnAccessibleByUser(user, resolved) {
		return v1dto.OrderReturnDetailDTO{}, utils.NewError(utils.ErrCodeForbidden, "Access denied")
	}

	events, err := s.trackingRepo.GetByOrderID(ctx, resolved.OrderID)
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

func (s *orderReturnService) GetMyOrderReturns(ctx context.Context, userUUID uuid.UUID, query v1dto.GetMyOrderReturnsQuery) ([]v1dto.UserReturnSummaryDTO, int64, error) {
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		return nil, 0, s.handleUserNotFoundError(err)
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

	total, err := s.orderReturnRepo.CountReturnsByUserID(ctx, sqlc.CountReturnsByUserIDParams{
		UserID: user.UserID,
		Status: query.Status,
		Search: search,
	})
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to count returns requests", err)
	}

	rows, err := s.orderReturnRepo.GetReturnsByUserID(ctx, sqlc.GetReturnsByUserIDParams{
		UserID: user.UserID,
		Status: query.Status,
		Search: search,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to get returns requests", err)
	}

	result := make([]v1dto.UserReturnSummaryDTO, len(rows))
	for i, r := range rows {
		result[i] = v1dto.UserReturnSummaryDTO{
			ReturnUUID:  r.ReturnUuid.String(),
			OrderUUID:   r.OrderUuid.String(),
			OrderNumber: r.OrderNumber,
			TotalAmount: utils.NumericToFloat(r.OrderTotalAmount),
			Reason:      r.ReturnReason,
			Images:      r.ReturnImages,
			VideoURL:    r.ReturnVideoUrl,
			Status:      r.ReturnStatus,
			StatusName:  v1dto.MapReturnStatusToString(r.ReturnStatus),
			StatusText:  v1dto.MapReturnStatusToVietnamese(r.ReturnStatus),
			AdminNote:   r.ReturnAdminNote,
			CreatedAt:   r.ReturnCreatedAt,
			UpdatedAt:   r.ReturnUpdatedAt,
		}
	}
	return result, total, nil
}

func (s *orderReturnService) CancelOrderReturn(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.CancelOrderReturnInput) (v1dto.CancelOrderReturnResponseDTO, error) {
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		return v1dto.CancelOrderReturnResponseDTO{}, s.handleUserNotFoundError(err)
	}

	resolved, err := s.resolveOrderReturn(ctx, orderUUID)
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

	note := "User cancelled the return request."
	if strings.TrimSpace(input.Reason) != "" {
		note += fmt.Sprintf(" Reason: %s", strings.TrimSpace(input.Reason))
	}

	var updatedReturn sqlc.OrderReturn
	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		tx, _ := db.GetTx(txCtx)
		var err error
		updatedReturn, err = s.orderReturnRepo.UpdateStatus(txCtx, tx, sqlc.UpdateReturnStatusParams{
			ReturnUuid: returnRequest.ReturnUuid,
			Status:     OrderReturnStatusCancelled,
			AdminNote:  &note,
		})
		if err != nil {
			return err
		}

		if s.trackingRepo != nil {
			_, err = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
				EventOrderID: returnRequest.ReturnOrderID,
				EventStatus:  "return_cancelled",
				EventNote:    &note,
				EventSource:  "user",
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return v1dto.CancelOrderReturnResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update return request status", err)
	}

	if s.cache != nil {
		_ = s.cache.Clear(fmt.Sprintf("order:%s", resolved.OrderUUID.String()))
	}

	if s.notificationService != nil {
		go func() {
			_ = s.notificationService.NotifyAdminsAsync(context.Background(), v1dto.AdminNotifyInput{
				Type:  "return",
				Title: "Return Request Cancelled",
				Body:  fmt.Sprintf("Customer cancelled return request for order #%s.", resolved.OrderNumber),
				Data: map[string]any{
					"order_uuid":   resolved.OrderUUID.String(),
					"return_uuid":  updatedReturn.ReturnUuid.String(),
					"order_number": resolved.OrderNumber,
					"link":         "/returns",
				},
			})
		}()
	}

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
