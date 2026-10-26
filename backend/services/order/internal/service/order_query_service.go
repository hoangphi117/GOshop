package service

import (
	"context"
	"fmt"
	v1dto "goshop/services/order/internal/dto"
	"goshop/pkg/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"strings"
	"time"

	"github.com/google/uuid"
)

type orderQueryService struct {
	orderStore     OrderStorePort
	orderItemStore OrderItemStorePort
	userReader     OrderCustomerReader
	cache          cache.RedisCacheService
	invoiceService OrderInvoiceService
}

func NewOrderQueryService(
	orderStore OrderStorePort,
	orderItemStore OrderItemStorePort,
	userReader OrderCustomerReader,
	cacheService cache.RedisCacheService,
	invoiceService OrderInvoiceService,
) OrderQueryService {
	return &orderQueryService{
		orderStore:     orderStore,
		orderItemStore: orderItemStore,
		userReader:     userReader,
		cache:          cacheService,
		invoiceService: invoiceService,
	}
}

func (s *orderQueryService) getCustomer(ctx context.Context, userUUID uuid.UUID) (*OrderCustomer, error) {
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

func (s *orderQueryService) GetMyOrders(ctx context.Context, userUUID uuid.UUID, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error) {
	user, err := s.getCustomer(ctx, userUUID)
	if err != nil {
		return []v1dto.OrderSummaryDTO{}, 0, err
	}

	if input.Sort == "" {
		input.Sort = "desc"
	}

	if input.OrderBy == "" {
		input.OrderBy = "order_created_at"
	}

	if input.Page < 1 {
		input.Page = 1
	}

	if input.Limit < 1 {
		limitInt := utils.GetIntEnv("LIMIT_ITEM_PER_PAGE", 10)
		input.Limit = int32(limitInt)
	}

	cacheKey := s.generateMyOrdersCacheKey(user.UserUuid.String(), input)
	var cacheData struct {
		Orders []v1dto.OrderSummaryDTO `json:"orders"`
		Total  int32                   `json:"total"`
	}

	if s.cache != nil {
		if err := s.cache.Get(cacheKey, &cacheData); err == nil && cacheData.Orders != nil {
			return cacheData.Orders, cacheData.Total, nil
		}
	}

	var statusPtr *int32
	if input.Status > 0 {
		statusPtr = &input.Status
	}

	totalOrders, err := s.orderStore.CountOrders(ctx, &user.UserID, statusPtr, input.Search)
	if err != nil {
		return []v1dto.OrderSummaryDTO{}, 0, utils.WrapError(utils.ErrCodeInternal, "failed to count orders", err)
	}
	if totalOrders == 0 {
		return []v1dto.OrderSummaryDTO{}, 0, nil
	}

	offset := (input.Page - 1) * input.Limit
	orders, err := s.orderStore.GetAll(ctx, &user.UserID, statusPtr, input.Search, input.OrderBy,
		input.Sort, input.Limit, offset)
	if err != nil {
		return []v1dto.OrderSummaryDTO{}, 0, utils.WrapError(utils.ErrCodeInternal, "failed to get orders", err)
	}

	ordersDTO := make([]v1dto.OrderSummaryDTO, len(orders))
	for i, o := range orders {
		ordersDTO[i] = v1dto.MapToOrderSummaryDTO(o)
		if items, err := s.orderItemStore.GetByOrderID(ctx, o.OrderID); err == nil {
			itemsDTO := make([]v1dto.OrderItemDTO, len(items))
			for j, item := range items {
				itemsDTO[j] = v1dto.MapToOrderItemRowDTO(item)
			}
			ordersDTO[i].Items = itemsDTO
		}
	}

	cacheData.Orders = ordersDTO
	cacheData.Total = int32(totalOrders)
	if s.cache != nil {
		if err := s.cache.Set(cacheKey, cacheData, 2*time.Minute); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to set user orders cache")
		}
	}

	return ordersDTO, int32(totalOrders), nil
}

func (s *orderQueryService) GetOrderByUUID(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (*v1dto.OrderDTO, error) {
	user, err := s.getCustomer(ctx, userUUID)
	if err != nil {
		return nil, err
	}

	order, err := s.orderStore.GetByUUID(ctx, orderUUID)
	if err != nil {
		return nil, handleOrderNotFoundError(err)
	}

	isAdmin := user.UserLevel == RoleAdmin
	isModerator := user.UserLevel == RoleModerator

	if order.OrderUserID != user.UserID && !isAdmin && !isModerator {
		return nil, utils.NewError(utils.ErrCodeForbidden, "You are not authorized to view this order")
	}

	orderItems, err := s.orderItemStore.GetByOrderID(ctx, order.OrderID)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get order items", err)
	}

	dto := v1dto.MapGetOrderByUUIDRowToOrderDTO(order, orderItems)
	return &dto, nil
}

func (s *orderQueryService) GetAllOrders(ctx context.Context, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error) {
	if input.Sort == "" {
		input.Sort = "desc"
	}

	if input.OrderBy == "" {
		input.OrderBy = "order_created_at"
	}

	if input.Page < 1 {
		input.Page = 1
	}

	if input.Limit < 1 {
		limitInt := utils.GetIntEnv("LIMIT_ITEM_PER_PAGE", 10)
		input.Limit = int32(limitInt)
	}

	cacheKey := s.generateAdminOrdersCacheKey(input)
	var cacheData struct {
		Orders []v1dto.OrderSummaryDTO `json:"orders"`
		Total  int32                   `json:"total"`
	}

	if s.cache != nil {
		if err := s.cache.Get(cacheKey, &cacheData); err == nil && cacheData.Orders != nil {
			return cacheData.Orders, cacheData.Total, nil
		}
	}

	var targetUserID *int32 = nil

	var statusPtr *int32
	if input.Status > 0 {
		statusPtr = &input.Status
	}

	totalRecords, err := s.orderStore.CountOrders(ctx, targetUserID, statusPtr, input.Search)
	if err != nil {
		return []v1dto.OrderSummaryDTO{}, 0, utils.WrapError(utils.ErrCodeInternal, "failed to count orders", err)
	}
	if totalRecords == 0 {
		return []v1dto.OrderSummaryDTO{}, 0, nil
	}

	offset := (input.Page - 1) * input.Limit
	orders, err := s.orderStore.GetAll(ctx, targetUserID, statusPtr, input.Search, input.OrderBy, input.Sort, input.Limit, offset)
	if err != nil {
		return []v1dto.OrderSummaryDTO{}, 0, utils.WrapError(utils.ErrCodeInternal, "failed to get orders", err)
	}

	orderDTOs := make([]v1dto.OrderSummaryDTO, len(orders))
	for i, o := range orders {
		orderDTOs[i] = v1dto.MapToOrderSummaryDTO(o)
		if items, err := s.orderItemStore.GetByOrderID(ctx, o.OrderID); err == nil {
			itemsDTO := make([]v1dto.OrderItemDTO, len(items))
			for j, item := range items {
				itemsDTO[j] = v1dto.MapToOrderItemRowDTO(item)
			}
			orderDTOs[i].Items = itemsDTO
		}
	}

	cacheData.Orders = orderDTOs
	cacheData.Total = int32(totalRecords)
	if s.cache != nil {
		if err := s.cache.Set(cacheKey, cacheData, 2*time.Minute); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to set admin orders cache")
		}
	}

	return orderDTOs, int32(totalRecords), nil
}

func (s *orderQueryService) GetOrderInvoicePDF(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) ([]byte, string, error) {
	if s.invoiceService != nil {
		return s.invoiceService.GetOrderInvoicePDF(ctx, userUUID, orderUUID)
	}
	return nil, "", utils.NewError(utils.ErrCodeInternal, "invoice service not initialized")
}

func (s *orderQueryService) generateMyOrdersCacheKey(userUUID string, input v1dto.GetOrdersParam) string {
	return fmt.Sprintf("orders:user:%s:page:%d:limit:%d:status:%d:search:%s:orderBy:%s:sort:%s",
		userUUID, input.Page, input.Limit, input.Status, strings.TrimSpace(input.Search), input.OrderBy, input.Sort)
}

func (s *orderQueryService) generateAdminOrdersCacheKey(input v1dto.GetOrdersParam) string {
	return fmt.Sprintf("orders:admin:page:%d:limit:%d:status:%d:search:%s:orderBy:%s:sort:%s",
		input.Page, input.Limit, input.Status, strings.TrimSpace(input.Search), input.OrderBy, input.Sort)
}
