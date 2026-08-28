package v1service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	OrderStatusPending                  int32 = 1
	OrderStatusConfirmed                int32 = 2
	OrderStatusShipped                  int32 = 3
	OrderStatusDelivered                int32 = 4
	OrderStatusCancelled                int32 = 5
	OrderStatusReturned                 int32 = 6
	OrderPaymentStatusUnpaid            int32 = 1
	OrderPaymentStatusPaid              int32 = 2
	OrderPaymentStatusPartiallyRefunded int32 = 3
	OrderPaymentStatusRefunded          int32 = 4
	RoleAdmin                           int32 = 1
	RoleModerator                       int32 = 2
)

type orderService struct {
	orderRepo     repository.OrderRepository
	orderItemRepo repository.OrderItemRepository
	productRepo   repository.ProductRepository
	userRepo      repository.UserRepository
	cartService   CartService
	cartRepo      repository.CartRepository
	cache         cache.RedisCacheService
}

func NewOrderService(orderRepo repository.OrderRepository, orderItemRepo repository.OrderItemRepository, productRepo repository.ProductRepository, userRepo repository.UserRepository, cartService CartService, cartRepo repository.CartRepository, redisClient *redis.Client) OrderService {
	return &orderService{
		orderRepo:     orderRepo,
		orderItemRepo: orderItemRepo,
		productRepo:   productRepo,
		userRepo:      userRepo,
		cartService:   cartService,
		cartRepo:      cartRepo,
		cache:         cache.NewRedisCacheService(redisClient),
	}
}

type preparedItem struct {
	productID   int32
	productName string
	price       float64
	quantity    int32
	subtotal    float64
}

const charset = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

func generateOrderNumber() string {
	now := time.Now()
	datePrefix := now.Format("060102")

	const randomLength = 6
	randomSuffix := make([]byte, randomLength)

	randomBytes := make([]byte, randomLength)

	_, err := rand.Read(randomBytes)
	if err != nil {
		fallbackSuffix := fmt.Sprintf("%06d", now.Nanosecond()%1000000)
		return fmt.Sprintf("%s-%s", datePrefix, fallbackSuffix)
	}

	for i, b := range randomBytes {
		randomSuffix[i] = charset[int(b)%len(charset)]
	}

	return fmt.Sprintf("%s-%s", datePrefix, string(randomSuffix))
}

func (s *orderService) CreateOrder(ctx *gin.Context, input v1dto.CreateOrderInput) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {

	reqCtx := ctx.Request.Context()

	// Get user UUID from context
	userUUIDStr, ok := ctx.Get("user_uuid")

	if !ok || userUUIDStr == "" {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}

	userUUID, err := uuid.Parse(userUUIDStr.(string))
	if err != nil {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeUnauthorized, "Invalid user UUID format")
	}

	user, err := s.userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeNotFound, "User not found")
	}

	// Validate duplicate products in the order
	productMapInput := make(map[string]int32)
	for _, item := range input.Items {
		if _, exists := productMapInput[item.ProductUUID]; exists {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Duplicate product in order")
		}
		productMapInput[item.ProductUUID] = item.Quantity
	}

	// Deterministic Lock Ordering to prevent deadlock
	sort.Slice(input.Items, func(i, j int) bool {
		return input.Items[i].ProductUUID < input.Items[j].ProductUUID
	})

	uuids := make([]uuid.UUID, len(input.Items))
	for i, item := range input.Items {
		parsed, err := uuid.Parse(item.ProductUUID)
		if err != nil {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Invalid product UUID format: "+item.ProductUUID)
		}
		uuids[i] = parsed
	}

	/* ======================TRANSACTION START ====================== */
	txCtx, cancel := context.WithTimeout(reqCtx, 10*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "Failed to start transaction", err)
	}
	defer tx.Rollback(txCtx)

	// Fetch and lock products for update ordered by UUID to prevent deadlocks
	products, err := s.productRepo.GetByUUIDs(txCtx, tx, uuids)
	if err != nil {
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "Failed to fetch & lock products", err)
	}

	if len(products) != len(input.Items) {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeNotFound, "One or more products not found")
	}

	// Move locked products into a map for quick access
	productMap := make(map[string]sqlc.GetByUUIDsForUpdateRow)
	for _, p := range products {
		productMap[p.ProductUuid.String()] = p
	}

	var totalAmount float64
	preparedItems := make([]preparedItem, 0, len(input.Items))

	// Loop through the input items to validate (status, quantity) and prepare order items
	for _, item := range input.Items {

		if item.Quantity <= 0 {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Quantity must be greater than 0 for product: "+item.ProductUUID)
		}

		product, ok := productMap[item.ProductUUID]
		if !ok {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeNotFound, "Product not found: "+item.ProductUUID)
		}

		// Read 100% fresh product status and stock from the locked row
		if product.ProductStatus != ProductStatusActive {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Product is unavailable: "+product.ProductName)
		}

		if utils.Int32Value(product.ProductStock) < item.Quantity {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Insufficient stock for product: "+product.ProductName)
		}

		err = s.productRepo.DeductStock(txCtx, tx, product.ProductID, item.Quantity)
		if err != nil {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Insufficient stock for product: "+product.ProductName)
		}

		subtotal := utils.RoundFloat(float64(item.Quantity)*utils.NumericToFloat(product.ProductPrice), 2)
		totalAmount = utils.RoundFloat(totalAmount+subtotal, 2)

		preparedItems = append(preparedItems, preparedItem{
			productID:   product.ProductID,
			productName: product.ProductName,
			price:       utils.NumericToFloat(product.ProductPrice),
			quantity:    item.Quantity,
			subtotal:    subtotal,
		})
	}

	orderNumber := generateOrderNumber()

	createOrderParams := sqlc.CreateOrderParams{
		OrderNumber:          orderNumber,
		OrderUserID:          user.UserID,
		OrderShippingName:    input.ShippingName,
		OrderShippingPhone:   input.ShippingPhone,
		OrderShippingAddress: input.ShippingAddress,
		OrderTotalAmount:     utils.FloatToNumeric(totalAmount),
		OrderPaymentMethod:   input.PaymentMethod,
		OrderPaymentStatus:   OrderPaymentStatusUnpaid,
		OrderNote:            &input.Note,
		OrderStatus:          OrderStatusPending,
	}

	order, err := s.orderRepo.Create(txCtx, tx, createOrderParams)
	if err != nil {
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "failed to create order", err)
	}

	purchasedProductIDs := make([]int32, len(preparedItems))

	for i, item := range preparedItems {
		purchasedProductIDs[i] = item.productID
		createdItemParams := sqlc.CreateOrderItemParams{
			OrderItemOrderID:     order.OrderID,
			OrderItemProductID:   item.productID,
			OrderItemProductName: item.productName,
			OrderItemPrice:       utils.FloatToNumeric(item.price),
			OrderItemQuantity:    item.quantity,
			OrderItemSubtotal:    utils.FloatToNumeric(item.subtotal),
		}

		_, err := s.orderItemRepo.Create(txCtx, tx, createdItemParams)
		if err != nil {
			return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "failed to create order item", err)
		}
	}

	cart, cartErr := s.cartRepo.GetCartByUserID(txCtx, user.UserID)
	if cartErr == nil {
		_ = s.cartRepo.DeleteCartItemsByProductIDs(txCtx, tx, cart.CartID, purchasedProductIDs)
	}

	if err := tx.Commit(txCtx); err != nil {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeInternal, "failed to commit transaction")
	}

	orderItemsWithDetails, err := s.orderItemRepo.GetByOrderID(txCtx, order.OrderID)
	if err != nil {
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "failed to get order items", err)
	}

	if err := s.cache.Clear("products:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
	}
	if err := s.cache.Clear("orders:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear order cache")
	}
	if err := s.cache.Clear(fmt.Sprintf("cart:user:%s", userUUID.String())); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear cart cache")
	}

	return order, orderItemsWithDetails, nil
}

func (s *orderService) GetMyOrders(ctx *gin.Context, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error) {
	context := ctx.Request.Context()

	userUuidStr, ok := ctx.Get("user_uuid")
	if !ok || userUuidStr == "" {
		return []v1dto.OrderSummaryDTO{}, 0, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}

	userUUID, err := uuid.Parse(userUuidStr.(string))
	if err != nil {
		return []v1dto.OrderSummaryDTO{}, 0, utils.NewError(utils.ErrCodeUnauthorized, "Invalid user UUID format")
	}

	user, err := s.userRepo.GetByUUID(context, userUUID)
	if err != nil {
		return []v1dto.OrderSummaryDTO{}, 0, utils.NewError(utils.ErrCodeNotFound, "User not found")
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

	cacheKey := s.generateMyOrdersCacheKey(userUUID.String(), input)
	var cacheData struct {
		Orders []v1dto.OrderSummaryDTO `json:"orders"`
		Total  int32                   `json:"total"`
	}

	if err := s.cache.Get(cacheKey, &cacheData); err == nil && cacheData.Orders != nil {
		return cacheData.Orders, cacheData.Total, nil
	}

	var statusPtr *int32
	if input.Status > 0 {
		statusPtr = &input.Status
	}

	totalOrders, err := s.orderRepo.CountOrders(context, &user.UserID, statusPtr, input.Search)

	if err != nil {
		return []v1dto.OrderSummaryDTO{}, 0, utils.WrapError(utils.ErrCodeInternal, "failed to count orders", err)
	}
	if totalOrders == 0 {
		return []v1dto.OrderSummaryDTO{}, 0, nil
	}

	offset := (input.Page - 1) * input.Limit
	orders, err := s.orderRepo.GetAll(context, &user.UserID, statusPtr, input.Search, input.OrderBy,
		input.Sort, input.Limit, offset)
	if err != nil {
		return []v1dto.OrderSummaryDTO{}, 0, utils.WrapError(utils.ErrCodeInternal, "failed to get orders", err)
	}

	ordersDTO := make([]v1dto.OrderSummaryDTO, len(orders))
	for i, o := range orders {
		ordersDTO[i] = v1dto.MapToOrderSummaryDTO(o)
	}

	cacheData.Orders = ordersDTO
	cacheData.Total = int32(totalOrders)
	if err := s.cache.Set(cacheKey, cacheData, 2*time.Minute); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to set user orders cache")
	}

	return ordersDTO, int32(totalOrders), nil
}

func (s *orderService) GetOrderByUUID(ctx *gin.Context, orderUUID uuid.UUID) (sqlc.GetOrderByUUIDRow, []sqlc.GetOrderItemsByOrderIDRow, error) {
	context := ctx.Request.Context()

	userUuidStr, ok := ctx.Get("user_uuid")
	if !ok || userUuidStr == "" {
		return sqlc.GetOrderByUUIDRow{}, nil, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}

	userUUID, err := uuid.Parse(userUuidStr.(string))
	if err != nil {
		return sqlc.GetOrderByUUIDRow{}, nil, utils.NewError(utils.ErrCodeUnauthorized, "Invalid user UUID format")
	}

	user, err := s.userRepo.GetByUUID(context, userUUID)
	if err != nil {
		return sqlc.GetOrderByUUIDRow{}, nil, utils.NewError(utils.ErrCodeNotFound, "User not found")
	}

	order, err := s.orderRepo.GetByUUID(context, orderUUID)
	if err != nil {
		return sqlc.GetOrderByUUIDRow{}, nil, utils.NewError(utils.ErrCodeNotFound, "Order not found")
	}

	isAdmin := user.UserLevel == RoleAdmin
	isModerator := user.UserLevel == RoleModerator

	if order.OrderUserID != user.UserID && !isAdmin && !isModerator {
		return sqlc.GetOrderByUUIDRow{}, nil, utils.NewError(utils.ErrCodeForbidden, "You are not authorized to view this order")
	}

	orderItems, err := s.orderItemRepo.GetByOrderID(context, order.OrderID)
	if err != nil {
		return sqlc.GetOrderByUUIDRow{}, nil, utils.WrapError(utils.ErrCodeInternal, "failed to get order items", err)
	}

	return order, orderItems, nil
}

func (s *orderService) generateMyOrdersCacheKey(userUUID string, input v1dto.GetOrdersParam) string {
	return fmt.Sprintf("orders:user:%s:page:%d:limit:%d:status:%d:search:%s:orderBy:%s:sort:%s",
		userUUID, input.Page, input.Limit, input.Status, strings.TrimSpace(input.Search), input.OrderBy, input.Sort)
}

func (s *orderService) generateAdminOrdersCacheKey(input v1dto.GetOrdersParam) string {
	return fmt.Sprintf("orders:admin:page:%d:limit:%d:status:%d:search:%s:orderBy:%s:sort:%s",
		input.Page, input.Limit, input.Status, strings.TrimSpace(input.Search), input.OrderBy, input.Sort)
}

func (s *orderService) GetAllOrders(ctx *gin.Context, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error) {
	reqCtx := ctx.Request.Context()

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

	if err := s.cache.Get(cacheKey, &cacheData); err == nil && cacheData.Orders != nil {
		return cacheData.Orders, cacheData.Total, nil
	}

	var targetUserID *int32 = nil

	var statusPtr *int32
	if input.Status > 0 {
		statusPtr = &input.Status
	}

	totalRecords, err := s.orderRepo.CountOrders(reqCtx, targetUserID, statusPtr, input.Search)
	if err != nil {
		return []v1dto.OrderSummaryDTO{}, 0, utils.WrapError(utils.ErrCodeInternal, "failed to count orders", err)
	}
	if totalRecords == 0 {
		return []v1dto.OrderSummaryDTO{}, 0, nil
	}

	offset := (input.Page - 1) * input.Limit
	orders, err := s.orderRepo.GetAll(reqCtx, targetUserID, statusPtr, input.Search, input.OrderBy, input.Sort, input.Limit, offset)
	if err != nil {
		return []v1dto.OrderSummaryDTO{}, 0, utils.WrapError(utils.ErrCodeInternal, "failed to get orders", err)
	}

	orderDTOs := make([]v1dto.OrderSummaryDTO, len(orders))
	for i, o := range orders {
		orderDTOs[i] = v1dto.MapToOrderSummaryDTO(o)
	}

	cacheData.Orders = orderDTOs
	cacheData.Total = int32(totalRecords)
	if err := s.cache.Set(cacheKey, cacheData, 2*time.Minute); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to set admin orders cache")
	}

	return orderDTOs, int32(totalRecords), nil
}

func (s *orderService) UpdateOrderStatus(ctx *gin.Context, input sqlc.UpdateOrderStatusParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {
	context := ctx.Request.Context()

	existingOrder, err := s.orderRepo.GetByUUID(context, input.OrderUuid)
	if err != nil {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeInternal, "Order not found")
	}

	if !isValidStatusTransition(existingOrder.OrderStatus, input.OrderStatus) {
		errMsg := fmt.Sprintf("Cannot change order status from '%s' to '%s'",
			v1dto.MapOrderStatusToString(existingOrder.OrderStatus),
			v1dto.MapOrderStatusToString(input.OrderStatus),
		)
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, errMsg)
	}

	updatedOrder, err := s.orderRepo.UpdateStatus(context, input)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeNotFound, "order not found")
		}
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "failed to update order status", err)
	}

	orderItems, err := s.orderItemRepo.GetByOrderID(context, updatedOrder.OrderID)
	if err != nil {
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "failed to get order items", err)
	}

	if err := s.cache.Clear("orders:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear order cache")
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

func (s *orderService) CancelOrder(ctx *gin.Context, orderUUID uuid.UUID) error {
	reqCtx := ctx.Request.Context()

	userUUIDStr, ok := ctx.Get("user_uuid")
	if !ok || userUUIDStr == "" {
		return utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}

	userUUID, err := uuid.Parse(userUUIDStr.(string))
	if err != nil {
		return utils.NewError(utils.ErrCodeUnauthorized, "Invalid user UUID format")
	}

	user, err := s.userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		return utils.NewError(utils.ErrCodeNotFound, "User not found")
	}

	orderRow, err := s.orderRepo.GetByUUID(reqCtx, orderUUID)
	if err != nil {
		return utils.NewError(utils.ErrCodeNotFound, "Order not found")
	}

	if orderRow.OrderUserID != user.UserID {
		return utils.NewError(utils.ErrCodeForbidden, "You are not authorized to cancel this order")
	}

	if orderRow.OrderStatus != OrderStatusPending && orderRow.OrderStatus != OrderStatusConfirmed {
		return utils.NewError(utils.ErrCodeBadRequest, "Only pending or confirmed orders can be cancelled")
	}

	orderItems, err := s.orderItemRepo.GetByOrderID(reqCtx, orderRow.OrderID)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "failed to get order items", err)
	}

	txCtx, cancel := context.WithTimeout(reqCtx, 10*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	defer tx.Rollback(txCtx)

	_, err = s.orderRepo.CancelOrder(txCtx, tx, orderUUID)
	if err != nil {
		return utils.WrapError(utils.ErrCodeBadRequest, "failed to cancel order", err)
	}

	sort.Slice(orderItems, func(i, j int) bool {
		return orderItems[i].OrderItemProductID < orderItems[j].OrderItemProductID
	})

	for _, item := range orderItems {
		err = s.productRepo.RestoreStock(txCtx, tx, item.OrderItemProductID, item.OrderItemQuantity)
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "failed to restore stock", err)
		}
	}

	if err := tx.Commit(txCtx); err != nil {
		return utils.NewError(utils.ErrCodeInternal, "failed to commit transaction")
	}

	if err := s.cache.Clear("products:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
	}
	if err := s.cache.Clear("orders:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear order cache")
	}

	return nil
}

func (s *orderService) UpdateOrderShippingInfo(ctx *gin.Context, input sqlc.UpdateOrderShippingInfoParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {
	reqCtx := ctx.Request.Context()

	userUuidStr, ok := ctx.Get("user_uuid")
	if !ok || userUuidStr == "" {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}

	userUUID, err := uuid.Parse(userUuidStr.(string))
	if err != nil {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeUnauthorized, "Invalid user UUID format")
	}

	user, err := s.userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeNotFound, "User not found")
	}

	existingOrder, err := s.orderRepo.GetByUUID(reqCtx, input.OrderUuid)
	if err != nil {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeNotFound, "Order not found")
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

	// Race condition prevention: Ensure that the order status is still valid for shipping info update
	allowedStatuses := []int32{OrderStatusPending}
	if isAdmin || isModerator {
		allowedStatuses = append(allowedStatuses, OrderStatusConfirmed)
	}

	isStatusValid := slices.Contains(allowedStatuses, existingOrder.OrderStatus)

	if !isStatusValid {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Order status does not allow shipping info update")
	}

	input.AllowedStatuses = allowedStatuses

	updatedOrder, err := s.orderRepo.UpdateOrderShippingInfo(reqCtx, input)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Possible reasons: order does not exist, OR status has changed (race condition
			// was blocked at the right time) -> return a clear error, suggest reloading the page.
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeConflict, "Order status has changed or order not found, please refresh and try again")
		}
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "Failed to update shipping info", err)
	}

	orderItems, err := s.orderItemRepo.GetByOrderID(reqCtx, updatedOrder.OrderID)
	if err != nil {
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "Failed to get order items", err)
	}

	if err := s.cache.Clear("orders:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear order cache")
	}
	return updatedOrder, orderItems, nil
}

func (s *orderService) UpdateOrderPaymentStatus(ctx *gin.Context, orderUUID uuid.UUID, input v1dto.UpdateOrderPaymentStatusInput) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {
	reqCtx := ctx.Request.Context()

	userUUIDStr, ok := ctx.Get("user_uuid")
	if !ok || userUUIDStr == "" {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}
	userUUID, err := uuid.Parse(userUUIDStr.(string))
	if err != nil {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Invalid user UUID")
	}

	existingOrder, err := s.orderRepo.GetByUUID(reqCtx, orderUUID)
	if err != nil {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeNotFound, "Order not found")
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

	case OrderPaymentStatusPartiallyRefunded, OrderPaymentStatusRefunded:
		if existingOrder.OrderPaymentStatus != OrderPaymentStatusPaid && existingOrder.OrderPaymentStatus != OrderPaymentStatusPartiallyRefunded {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Only Paid or Partially Refunded orders can be refunded")
		}
		// Security Check: Refund is ONLY allowed when order is Cancelled (5) or Returned (6)
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
			targetPaymentStatus = OrderPaymentStatusRefunded
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

	updatedOrder, err := s.orderRepo.UpdateOrderPaymentStatus(reqCtx, updateParams)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeConflict, "Payment status has changed by another request, please refresh")
		}
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "Failed to update payment status", err)
	}

	orderItems, err := s.orderItemRepo.GetByOrderID(reqCtx, updatedOrder.OrderID)
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

	userKey := fmt.Sprintf("orders:user:%s:*", existingOrder.UserUuid.String())
	_ = s.cache.Clear(userKey)
	_ = s.cache.Clear("orders:admin:*")

	return updatedOrder, orderItems, nil
}
