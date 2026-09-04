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
	"goshop/pkg/pdf"
	"goshop/pkg/shipping"
	"math/big"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
)

const (
	OrderStatusPending                  int32 = 1
	OrderStatusConfirmed                int32 = 2
	OrderStatusShipped                  int32 = 3
	OrderStatusDelivered                int32 = 4
	OrderStatusCancelled                int32 = 5
	OrderStatusReturned                 int32 = 6
	OrderStatusRefunded                 int32 = 6
	OrderPaymentStatusUnpaid            int32 = 1
	OrderPaymentStatusPaid              int32 = 2
	OrderPaymentStatusPartiallyRefunded int32 = 3
	OrderPaymentStatusFullyRefunded     int32 = 4
	OrderPaymentMethodCOD               int32 = 1
	OrderPaymentMethodOnline            int32 = 2
	OrderPaymentMethodEWallet           int32 = 3

	RoleAdmin     int32 = 1
	RoleModerator int32 = 2

	CouponTypePercentage  int32 = 1
	CouponTypeFixedAmount int32 = 2
)

type orderService struct {
	orderRepo       repository.OrderRepository
	orderItemRepo   repository.OrderItemRepository
	productRepo     repository.ProductRepository
	userRepo        repository.UserRepository
	cartService     CartService
	cartRepo        repository.CartRepository
	couponRepo      repository.CouponRepository
	trackingRepo    repository.TrackingRepository
	cache           cache.RedisCacheService
	shippingFactory *shipping.ShippingFactory
}

func NewOrderService(orderRepo repository.OrderRepository, orderItemRepo repository.OrderItemRepository,
	productRepo repository.ProductRepository,
	userRepo repository.UserRepository,
	cartService CartService,
	cartRepo repository.CartRepository,
	couponRepo repository.CouponRepository,
	trackingRepo repository.TrackingRepository,
	redisClient *redis.Client,
	shippingFactory *shipping.ShippingFactory) OrderService {
	return &orderService{
		orderRepo:       orderRepo,
		orderItemRepo:   orderItemRepo,
		productRepo:     productRepo,
		userRepo:        userRepo,
		cartService:     cartService,
		cartRepo:        cartRepo,
		couponRepo:      couponRepo,
		trackingRepo:    trackingRepo,
		cache:           cache.NewRedisCacheService(redisClient),
		shippingFactory: shippingFactory,
	}
}

type preparedItem struct {
	productID   int32
	productName string
	price       float64
	quantity    int32
	subtotal    float64
}

const base32Alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func GenerateOrderNumber(userID int32) string {
	now := time.Now().UTC()
	datePrefix := now.Format("060102") // 6 characters: YYMMDD

	// 1. Add user fragment based on userID modulo 256 to ensure uniqueness per user
	userFragment := fmt.Sprintf("%02X", uint32(userID%256))

	// 2. Generate 6 random characters using a cryptographically secure method without modulo bias
	const entropyLen = 6
	randomSuffix := make([]byte, entropyLen)
	alphabetLen := big.NewInt(int64(len(base32Alphabet)))

	for i := 0; i < entropyLen; i++ {
		num, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			// Safe fallback: use time-based seed if crypto/rand fails
			nano := now.UnixNano() + int64(i*1000)
			randomSuffix[i] = base32Alphabet[nano%int64(len(base32Alphabet))]
			continue
		}
		randomSuffix[i] = base32Alphabet[num.Int64()]
	}

	// Format: YYMMDD-UF-RANDOM (Ex: 260831-3F-9K2M4P) -> 16 chars
	return fmt.Sprintf("%s-%s-%s", datePrefix, userFragment, string(randomSuffix))
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

	var subtotalAmount float64
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
		subtotalAmount = utils.RoundFloat(subtotalAmount+subtotal, 2)

		preparedItems = append(preparedItems, preparedItem{
			productID:   product.ProductID,
			productName: product.ProductName,
			price:       utils.NumericToFloat(product.ProductPrice),
			quantity:    item.Quantity,
			subtotal:    subtotal,
		})
	}

	var discountAmount float64
	var appliedCouponID *int32
	var appliedCouponCode *string

	// If a coupon code is provided, validate it and apply the discount
	if input.CouponCode != nil && strings.TrimSpace(*input.CouponCode) != "" {
		normalizedCode := strings.ToUpper(strings.TrimSpace(*input.CouponCode))

		coupon, err := s.couponRepo.GetByCode(txCtx, normalizedCode)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeNotFound, "Coupon not found")
			}
			return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "failed to get coupon", err)
		}
		if !coupon.CouponIsActive {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Coupon is not active")
		}

		now := time.Now()
		if now.Before(coupon.CouponStartDate) {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Coupon is not yet active")
		}
		if now.After(coupon.CouponEndDate) {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Coupon has expired")
		}

		minOrderAmount := utils.NumericToFloat(coupon.CouponMinOrderAmount)
		if subtotalAmount < minOrderAmount {
			return sqlc.Order{}, nil, utils.NewError(
				utils.ErrCodeBadRequest,
				fmt.Sprintf("Order subtotal must be at least $%.2f to apply coupon %s", minOrderAmount, coupon.CouponCode),
			)
		}

		userUsageCount, err := s.couponRepo.CountUserUsages(txCtx, user.UserID, coupon.CouponID)
		if err != nil {
			return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "failed to count user coupon usages", err)
		}
		if userUsageCount >= int64(coupon.CouponUserUsageLimit) {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Coupon has been used the maximum number of times")
		}

		discountAmount = CalculateDiscount(coupon, subtotalAmount)

		err = s.couponRepo.IncrementUsedCount(txCtx, tx, coupon.CouponID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
				return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeConflict, "Coupon usage limit reached, please try another coupon")
			}
			return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "failed to increment coupon used count", err)
		}

		appliedCouponID = &coupon.CouponID
		appliedCouponCode = &coupon.CouponCode
	}

	finalTotalAmount := utils.RoundFloat(subtotalAmount-discountAmount, 2)

	orderNumber := GenerateOrderNumber(user.UserID)

	createOrderParams := sqlc.CreateOrderParams{
		OrderNumber:          orderNumber,
		OrderUserID:          user.UserID,
		OrderShippingName:    input.ShippingName,
		OrderShippingPhone:   input.ShippingPhone,
		OrderShippingAddress: input.ShippingAddress,
		OrderCouponID:        appliedCouponID,
		OrderCouponCode:      appliedCouponCode,
		OrderSubtotalAmount:  utils.FloatToNumeric(subtotalAmount),
		OrderDiscountAmount:  utils.FloatToNumeric(discountAmount),
		OrderTotalAmount:     utils.FloatToNumeric(finalTotalAmount),
		OrderPaymentMethod:   input.PaymentMethod,
		OrderPaymentStatus:   OrderPaymentStatusUnpaid,
		OrderNote:            &input.Note,
		OrderStatus:          OrderStatusPending,
	}

	order, err := s.orderRepo.Create(txCtx, tx, createOrderParams)
	if err != nil {
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "failed to create order", err)
	}

	if appliedCouponID != nil {
		_, err = s.couponRepo.RecordUserUsage(txCtx, tx, sqlc.RecordCouponUsageParams{
			CouponUsageCouponID: *appliedCouponID,
			CouponUsageUserID:   user.UserID,
			CouponUsageOrderID:  &order.OrderID,
		})
		if err != nil {
			return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "failed to record coupon usage", err)
		}
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

	// Remove purchased items from the user's cart if it exists
	cart, cartErr := s.cartRepo.GetCartByUserID(txCtx, user.UserID)
	if cartErr == nil {
		if delErr := s.cartRepo.DeleteCartItemsByProductIDs(txCtx, tx, cart.CartID, purchasedProductIDs); delErr != nil {
			logger.Log.Warn().Err(delErr).Msg("Failed to cleanup cart items after order creation")
		}
	}

	/* ====================== TRANSACTION END ====================== */
	if err := tx.Commit(txCtx); err != nil {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeInternal, "failed to commit transaction")
	}

	// Get order items with details after the transaction is committed
	orderItemsWithDetails, err := s.orderItemRepo.GetByOrderID(reqCtx, order.OrderID)
	if err != nil {
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "failed to get order items", err)
	}

	if err := s.cache.Clear("products:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
	}
	if err := s.cache.Clear("orders:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear order cache")
	}
	if err := s.cache.Clear("coupons:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear coupon cache")
	}
	if err := s.cache.Clear(fmt.Sprintf("cart:user:%s", userUUID.String())); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear cart cache")
	}

	return order, orderItemsWithDetails, nil
}

func CalculateDiscount(coupon sqlc.Coupon, orderAmount float64) float64 {
	var discount float64

	switch coupon.CouponType {
	case CouponTypePercentage:
		discount = utils.RoundFloat(orderAmount*utils.NumericToFloat(coupon.CouponDiscountValue)/100, 2)
		if coupon.CouponMaxDiscountAmount.Valid {
			maxDiscount := utils.NumericToFloat(coupon.CouponMaxDiscountAmount)
			if discount > maxDiscount {
				discount = maxDiscount
			}
		}
	case CouponTypeFixedAmount:
		discount = utils.NumericToFloat(coupon.CouponDiscountValue)
		if discount > orderAmount {
			discount = orderAmount
		}
	}

	return discount
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

	case OrderPaymentStatusPartiallyRefunded, OrderPaymentStatusFullyRefunded:
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

	updatedOrder, err := s.orderRepo.UpdateOrderPaymentStatus(reqCtx, nil, updateParams)
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
	_ = s.cache.Del(fmt.Sprintf("invoice:pdf:%s", existingOrder.OrderUuid.String()))

	return updatedOrder, orderItems, nil
}

type cachedInvoicePDF struct {
	Bytes    []byte `json:"bytes"`
	FileName string `json:"file_name"`
}

func (s *orderService) GetOrderInvoicePDF(ctx *gin.Context, orderUUID uuid.UUID) ([]byte, string, error) {
	reqCtx := ctx.Request.Context()

	userUUIDStr, ok := ctx.Get("user_uuid")
	if !ok || userUUIDStr == "" {
		return nil, "", utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}

	userUUID, err := uuid.Parse(userUUIDStr.(string))
	if err != nil {
		return nil, "", utils.NewError(utils.ErrCodeUnauthorized, "Invalid user UUID format")
	}

	user, err := s.userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		return nil, "", utils.NewError(utils.ErrCodeNotFound, "User not found")
	}

	order, err := s.orderRepo.GetByUUID(reqCtx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return nil, "", utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
	}

	if order.OrderPaymentStatus == OrderPaymentStatusUnpaid {
		return nil, "", utils.NewError(utils.ErrCodeBadRequest, "Invoice is not available until payment is confirmed")
	}

	if order.OrderUserID != user.UserID && user.UserLevel != RoleAdmin && user.UserLevel != RoleModerator {
		return nil, "", utils.NewError(utils.ErrCodeForbidden, "Access denied")
	}

	// 1. Kiểm tra cache Redis (Nếu đã sinh PDF trước đó -> trả về ngay trong ~1ms)
	cacheKey := fmt.Sprintf("invoice:pdf:%s", orderUUID.String())
	var cached cachedInvoicePDF
	if err := s.cache.Get(cacheKey, &cached); err == nil && len(cached.Bytes) > 0 {
		return cached.Bytes, cached.FileName, nil
	}

	orderItems, err := s.orderItemRepo.GetByOrderID(reqCtx, order.OrderID)
	if err != nil {
		return nil, "", utils.WrapError(utils.ErrCodeInternal, "Failed to get order items", err)
	}

	items := make([]pdf.InvoiceItem, len(orderItems))
	for i, item := range orderItems {
		items[i] = pdf.InvoiceItem{
			ProductName: item.OrderItemProductName,
			Quantity:    int(item.OrderItemQuantity),
			UnitPrice:   utils.NumericToFloat(item.OrderItemPrice),
			Subtotal:    utils.NumericToFloat(item.OrderItemSubtotal),
		}
	}

	paymentMethod := "COD (Cash on Delivery)"
	switch order.OrderPaymentMethod {
	case OrderPaymentMethodOnline:
		paymentMethod = "Bank Transfer / Online Payment"
	case OrderPaymentMethodEWallet:
		paymentMethod = "E-Wallet Payment"
	}

	paymentStatus := "UNPAID"
	switch order.OrderPaymentStatus {
	case OrderPaymentStatusPaid:
		paymentStatus = "PAID"
	case OrderPaymentStatusPartiallyRefunded:
		paymentStatus = "PARTIALLY REFUNDED"
	case OrderPaymentStatusFullyRefunded:
		paymentStatus = "FULLY REFUNDED"
	}

	couponCode := ""
	if order.OrderCouponCode != nil {
		couponCode = *order.OrderCouponCode
	}

	invoiceData := pdf.InvoiceData{
		OrderNumber:     order.OrderNumber,
		OrderDate:       order.OrderCreatedAt,
		CustomerName:    order.OrderShippingName,
		CustomerPhone:   order.OrderShippingPhone,
		CustomerAddress: order.OrderShippingAddress,
		PaymentMethod:   paymentMethod,
		PaymentStatus:   paymentStatus,
		Items:           items,
		SubtotalAmount:  utils.NumericToFloat(order.OrderSubtotalAmount),
		DiscountAmount:  utils.NumericToFloat(order.OrderDiscountAmount),
		CouponCode:      couponCode,
		TotalAmount:     utils.NumericToFloat(order.OrderTotalAmount),
	}

	pdfBytes, err := pdf.GenerateInvoicePDF(invoiceData)
	if err != nil {
		return nil, "", utils.WrapError(utils.ErrCodeInternal, "Failed to generate invoice PDF", err)
	}

	fileName := fmt.Sprintf("Invoice_%s.pdf", order.OrderNumber)

	// 2. Lưu vào Redis Cache (TTL 24h) cho các lần truy cập tiếp theo
	_ = s.cache.Set(cacheKey, cachedInvoicePDF{
		Bytes:    pdfBytes,
		FileName: fileName,
	}, 24*time.Hour)

	return pdfBytes, fileName, nil
}

// Shipping

func (s *orderService) CreateShipping(ctx *gin.Context, orderUUID uuid.UUID, input v1dto.CreateShippingInput) (v1dto.ShippingResponseDTO, error) {
	reqCtx := ctx.Request.Context()

	order, err := s.orderRepo.GetByUUID(reqCtx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v1dto.ShippingResponseDTO{}, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return v1dto.ShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
	}

	if order.OrderStatus != OrderStatusConfirmed {
		return v1dto.ShippingResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest,
			fmt.Sprintf("Cannot create shipping for order with status %d. Order must be Confirmed.", order.OrderStatus))
	}

	if order.OrderShippingCode != nil && strings.TrimSpace(*order.OrderShippingCode) != "" {
		return v1dto.ShippingResponseDTO{},
			utils.NewError(utils.ErrCodeConflict, "Shipping already created for this order")
	}

	orderItems, err := s.orderItemRepo.GetByOrderID(reqCtx, order.OrderID)
	if err != nil {
		return v1dto.ShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order items", err)
	}

	if len(orderItems) == 0 {
		return v1dto.ShippingResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Order contains no items")
	}

	shippingItems := make([]shipping.ShippingItem, 0, len(orderItems))
	for _, item := range orderItems {
		shippingItems = append(shippingItems, shipping.ShippingItem{
			Name:     item.OrderItemProductName,
			Quantity: int(item.OrderItemQuantity),
			Price:    utils.NumericToFloat(item.OrderItemPrice),
			Weight:   input.Weight / len(orderItems), // Chia đều hoặc lấy theo sản phẩm
		})
	}

	// Calculate COD amount if applicable
	var codAmount float64
	if order.OrderPaymentMethod == OrderPaymentMethodCOD && order.OrderPaymentStatus == OrderPaymentStatusUnpaid {
		codAmount = utils.NumericToFloat(order.OrderTotalAmount)
	}

	provider, err := s.shippingFactory.GetProvider(input.Provider)
	if err != nil {
		return v1dto.ShippingResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest, err.Error())
	}

	callbackURL := utils.GetEnv("GHN_CALLBACK_URL", "")

	ghnReq := shipping.CreateOrderRequest{
		ToName:       order.OrderShippingName,
		ToPhone:      order.OrderShippingPhone,
		ToAddress:    order.OrderShippingAddress,
		ToDistrictID: input.DistrictID,
		ToWardCode:   input.WardCode,
		Weight:       input.Weight,
		Length:       input.Length,
		Width:        input.Width,
		Height:       input.Height,
		Items:        shippingItems,
		Note:         input.Note,
		RequiredNote: input.RequiredNote,
		CODAmount:    codAmount,
		CallbackURL:  callbackURL,
	}

	ghnResp, err := provider.CreateShippingOrder(reqCtx, ghnReq)
	if err != nil {
		return v1dto.ShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to create shipping order with GHN", err)
	}

	tx, err := db.BeginTx(reqCtx)
	if err != nil {
		_ = provider.CancelShippingOrder(context.Background(), ghnResp.ShippingCode) // Attempt to cancel the shipping order if DB transaction fails
		return v1dto.ShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to begin database transaction", err)
	}
	defer tx.Rollback(reqCtx)

	var expectedDelivery pgtype.Timestamptz
	if ghnResp.ExpectedDelivery != nil {
		expectedDelivery = pgtype.Timestamptz{
			Time:  *ghnResp.ExpectedDelivery,
			Valid: true,
		}
	}

	updatedOrder, err := s.orderRepo.UpdateShippingAndStatus(reqCtx, tx, orderUUID, sqlc.UpdateOrderShippingAndStatusParams{
		ShippingProvider: &input.Provider,
		ShippingCode:     &ghnResp.ShippingCode,
		ShippingFee:      utils.FloatToNumeric(ghnResp.ShippingFee),
		ExpectedDelivery: expectedDelivery,
		OrderStatus:      OrderStatusShipped,
		OrderUuid:        orderUUID,
	})
	if err != nil {
		_ = provider.CancelShippingOrder(context.Background(), ghnResp.ShippingCode) // Attempt to cancel the shipping order if DB update fails
		return v1dto.ShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update order with shipping info", err)
	}

	eventNote := fmt.Sprintf("Shipping created with provider %s, code %s, fee: %.0f đ", strings.ToUpper(input.Provider), ghnResp.ShippingCode, ghnResp.ShippingFee)

	_, err = s.trackingRepo.InsertEvent(reqCtx, tx, sqlc.InsertTrackingEventParams{
		EventOrderID: order.OrderID,
		EventStatus:  "shipped",
		EventNote:    &eventNote,
		EventSource:  "admin",
	})

	if err != nil {
		_ = provider.CancelShippingOrder(context.Background(), ghnResp.ShippingCode)
		return v1dto.ShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to insert tracking event", err)
	}

	if err := tx.Commit(ctx); err != nil {
		_ = provider.CancelShippingOrder(context.Background(), ghnResp.ShippingCode)
		return v1dto.ShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	_ = s.cache.Clear(fmt.Sprintf("order:%s", orderUUID.String()))

	return v1dto.ShippingResponseDTO{
		OrderUUID:        updatedOrder.OrderUuid.String(),
		OrderNumber:      updatedOrder.OrderNumber,
		ShippingProvider: input.Provider,
		ShippingCode:     ghnResp.ShippingCode,
		ShippingFee:      ghnResp.ShippingFee,
		ExpectedDelivery: ghnResp.ExpectedDelivery,
		OrderStatus:      "Shipped",
		CreatedAt:        updatedOrder.OrderUpdatedAt,
	}, nil
}
