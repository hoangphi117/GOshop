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
	"goshop/pkg/rabbitmq"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

type orderReturnService struct {
	txManager           db.TxManager
	orderRepo           OrderStorePort
	orderItemRepo       OrderItemStorePort
	orderReturnRepo     OrderReturnStorePort
	stockPort           OrderStockPort
	userRepo            OrderCustomerReader
	trackingRepo        OrderTrackingPort
	paymentService      OrderPaymentRefundPort
	outboxStore         OrderOutboxPort
	cache               cache.RedisCacheService
}

// OrderReturnServiceDeps groups the dependencies required by OrderReturnService.
type OrderReturnServiceDeps struct {
	TxManager       db.TxManager
	OrderRepo       OrderStorePort
	OrderItemRepo   OrderItemStorePort
	OrderReturnRepo OrderReturnStorePort
	StockPort       OrderStockPort
	UserRepo        OrderCustomerReader
	TrackingRepo    OrderTrackingPort
	PaymentService  OrderPaymentRefundPort
	OutboxStore     OrderOutboxPort
	RedisClient     *redis.Client
	CacheService    cache.RedisCacheService
}

func NewOrderReturnService(deps OrderReturnServiceDeps) OrderReturnService {
	cacheService := deps.CacheService
	if cacheService == nil && deps.RedisClient != nil {
		cacheService = cache.NewRedisCacheService(deps.RedisClient)
	}
	return &orderReturnService{
		txManager:       deps.TxManager,
		orderRepo:       deps.OrderRepo,
		orderItemRepo:   deps.OrderItemRepo,
		orderReturnRepo: deps.OrderReturnRepo,
		stockPort:       deps.StockPort,
		userRepo:        deps.UserRepo,
		trackingRepo:    deps.TrackingRepo,
		paymentService:  deps.PaymentService,
		outboxStore:     deps.OutboxStore,
		cache:           cacheService,
	}
}

func (s *orderReturnService) handleUserNotFoundError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return utils.NewError(utils.ErrCodeNotFound, "User not found")
	}
	return utils.WrapError(utils.ErrCodeInternal, "Failed to get user", err)
}

func (s *orderReturnService) getCustomer(ctx context.Context, userUUID uuid.UUID) (*OrderCustomer, error) {
	if cust, ok := GetOrderCustomerFromContext(ctx); ok && cust.UserUUID == userUUID && cust.UserID > 0 {
		return cust, nil
	}
	if s.userRepo == nil {
		return nil, utils.NewError(utils.ErrCodeNotFound, "User not found")
	}
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		return nil, s.handleUserNotFoundError(err)
	}
	return user, nil
}

func (s *orderReturnService) handleOrderNotFoundError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return utils.NewError(utils.ErrCodeNotFound, "Order not found")
	}
	return utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
}

func (s *orderReturnService) RequestOrderReturn(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.CreateOrderReturnInput, imageURLs []string, videoURL *string) (v1dto.OrderReturnDTO, error) {
	user, err := s.getCustomer(ctx, userUUID)
	if err != nil {
		return v1dto.OrderReturnDTO{}, err
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

		if s.outboxStore != nil {
			event := rabbitmq.NewOrderReturnRequestedEvent(
				uuid.NewString(),
				createdReturn.ReturnUuid.String(),
				order.OrderUuid.String(),
				order.OrderNumber,
				createdReturn.ReturnID,
				order.OrderID,
				order.OrderUserID,
				createdReturn.ReturnReason,
			)
			_, _ = s.outboxStore.Insert(ctx, tx, "order", order.OrderUuid.String(), rabbitmq.QueueOrderReturnRequested, event)
		}
		return nil
	})
	if err != nil {
		return v1dto.OrderReturnDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to create return request", err)
	}

	if s.cache != nil {
		_ = s.cache.Clear(fmt.Sprintf("order:%s", orderUUID.String()))
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

func isReturnAccessibleByUser(user *OrderCustomer, res *resolvedOrderReturn) bool {
	if user.UserLevel == RoleAdmin || user.UserLevel == RoleModerator {
		return true
	}
	return user.UserID == res.OrderUserID || user.UserID == res.Return.ReturnUserID
}

func (s *orderReturnService) GetOrderReturnDetails(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.OrderReturnDetailDTO, error) {
	user, err := s.getCustomer(ctx, userUUID)
	if err != nil {
		return v1dto.OrderReturnDetailDTO{}, err
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
	user, err := s.getCustomer(ctx, userUUID)
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
	user, err := s.getCustomer(ctx, userUUID)
	if err != nil {
		return v1dto.CancelOrderReturnResponseDTO{}, err
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
				"Return request cancelled by customer",
			)
			_, _ = s.outboxStore.Insert(ctx, tx, "order", resolved.OrderUUID.String(), rabbitmq.QueueOrderReturnUpdated, event)
		}
		return nil
	})
	if err != nil {
		return v1dto.CancelOrderReturnResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update return request status", err)
	}

	if s.cache != nil {
		_ = s.cache.Clear(fmt.Sprintf("order:%s", resolved.OrderUUID.String()))
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
