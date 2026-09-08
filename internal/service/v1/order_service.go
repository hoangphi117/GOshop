package v1service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/payment"
	"goshop/pkg/pdf"
	"goshop/pkg/rabbitmq"
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
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

const (
	OrderStatusPending   int32 = 1
	OrderStatusConfirmed int32 = 2
	OrderStatusShipped   int32 = 3
	OrderStatusDelivered int32 = 4
	OrderStatusCancelled int32 = 5
	OrderStatusReturned  int32 = 6
	OrderStatusRefunded  int32 = 6

	OrderPaymentStatusUnpaid            int32 = 1
	OrderPaymentStatusPaid              int32 = 2
	OrderPaymentStatusPartiallyRefunded int32 = 3
	OrderPaymentStatusFullyRefunded     int32 = 4
	OrderPaymentMethodCOD               int32 = 1
	OrderPaymentMethodOnline            int32 = 2
	OrderPaymentMethodEWallet           int32 = 3

	OrderReturnStatusPending   int16 = 1
	OrderReturnStatusApproved  int16 = 2
	OrderReturnStatusRejected  int16 = 3
	OrderReturnStatusCompleted int16 = 4
	OrderReturnStatusCancelled int16 = 5

	RoleAdmin     int32 = 1
	RoleModerator int32 = 2

	CouponTypePercentage  int32 = 1
	CouponTypeFixedAmount int32 = 2

	QueueShippingCreate = "shipping.order.create"

	addressCacheTTL = 30 * 24 * time.Hour
)

type orderService struct {
	orderRepo           repository.OrderRepository
	orderItemRepo       repository.OrderItemRepository
	orderReturnRepo     repository.OrderReturnsRepository
	productRepo         repository.ProductRepository
	userRepo            repository.UserRepository
	cartService         CartService
	cartRepo            repository.CartRepository
	couponRepo          repository.CouponRepository
	trackingRepo        repository.TrackingRepository
	webhookRepo         repository.WebhookEventRepository
	paymentRepo         repository.PaymentRepository
	paymentProviders    map[string]payment.PaymentProvider
	notificationService NotificationService
	cache               cache.RedisCacheService
	shippingFactory     *shipping.ShippingFactory
	rabbitmq            rabbitmq.RabbitMQService
}

func NewOrderService(orderRepo repository.OrderRepository,
	orderItemRepo repository.OrderItemRepository,
	orderReturnRepo repository.OrderReturnsRepository,
	productRepo repository.ProductRepository,
	userRepo repository.UserRepository,
	cartService CartService,
	cartRepo repository.CartRepository,
	couponRepo repository.CouponRepository,
	trackingRepo repository.TrackingRepository,
	webhookRepo repository.WebhookEventRepository,
	paymentRepo repository.PaymentRepository,
	paymentProviders map[string]payment.PaymentProvider,
	notificationService NotificationService,
	redisClient *redis.Client,
	shippingFactory *shipping.ShippingFactory,
	rabbitmq rabbitmq.RabbitMQService) OrderService {
	return &orderService{
		orderRepo:           orderRepo,
		orderItemRepo:       orderItemRepo,
		orderReturnRepo:     orderReturnRepo,
		productRepo:         productRepo,
		userRepo:            userRepo,
		cartService:         cartService,
		cartRepo:            cartRepo,
		couponRepo:          couponRepo,
		trackingRepo:        trackingRepo,
		webhookRepo:         webhookRepo,
		paymentRepo:         paymentRepo,
		paymentProviders:    paymentProviders,
		notificationService: notificationService,
		cache:               cache.NewRedisCacheService(redisClient),
		shippingFactory:     shippingFactory,
		rabbitmq:            rabbitmq,
	}
}

func (s *orderService) getUser(ctx *gin.Context) (sqlc.User, error) {
	return GetUserFromContext(ctx, s.userRepo)
}

func (s *orderService) getUserUUID(ctx *gin.Context) (uuid.UUID, error) {
	return GetUserUUIDFromContext(ctx)
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

	// Get user from context
	user, err := s.getUser(ctx)
	if err != nil {
		return sqlc.Order{}, nil, err
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

	shippingFee := utils.RoundFloat(input.ShippingFee, 2)

	finalTotalAmount := utils.RoundFloat(subtotalAmount-discountAmount+shippingFee, 2)
	if finalTotalAmount < 0 {
		finalTotalAmount = 0
	}

	orderNumber := GenerateOrderNumber(user.UserID)

	createOrderParams := sqlc.CreateOrderParams{
		OrderNumber:          orderNumber,
		OrderUserID:          user.UserID,
		OrderShippingName:    input.ShippingName,
		OrderShippingPhone:   input.ShippingPhone,
		OrderShippingAddress: input.ShippingAddress,
		OrderShippingFee:     utils.FloatToNumeric(shippingFee),
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

	// Record the initial order tracking event
	createNote := "Order created and pending confirmation"
	_, err = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
		EventOrderID: order.OrderID,
		EventStatus:  "pending",
		EventNote:    &createNote,
		EventSource:  "user",
	})

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
	if err := s.cache.Clear(fmt.Sprintf("cart:user:%s", user.UserUuid.String())); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear cart cache")
	}

	go func() {
		_ = s.notificationService.Send(context.Background(), v1dto.SendNotificationInput{
			UserID: order.OrderUserID,
			Type:   "order_created",
			Title:  "Order Placed Successfully",
			Body:   fmt.Sprintf("Your order #%s has been placed successfully.", order.OrderNumber),
			Data:   map[string]any{"order_uuid": order.OrderUuid.String()},
		})
	}()

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

	user, err := s.getUser(ctx)
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

	user, err := s.getUser(ctx)
	if err != nil {
		return sqlc.GetOrderByUUIDRow{}, nil, err
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
	reqCtx := ctx.Request.Context()

	existingOrder, err := s.orderRepo.GetByUUID(reqCtx, input.OrderUuid)
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

	updatedOrder, err := s.orderRepo.UpdateStatus(reqCtx, input)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
		_, _ = s.trackingRepo.InsertEvent(reqCtx, nil, sqlc.InsertTrackingEventParams{
			EventOrderID: updatedOrder.OrderID,
			EventStatus:  v1dto.MapOrderStatusToString(input.OrderStatus),
			EventNote:    &statusNote,
			EventSource:  "admin",
		})
	}

	orderItems, err := s.orderItemRepo.GetByOrderID(reqCtx, updatedOrder.OrderID)
	if err != nil {
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "failed to get order items", err)
	}

	if err := s.cache.Clear("orders:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear order cache")
	}

	if input.OrderStatus == OrderStatusConfirmed {
		go func() {
			logger.Log.Info().
				Int32("target_customer_id", updatedOrder.OrderUserID).
				Str("order_number", updatedOrder.OrderNumber).
				Msg("Dispatching order_confirmed notification")

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

func (s *orderService) CancelOrder(ctx *gin.Context, orderUUID uuid.UUID) error {
	reqCtx := ctx.Request.Context()

	user, err := s.getUser(ctx)
	if err != nil {
		return err
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

	cancelNote := "Order cancelled."
	_, _ = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
		EventOrderID: orderRow.OrderID,
		EventStatus:  "cancelled",
		EventNote:    &cancelNote,
		EventSource:  "user",
	})

	if err := tx.Commit(txCtx); err != nil {
		return utils.NewError(utils.ErrCodeInternal, "failed to commit transaction")
	}

	if err := s.cache.Clear("products:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
	}
	if err := s.cache.Clear("orders:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear order cache")
	}

	go func() {
		_ = s.notificationService.Send(context.Background(), v1dto.SendNotificationInput{
			UserID: orderRow.OrderUserID,
			Type:   "order_cancelled",
			Title:  "Order Cancelled",
			Body:   fmt.Sprintf("Your order #%s has been cancelled.", orderRow.OrderNumber),
			Data:   map[string]any{"order_uuid": orderRow.OrderUuid.String()},
		})
	}()

	return nil
}

func (s *orderService) UpdateOrderShippingInfo(ctx *gin.Context, input sqlc.UpdateOrderShippingInfoParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {
	reqCtx := ctx.Request.Context()

	user, err := s.getUser(ctx)
	if err != nil {
		return sqlc.Order{}, nil, err
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

	userUUID, err := s.getUserUUID(ctx)
	if err != nil {
		return sqlc.Order{}, nil, err
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

	user, err := s.getUser(ctx)
	if err != nil {
		return nil, "", err
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

func (s *orderService) CreateShipping(ctx *gin.Context, orderUUID uuid.UUID, input v1dto.CreateShippingInput) (v1dto.AsyncShippingResponseDTO, error) {
	reqCtx := ctx.Request.Context()

	order, err := s.orderRepo.GetByUUID(reqCtx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v1dto.AsyncShippingResponseDTO{}, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return v1dto.AsyncShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
	}

	if order.OrderStatus != OrderStatusConfirmed {
		return v1dto.AsyncShippingResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest,
			fmt.Sprintf("Cannot create shipping for order with status %d. Order must be Confirmed.", order.OrderStatus))
	}

	if order.OrderShippingCode != nil && strings.TrimSpace(*order.OrderShippingCode) != "" {
		return v1dto.AsyncShippingResponseDTO{},
			utils.NewError(utils.ErrCodeConflict, "Shipping already created for this order")
	}

	msg := v1dto.CreateShippingQueueMessage{
		OrderUUID:  orderUUID.String(),
		Input:      input,
		EnqueuedAt: time.Now(),
	}

	if err := s.rabbitmq.Publish(reqCtx, QueueShippingCreate, msg); err != nil {
		logger.Log.Error().Err(err).Str("order_uuid", orderUUID.String()).Msg("Failed to enqueue shipping creation")
		return v1dto.AsyncShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to enqueue shipping creation", err)
	}

	return v1dto.AsyncShippingResponseDTO{
		OrderUUID: orderUUID.String(),
		Status:    "processing",
		Message:   "Yêu cầu tạo vận đơn đã được tiếp nhận và đang được hệ thống xử lý",
	}, nil
}

func (s *orderService) ProcessShippingOrderMessage(headers amqp.Table, body []byte) error {
	ctx := context.Background()

	var msg v1dto.CreateShippingQueueMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to unmarshal shipping creation message")
		return err
	}

	orderUUID, err := uuid.Parse(msg.OrderUUID)
	if err != nil {
		return err
	}

	order, err := s.orderRepo.GetByUUID(ctx, orderUUID)
	if err != nil {
		return err
	}

	if order.OrderStatus != OrderStatusConfirmed {
		logger.Log.Warn().Str("order_uuid", msg.OrderUUID).Msg("Order is no longer Confirmed, skipping shipping creation")
		return nil
	}

	orderItems, err := s.orderItemRepo.GetByOrderID(ctx, order.OrderID)
	if err != nil {
		return err
	}

	shippingItems := make([]shipping.ShippingItem, 0, len(orderItems))
	for _, item := range orderItems {
		shippingItems = append(shippingItems, shipping.ShippingItem{
			Name:     item.OrderItemProductName,
			Quantity: int(item.OrderItemQuantity),
			Price:    utils.NumericToFloat(item.OrderItemPrice),
			Weight:   msg.Input.Weight / len(orderItems),
		})
	}

	var codAmount float64
	if order.OrderPaymentMethod == OrderPaymentMethodCOD && order.OrderPaymentStatus == OrderPaymentStatusUnpaid {
		codAmount = utils.NumericToFloat(order.OrderTotalAmount)
	}
	provider, err := s.shippingFactory.GetProvider(msg.Input.Provider)
	if err != nil {
		return err
	}
	ghnReq := shipping.CreateOrderRequest{
		ToName:       order.OrderShippingName,
		ToPhone:      order.OrderShippingPhone,
		ToAddress:    order.OrderShippingAddress,
		ToDistrictID: msg.Input.DistrictID,
		ToWardCode:   msg.Input.WardCode,
		Weight:       msg.Input.Weight,
		Length:       msg.Input.Length,
		Width:        msg.Input.Width,
		Height:       msg.Input.Height,
		Items:        shippingItems,
		Note:         msg.Input.Note,
		RequiredNote: msg.Input.RequiredNote,
		CODAmount:    codAmount,
		CallbackURL:  utils.GetEnv("GHN_CALLBACK_URL", ""),
	}

	ghnResp, err := provider.CreateShippingOrder(ctx, ghnReq)
	if err != nil {
		logger.Log.Error().Err(err).Str("order_uuid", msg.OrderUUID).Msg("GHN API creation failed")
		failNote := fmt.Sprintf("Tạo vận đơn GHN thất bại: %s", err.Error())
		_, _ = s.trackingRepo.InsertEvent(ctx, nil, sqlc.InsertTrackingEventParams{
			EventOrderID: order.OrderID,
			EventStatus:  "shipping_failed",
			EventNote:    &failNote,
			EventSource:  "system",
		})
		return err
	}

	tx, err := db.BeginTx(ctx)
	if err != nil {
		_ = provider.CancelShippingOrder(ctx, ghnResp.ShippingCode)
		return err
	}
	defer tx.Rollback(ctx)
	var expectedDelivery pgtype.Timestamptz
	if ghnResp.ExpectedDelivery != nil {
		expectedDelivery = pgtype.Timestamptz{
			Time:  *ghnResp.ExpectedDelivery,
			Valid: true,
		}
	}
	_, err = s.orderRepo.UpdateShippingAndStatus(ctx, tx, orderUUID, sqlc.UpdateOrderShippingAndStatusParams{
		ShippingProvider: &msg.Input.Provider,
		ShippingCode:     &ghnResp.ShippingCode,
		ShippingFee:      utils.FloatToNumeric(ghnResp.ShippingFee),
		ExpectedDelivery: expectedDelivery,
		OrderStatus:      OrderStatusShipped,
		OrderUuid:        orderUUID,
	})
	if err != nil {
		_ = provider.CancelShippingOrder(ctx, ghnResp.ShippingCode)
		return err
	}
	// Ghi nhận tracking timeline
	eventNote := fmt.Sprintf("Tạo đơn vận chuyển qua %s thành công. Mã vận đơn: %s, Cước: %.0f đ",
		strings.ToUpper(msg.Input.Provider), ghnResp.ShippingCode, ghnResp.ShippingFee)
	_, err = s.trackingRepo.InsertEvent(ctx, tx, sqlc.InsertTrackingEventParams{
		EventOrderID: order.OrderID,
		EventStatus:  "shipped",
		EventNote:    &eventNote,
		EventSource:  "admin",
	})
	if err != nil {
		_ = provider.CancelShippingOrder(ctx, ghnResp.ShippingCode)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		_ = provider.CancelShippingOrder(ctx, ghnResp.ShippingCode)
		return err
	}
	_ = s.cache.Clear(fmt.Sprintf("order:%s", orderUUID.String()))
	logger.Log.Info().Str("order_uuid", msg.OrderUUID).Str("shipping_code", ghnResp.ShippingCode).Msg("Shipping order created successfully via RabbitMQ")

	go func() {
		_ = s.notificationService.Send(context.Background(), v1dto.SendNotificationInput{
			UserID: order.OrderUserID,
			Type:   "order_shipped",
			Title:  "Your Order Is On The Way",
			Body:   fmt.Sprintf("Order #%s has been shipped with tracking code: %s.", order.OrderNumber, ghnResp.ShippingCode),
			Data: map[string]any{
				"order_uuid":    orderUUID.String(),
				"shipping_code": ghnResp.ShippingCode,
			},
		})
	}()

	return nil

}

func (s *orderService) GetOrderTimeline(ctx *gin.Context, orderUUID uuid.UUID) (v1dto.OrderTimelineResponseDTO, error) {
	reqCtx := ctx.Request.Context()

	user, err := s.getUser(ctx)
	if err != nil {
		return v1dto.OrderTimelineResponseDTO{}, err
	}

	order, err := s.orderRepo.GetByUUID(reqCtx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v1dto.OrderTimelineResponseDTO{}, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return v1dto.OrderTimelineResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
	}

	// Check if the user is the owner of the order or has admin/moderator privileges
	isAdminOrMod := user.UserLevel == RoleAdmin || user.UserLevel == RoleModerator
	if order.OrderUserID != user.UserID && !isAdminOrMod {
		return v1dto.OrderTimelineResponseDTO{}, utils.NewError(utils.ErrCodeForbidden, "Access denied")
	}

	events, err := s.trackingRepo.GetByOrderID(reqCtx, order.OrderID)
	if err != nil {
		return v1dto.OrderTimelineResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get tracking events", err)
	}
	return v1dto.OrderTimelineResponseDTO{
		OrderUUID:   order.OrderUuid.String(),
		OrderNumber: order.OrderNumber,
		Events:      v1dto.MapToEventDTOs(events),
	}, nil
}

func (s *orderService) HandleShippingWebhook(ctx *gin.Context, rawBody []byte, payload v1dto.GHNWebhookPayload) error {
	reqCtx := ctx.Request.Context()

	ghnStatus := strings.ToLower(strings.TrimSpace(payload.Status))

	// Tạo externalEventID để dedup qua bảng webhook_events
	var eventTime int64
	if !payload.Time.IsZero() {
		eventTime = payload.Time.Unix()
	}
	externalEventID := fmt.Sprintf("%s_%s_%d", payload.OrderCode, ghnStatus, eventTime)

	eventParams := sqlc.InsertWebhookEventParams{
		WeProvider:        "ghn",
		WeExternalEventID: externalEventID,
		WeEventType:       ghnStatus,
		WeRawBody:         string(rawBody),
	}

	event, err := s.webhookRepo.InsertEvent(reqCtx, nil, eventParams)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Log.Info().
				Str("provider", "ghn").
				Str("event_id", externalEventID).
				Msg("Duplicate GHN webhook event received (ON CONFLICT), acknowledging gateway")
			return nil
		}
		logger.Log.Error().
			Err(err).
			Str("provider", "ghn").
			Str("event_id", externalEventID).
			Msg("Database failure while inserting GHN webhook event")
		return utils.WrapError(utils.ErrCodeInternal, "failed to persist webhook event", err)
	}

	if event.WeID == 0 {
		logger.Log.Info().
			Str("provider", "ghn").
			Str("event_id", externalEventID).
			Msg("Duplicate GHN webhook event received (WeID=0), acknowledging gateway")
		return nil
	}

	order, err := s.orderRepo.GetByShippingCode(reqCtx, payload.OrderCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Log.Warn().Str("shipping_code", payload.OrderCode).Msg("Received webhook for unknown shipping code")
			_ = s.webhookRepo.MarkProcessed(reqCtx, nil, event.WeID)
			return nil
		}
		return utils.WrapError(utils.ErrCodeInternal, "Failed to get order by shipping code", err)
	}

	// Xử lý nghiệp vụ theo trạng thái từ GHN
	switch ghnStatus {
	case "delivered":
		if order.OrderStatus == OrderStatusDelivered {
			_ = s.webhookRepo.MarkProcessed(reqCtx, nil, event.WeID)
			return nil
		}

		tx, err := db.BeginTx(reqCtx)
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to begin transaction", err)
		}
		defer tx.Rollback(reqCtx)

		updatedOrder, err := s.orderRepo.MarkDeliveredByShippingCode(reqCtx, tx, payload.OrderCode)
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to update order status to delivered", err)
		}

		eventNote := "Order delivered successfully via shipping provider."
		_, err = s.trackingRepo.InsertEvent(reqCtx, tx, sqlc.InsertTrackingEventParams{
			EventOrderID: order.OrderID,
			EventStatus:  "delivered",
			EventNote:    &eventNote,
			EventSource:  "ghn",
		})
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to record tracking event", err)
		}

		if err := tx.Commit(reqCtx); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
		}

		// Xóa cache đơn hàng
		_ = s.cache.Clear(fmt.Sprintf("order:%s", updatedOrder.OrderUuid.String()))
		logger.Log.Info().Str("order_number", order.OrderNumber).Str("code", payload.OrderCode).Msg("Order marked as DELIVERED via GHN Webhook")

		go func() {
			_ = s.notificationService.Send(context.Background(), v1dto.SendNotificationInput{
				UserID: updatedOrder.OrderUserID,
				Type:   "order_delivered",
				Title:  "Order Delivered",
				Body:   fmt.Sprintf("Your order #%s has been delivered. Please confirm receipt.", order.OrderNumber),
				Data:   map[string]any{"order_uuid": updatedOrder.OrderUuid.String()},
			})
		}()

	case "delivering":
		eventNote := "Order is currently being delivered by shipping provider."
		if payload.Warehouse != "" {
			eventNote = fmt.Sprintf("Order is being shipped from %s", payload.Warehouse)
		}
		_, _ = s.trackingRepo.InsertEvent(reqCtx, nil, sqlc.InsertTrackingEventParams{
			EventOrderID: order.OrderID,
			EventStatus:  "delivering",
			EventNote:    &eventNote,
			EventSource:  "ghn",
		})

	case "delivery_fail":
		eventNote := "Delivery attempt failed."
		if payload.Description != "" {
			eventNote = fmt.Sprintf("Delivery failed: %s", payload.Description)
		}

		_, _ = s.trackingRepo.InsertEvent(reqCtx, nil, sqlc.InsertTrackingEventParams{
			EventOrderID: order.OrderID,
			EventStatus:  "delivery_fail",
			EventNote:    &eventNote,
			EventSource:  "ghn",
		})

	case "return", "returned":
		eventNote := "Order is being returned by shipping provider."
		if payload.Description != "" {
			eventNote = fmt.Sprintf("Order returned: %s", payload.Description)
		}

		_, _ = s.trackingRepo.InsertEvent(reqCtx, nil, sqlc.InsertTrackingEventParams{
			EventOrderID: order.OrderID,
			EventStatus:  "returned",
			EventNote:    &eventNote,
			EventSource:  "ghn",
		})

	default:
		eventNote := fmt.Sprintf("GHN update status: %s", ghnStatus)
		if payload.Description != "" {
			eventNote = payload.Description
		}
		_, _ = s.trackingRepo.InsertEvent(reqCtx, nil, sqlc.InsertTrackingEventParams{
			EventOrderID: order.OrderID,
			EventStatus:  ghnStatus,
			EventNote:    &eventNote,
			EventSource:  "ghn",
		})
	}

	// 4. Đánh dấu webhook event đã xử lý thành công
	if err := s.webhookRepo.MarkProcessed(reqCtx, nil, event.WeID); err != nil {
		logger.Log.Error().
			Err(err).
			Int32("webhook_event_id", event.WeID).
			Msg("Failed to mark webhook event as processed")
	}

	return nil
}

func (s *orderService) CalculateShippingFee(ctx *gin.Context, input v1dto.CalculateShippingFeeInput) (v1dto.ShippingFeeResponseDTO, error) {
	reqCtx := ctx.Request.Context()

	provider, err := s.shippingFactory.GetProvider(input.Provider)
	if err != nil {
		return v1dto.ShippingFeeResponseDTO{}, utils.NewError(
			utils.ErrCodeBadRequest,
			fmt.Sprintf("Unsupported shipping provider: %s", input.Provider),
		)
	}

	req := shipping.CreateOrderRequest{
		ToDistrictID: input.DistrictID,
		ToWardCode:   input.WardCode,
		Weight:       input.Weight,
		Length:       input.Length,
		Width:        input.Width,
		Height:       input.Height,
		CODAmount:    input.InsuranceValue,
	}

	fee, err := provider.CalculateFee(reqCtx, req)
	if err != nil {
		logger.Log.Error().
			Err(err).
			Str("provider", input.Provider).
			Int("district_id", input.DistrictID).
			Str("ward_code", input.WardCode).
			Msg("Failed to calculate shipping fee from provider")
		return v1dto.ShippingFeeResponseDTO{}, utils.WrapError(
			utils.ErrCodeInternal,
			"Failed to calculate shipping fee from shipping provider",
			err,
		)
	}

	return v1dto.ShippingFeeResponseDTO{
		Provider:    input.Provider,
		ShippingFee: fee,
	}, nil
}

func (s *orderService) CancelShipping(ctx *gin.Context, orderUUID uuid.UUID, input v1dto.CancelShippingInput) (v1dto.CancelShippingResponseDTO, error) {
	reqCtx := ctx.Request.Context()

	order, err := s.orderRepo.GetByUUID(reqCtx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v1dto.CancelShippingResponseDTO{}, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return v1dto.CancelShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
	}

	if order.OrderStatus != OrderStatusShipped {
		return v1dto.CancelShippingResponseDTO{}, utils.NewError(
			utils.ErrCodeBadRequest,
			fmt.Sprintf("Cannot cancel shipping for order with status %d. Order must be in Shipped status.", order.OrderStatus),
		)
	}

	if order.OrderShippingCode == nil || strings.TrimSpace(*order.OrderShippingCode) == "" {
		return v1dto.CancelShippingResponseDTO{}, utils.NewError(
			utils.ErrCodeBadRequest,
			"Order does not have a valid shipping code",
		)
	}

	shippingCode := *order.OrderShippingCode
	providerName := "ghn"
	if order.OrderShippingProvider != nil && strings.TrimSpace(*order.OrderShippingProvider) != "" {
		providerName = *order.OrderShippingProvider
	}

	provider, err := s.shippingFactory.GetProvider(providerName)
	if err != nil {
		return v1dto.CancelShippingResponseDTO{}, utils.NewError(
			utils.ErrCodeBadRequest,
			fmt.Sprintf("Unsupported shipping provider: %s", providerName),
		)
	}

	if err := provider.CancelShippingOrder(reqCtx, shippingCode); err != nil {
		logger.Log.Error().
			Err(err).
			Str("provider", providerName).
			Str("shipping_code", shippingCode).
			Msg("Failed to cancel shipping order with provider")
		return v1dto.CancelShippingResponseDTO{}, utils.WrapError(
			utils.ErrCodeInternal,
			"Failed to cancel shipping order with provider",
			err,
		)
	}

	txCtx, cancel := context.WithTimeout(reqCtx, 10*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return v1dto.CancelShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to begin transaction", err)
	}
	defer tx.Rollback(txCtx)

	updatedOrder, err := s.orderRepo.CancelShipping(txCtx, tx, orderUUID)
	if err != nil {
		return v1dto.CancelShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update order status after shipping cancellation", err)
	}

	eventNote := fmt.Sprintf("Hủy vận đơn %s (%s) thành công.", strings.ToUpper(providerName), shippingCode)
	if input.Reason != "" {
		eventNote += fmt.Sprintf(" Lý do: %s", input.Reason)
	}

	_, err = s.trackingRepo.InsertEvent(reqCtx, tx, sqlc.InsertTrackingEventParams{
		EventOrderID: order.OrderID,
		EventStatus:  "shipping_cancelled",
		EventNote:    &eventNote,
		EventSource:  "admin",
	})
	if err != nil {
		return v1dto.CancelShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to record tracking event", err)
	}
	if err := tx.Commit(reqCtx); err != nil {
		return v1dto.CancelShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	_ = s.cache.Clear(fmt.Sprintf("order:%s", orderUUID.String()))

	logger.Log.Info().
		Str("order_number", order.OrderNumber).
		Str("shipping_code", shippingCode).
		Msg("Order shipping cancelled successfully")

	return v1dto.CancelShippingResponseDTO{
		OrderUUID:    orderUUID.String(),
		OrderNumber:  updatedOrder.OrderNumber,
		ShippingCode: shippingCode,
		OrderStatus:  "Confirmed",
		Message:      "Hủy vận đơn thành công. Đơn hàng đã quay về trạng thái Chờ giao hàng.",
	}, nil
}

func (s *orderService) ConfirmOrderDelivery(ctx *gin.Context, orderUUID uuid.UUID) (v1dto.ConfirmDeliveryResponseDTO, error) {
	reqCtx := ctx.Request.Context()

	user, err := s.getUser(ctx)
	if err != nil {
		return v1dto.ConfirmDeliveryResponseDTO{}, err
	}

	order, err := s.orderRepo.GetByUUID(reqCtx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v1dto.ConfirmDeliveryResponseDTO{}, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return v1dto.ConfirmDeliveryResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
	}

	isAdminOrMod := user.UserLevel == RoleAdmin || user.UserLevel == RoleModerator
	if order.OrderUserID != user.UserID && !isAdminOrMod {
		return v1dto.ConfirmDeliveryResponseDTO{}, utils.NewError(utils.ErrCodeForbidden, "Access denied")
	}

	if order.OrderStatus == OrderStatusDelivered {
		return v1dto.ConfirmDeliveryResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Order is already marked as delivered")
	}

	if order.OrderStatus != OrderStatusShipped {
		var errMsg string
		switch order.OrderStatus {
		case OrderStatusPending:
			errMsg = "Order is pending and has not been processed yet"
		case OrderStatusConfirmed:
			errMsg = "Order is confirmed but has not been shipped yet. Delivery can only be confirmed once shipped"
		case OrderStatusCancelled:
			errMsg = "Cannot confirm delivery for a cancelled order"
		case OrderStatusReturned:
			errMsg = "Cannot confirm delivery for a returned order"
		default:
			errMsg = fmt.Sprintf("Cannot confirm delivery for an order with status: %s", v1dto.MapOrderStatusToString(order.OrderStatus))
		}
		return v1dto.ConfirmDeliveryResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest, errMsg)
	}

	txCtx, cancel := context.WithTimeout(reqCtx, 10*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return v1dto.ConfirmDeliveryResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to begin transaction", err)
	}
	defer tx.Rollback(txCtx)

	updatedOrder, err := s.orderRepo.MarkDeliveredByUUID(txCtx, tx, orderUUID)
	if err != nil {
		return v1dto.ConfirmDeliveryResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update order to delivered", err)
	}

	eventNote := "Customer confirmed order receipt."
	_, err = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
		EventOrderID: order.OrderID,
		EventStatus:  "delivered",
		EventNote:    &eventNote,
		EventSource:  "user",
	})
	if err != nil {
		return v1dto.ConfirmDeliveryResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to record tracking event", err)
	}

	if err := tx.Commit(txCtx); err != nil {
		return v1dto.ConfirmDeliveryResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	_ = s.cache.Clear(fmt.Sprintf("order:%s", orderUUID.String()))
	_ = s.cache.Clear("orders:*")

	go func() {
		_ = s.notificationService.Send(context.Background(), v1dto.SendNotificationInput{
			UserID: updatedOrder.OrderUserID,
			Type:   "order_completed",
			Title:  "Order Completed",
			Body:   fmt.Sprintf("Thank you! Order #%s is completed. You can now write a review.", updatedOrder.OrderNumber),
			Data:   map[string]any{"order_uuid": updatedOrder.OrderUuid.String()},
		})
	}()

	var deliveredAt *time.Time
	if updatedOrder.OrderDeliveredAt.Valid {
		deliveredAt = &updatedOrder.OrderDeliveredAt.Time
	}

	return v1dto.ConfirmDeliveryResponseDTO{
		OrderUUID:         updatedOrder.OrderUuid.String(),
		OrderNumber:       updatedOrder.OrderNumber,
		OrderStatus:       updatedOrder.OrderStatus,
		StatusName:        v1dto.MapOrderStatusToString(updatedOrder.OrderStatus),
		StatusText:        v1dto.MapOrderStatusToVietnamese(updatedOrder.OrderStatus),
		PaymentMethod:     v1dto.MapPaymentMethodToString(updatedOrder.OrderPaymentMethod),
		PaymentStatus:     updatedOrder.OrderPaymentStatus,
		PaymentStatusName: v1dto.MapPaymentStatusToString(updatedOrder.OrderPaymentStatus),
		DeliveredAt:       deliveredAt,
		Message:           "Order delivery confirmed successfully. You can now leave a product review or request a return within 7 days.",
	}, nil
}

func (s *orderService) GetPrintBill(ctx *gin.Context, orderUUID uuid.UUID) (v1dto.PrintBillResponseDTO, error) {
	reqCtx := ctx.Request.Context()

	order, err := s.orderRepo.GetByUUID(reqCtx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v1dto.PrintBillResponseDTO{}, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return v1dto.PrintBillResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
	}

	if order.OrderShippingCode == nil || strings.TrimSpace(*order.OrderShippingCode) == "" {
		return v1dto.PrintBillResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Order does not have a valid shipping code")
	}

	shippingCode := *order.OrderShippingCode
	providerName := "ghn"
	if order.OrderShippingProvider != nil && strings.TrimSpace(*order.OrderShippingProvider) != "" {
		providerName = *order.OrderShippingProvider
	}

	provider, err := s.shippingFactory.GetProvider(providerName)
	if err != nil {
		return v1dto.PrintBillResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get shipping provider", err)
	}

	res, err := provider.GetPrintBillURL(reqCtx, shippingCode)
	if err != nil {
		logger.Log.Error().
			Err(err).
			Str("provider", providerName).
			Str("shipping_code", shippingCode).
			Msg("Failed to get print bill URL from provider")
		return v1dto.PrintBillResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get print bill URL from provider", err)
	}

	return v1dto.PrintBillResponseDTO{
		OrderUUID:    orderUUID.String(),
		OrderNumber:  order.OrderNumber,
		ShippingCode: shippingCode,
		Token:        res.Token,
		PrintA5:      res.PrintA5,
		Print80x80:   res.Print80,
		Print52x70:   res.Print52,
	}, nil
}

func (s *orderService) GetProvinces(ctx *gin.Context) ([]v1dto.ProvinceDTO, error) {
	reqCtx := ctx.Request.Context()

	cacheKey := "shipping:ghn:provinces"
	var cached []v1dto.ProvinceDTO
	if err := s.cache.Get(cacheKey, &cached); err == nil && len(cached) > 0 {
		return cached, nil
	}

	provider, err := s.shippingFactory.GetProvider("ghn")
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeInternal, "GHN provider not configured")
	}

	ghnAdapter, ok := provider.(*shipping.GHNAdapter)
	if !ok {
		return nil, utils.NewError(utils.ErrCodeInternal, "Invalid GHN provider adapter")
	}

	provinces, err := ghnAdapter.GetProvinces(reqCtx)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to get provinces from GHN", err)
	}

	_ = s.cache.Set(cacheKey, provinces, 24*time.Hour)

	return provinces, nil
}

func (s *orderService) GetDistricts(ctx *gin.Context, provinceID int) ([]v1dto.DistrictDTO, error) {

	reqCtx := ctx.Request.Context()

	cacheKey := fmt.Sprintf("shipping:ghn:districts:%d", provinceID)
	var cached []v1dto.DistrictDTO
	if err := s.cache.Get(cacheKey, &cached); err == nil && len(cached) > 0 {
		return cached, nil
	}

	provider, err := s.shippingFactory.GetProvider("ghn")
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeInternal, "GHN provider not configured")
	}

	ghnAdapter, ok := provider.(*shipping.GHNAdapter)
	if !ok {
		return nil, utils.NewError(utils.ErrCodeInternal, "Invalid GHN provider adapter")
	}

	districts, err := ghnAdapter.GetDistricts(reqCtx, provinceID)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to fetch districts from GHN", err)

	}
	_ = s.cache.Set(cacheKey, districts, addressCacheTTL)

	return districts, nil
}

func (s *orderService) GetWards(ctx *gin.Context, districtID int) ([]v1dto.WardDTO, error) {
	reqCtx := ctx.Request.Context()

	cacheKey := fmt.Sprintf("shipping:ghn:wards:%d", districtID)
	var cached []v1dto.WardDTO
	if err := s.cache.Get(cacheKey, &cached); err == nil && len(cached) > 0 {
		return cached, nil
	}

	provider, err := s.shippingFactory.GetProvider("ghn")
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeInternal, "GHN provider not configured")
	}

	ghnAdapter, ok := provider.(*shipping.GHNAdapter)
	if !ok {
		return nil, utils.NewError(utils.ErrCodeInternal, "Invalid GHN provider adapter")
	}

	wards, err := ghnAdapter.GetWards(reqCtx, districtID)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to fetch wards from GHN", err)
	}
	_ = s.cache.Set(cacheKey, wards, addressCacheTTL)

	return wards, nil
}

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

		eventNote := fmt.Sprintf("Kho đã nhận hàng hoàn về kho thành công và khôi phục tồn kho. Đang chờ kế toán thực hiện hoàn tiền. Ghi chú: %s", noteStr)
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

func (s *orderService) SyncOrderTracking(ctx *gin.Context, orderUUID uuid.UUID) (v1dto.SyncTrackingResponseDTO, error) {
	reqCtx := ctx.Request.Context()

	user, err := s.getUser(ctx)
	if err != nil {
		return v1dto.SyncTrackingResponseDTO{}, err
	}

	order, err := s.orderRepo.GetByUUID(reqCtx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v1dto.SyncTrackingResponseDTO{}, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return v1dto.SyncTrackingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
	}

	isAdminOrMod := user.UserLevel == RoleAdmin || user.UserLevel == RoleModerator
	if order.OrderUserID != user.UserID && !isAdminOrMod {
		return v1dto.SyncTrackingResponseDTO{}, utils.NewError(utils.ErrCodeForbidden, "Access denied")
	}

	if order.OrderShippingCode == nil || strings.TrimSpace(*order.OrderShippingCode) == "" {
		return v1dto.SyncTrackingResponseDTO{}, utils.NewError(
			utils.ErrCodeBadRequest,
			"Order does not have a shipping tracking code to synchronize",
		)
	}
	shippingCode := strings.TrimSpace(*order.OrderShippingCode)

	providerName := "ghn"
	if order.OrderShippingProvider != nil && strings.TrimSpace(*order.OrderShippingProvider) != "" {
		providerName = strings.TrimSpace(*order.OrderShippingProvider)
	}

	provider, err := s.shippingFactory.GetProvider(providerName)
	if err != nil {
		return v1dto.SyncTrackingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Unsupported shipping provider", err)
	}

	// Call shipping provider API to get tracking info
	trackingInfo, err := provider.GetTrackingInfo(reqCtx, shippingCode)
	if err != nil {
		logger.Log.Error().
			Err(err).
			Str("order_number", order.OrderNumber).
			Str("shipping_code", shippingCode).
			Msg("Failed to fetch tracking info from shipping provider")
		return v1dto.SyncTrackingResponseDTO{}, utils.WrapError(
			utils.ErrCodeBadGateway,
			"Failed to query tracking information from GHN gateway",
			err,
		)
	}

	ghnStatus := strings.ToLower(strings.TrimSpace(trackingInfo.Status))

	isDeliveredNow := false
	if ghnStatus == "delivered" && order.OrderStatus != OrderStatusDelivered {
		txCtx, cancel := context.WithTimeout(reqCtx, 10*time.Second)
		defer cancel()

		tx, err := db.BeginTx(txCtx)
		if err == nil {
			defer tx.Rollback(txCtx)

			_, err = s.orderRepo.MarkDeliveredByShippingCode(txCtx, tx, shippingCode)
			if err == nil {
				eventNote := fmt.Sprintf("GHN Sync: %s", trackingInfo.StatusText)
				_, _ = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
					EventOrderID: order.OrderID,
					EventStatus:  "delivered",
					EventNote:    &eventNote,
					EventSource:  "ghn_sync",
				})

				if err := tx.Commit(txCtx); err == nil {
					isDeliveredNow = true
					_ = s.cache.Clear(fmt.Sprintf("order:%s", orderUUID.String()))
					_ = s.cache.Clear("orders:*")

					// Realtime notify
					go func() {
						_ = s.notificationService.Send(context.Background(), v1dto.SendNotificationInput{
							UserID: order.OrderUserID,
							Type:   "order_delivered",
							Title:  "Order Delivered Successfully",
							Body:   fmt.Sprintf("Your order #%s has been delivered by GHN.", order.OrderNumber),
							Data: map[string]any{
								"order_uuid":    orderUUID.String(),
								"shipping_code": shippingCode,
							},
						})
					}()
				}
			}
		}
	} else {
		// Write log if this is has a new status
		existingEvents, _ := s.trackingRepo.GetByOrderID(reqCtx, order.OrderID)
		hasStatusAlready := false
		for _, ev := range existingEvents {
			if strings.EqualFold(ev.EventStatus, ghnStatus) {
				hasStatusAlready = true
				break
			}
		}

		if !hasStatusAlready && ghnStatus != "" {
			eventNote := fmt.Sprintf("GHN tracking update: %s", trackingInfo.StatusText)
			_, _ = s.trackingRepo.InsertEvent(reqCtx, nil, sqlc.InsertTrackingEventParams{
				EventOrderID: order.OrderID,
				EventStatus:  ghnStatus,
				EventNote:    &eventNote,
				EventSource:  "ghn_sync",
			})
		}
	}

	// Get all latest timeline
	updatedEvents, _ := s.trackingRepo.GetByOrderID(reqCtx, order.OrderID)

	return v1dto.SyncTrackingResponseDTO{
		OrderUUID:        order.OrderUuid.String(),
		OrderNumber:      order.OrderNumber,
		ShippingCode:     shippingCode,
		ShippingProvider: strings.ToUpper(providerName),
		CurrentStatus:    ghnStatus,
		StatusText:       trackingInfo.StatusText,
		IsDelivered:      order.OrderStatus == OrderStatusDelivered || isDeliveredNow,
		LastSyncAt:       time.Now().UTC(),
		Timeline:         v1dto.MapToEventDTOs(updatedEvents),
		Message:          "Order tracking information synchronized successfully from GHN.",
	}, nil
}

func (s *orderService) AdminSyncActiveShippingOrders(ctx *gin.Context) (v1dto.SyncActiveShippingStatsDTO, error) {
	reqCtx := ctx.Request.Context()

	user, err := s.getUser(ctx)
	if err != nil {
		return v1dto.SyncActiveShippingStatsDTO{}, err
	}

	if user.UserLevel != RoleAdmin && user.UserLevel != RoleModerator {
		return v1dto.SyncActiveShippingStatsDTO{}, utils.NewError(utils.ErrCodeForbidden, "Access denied")
	}

	return s.SyncActiveShippingOrders(reqCtx)
}

func (s *orderService) SyncActiveShippingOrders(ctx context.Context) (v1dto.SyncActiveShippingStatsDTO, error) {
	const batchLimit int32 = 50
	orders, err := s.orderRepo.GetActiveShippedOrders(ctx, batchLimit)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to query active shipped orders for sync cronjob")
		return v1dto.SyncActiveShippingStatsDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to query active shipped orders", err)
	}

	stats := v1dto.SyncActiveShippingStatsDTO{
		ScannedCount: len(orders),
		SyncedAt:     time.Now().UTC(),
	}

	if len(orders) == 0 {
		logger.Log.Debug().Msg("No active shipped orders found for cronjob synchronization")
		return stats, nil
	}

	logger.Log.Info().
		Int("orders_to_scan", len(orders)).
		Msg("Starting background shipping sync cronjob batch")

	for _, order := range orders {
		select {
		case <-ctx.Done():
			logger.Log.Warn().Msg("Shipping cronjob synchronization aborted due to context cancellation")
			return stats, ctx.Err()
		default:
		}

		if order.OrderShippingCode == nil || strings.TrimSpace(*order.OrderShippingCode) == "" {
			continue
		}
		shippingCode := strings.TrimSpace(*order.OrderShippingCode)

		providerName := "ghn"
		if order.OrderShippingProvider != nil && strings.TrimSpace(*order.OrderShippingProvider) != "" {
			providerName = strings.TrimSpace(*order.OrderShippingProvider)
		}

		provider, err := s.shippingFactory.GetProvider(providerName)
		if err != nil {
			logger.Log.Warn().
				Str("provider", providerName).
				Str("order_number", order.OrderNumber).
				Msg("Shipping provider not supported in sync cronjob")
			stats.FailedCount++
			continue
		}

		trackingInfo, err := provider.GetTrackingInfo(ctx, shippingCode)
		if err != nil {
			logger.Log.Warn().
				Err(err).
				Str("order_number", order.OrderNumber).
				Str("shipping_code", shippingCode).
				Msg("Failed to query tracking info in sync cronjob")
			stats.FailedCount++
			time.Sleep(200 * time.Millisecond)
			continue
		}

		stats.SyncedCount++
		ghnStatus := strings.ToLower(strings.TrimSpace(trackingInfo.Status))

		if ghnStatus == "delivered" && order.OrderStatus != OrderStatusDelivered {
			txCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			tx, err := db.BeginTx(txCtx)
			if err != nil {
				cancel()
				logger.Log.Error().Err(err).Str("order_number", order.OrderNumber).Msg("Failed to begin transaction for order delivery update")
				stats.FailedCount++
				continue
			}

			updatedOrder, err := s.orderRepo.MarkDeliveredByShippingCode(txCtx, tx, shippingCode)
			if err != nil {
				_ = tx.Rollback(txCtx)
				cancel()
				logger.Log.Error().Err(err).Str("order_number", order.OrderNumber).Msg("Failed to mark order delivered in sync cronjob")
				stats.FailedCount++
				continue
			}

			eventNote := fmt.Sprintf("Cronjob Sync: %s", trackingInfo.StatusText)
			_, err = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
				EventOrderID: order.OrderID,
				EventStatus:  "delivered",
				EventNote:    &eventNote,
				EventSource:  "ghn_cron",
			})
			if err != nil {
				_ = tx.Rollback(txCtx)
				cancel()
				logger.Log.Error().Err(err).Str("order_number", order.OrderNumber).Msg("Failed to record tracking event in sync cronjob")
				stats.FailedCount++
				continue
			}

			if err := tx.Commit(txCtx); err != nil {
				cancel()
				logger.Log.Error().Err(err).Str("order_number", order.OrderNumber).Msg("Failed to commit transaction in sync cronjob")
				stats.FailedCount++
				continue
			}
			cancel()

			stats.DeliveredCount++
			_ = s.cache.Clear(fmt.Sprintf("order:%s", order.OrderUuid.String()))
			_ = s.cache.Clear("orders:*")

			go func(ord sqlc.Order, code string) {
				_ = s.notificationService.Send(context.Background(), v1dto.SendNotificationInput{
					UserID: ord.OrderUserID,
					Type:   "order_delivered",
					Title:  "Order Delivered Successfully",
					Body:   fmt.Sprintf("Your order #%s has been delivered by GHN.", ord.OrderNumber),
					Data: map[string]any{
						"order_uuid":    ord.OrderUuid.String(),
						"shipping_code": code,
					},
				})
			}(updatedOrder, shippingCode)

			logger.Log.Info().
				Str("order_number", order.OrderNumber).
				Str("shipping_code", shippingCode).
				Msg("Order transitioned to DELIVERED via shipping sync cronjob")
		} else if ghnStatus != "" {
			existingEvents, err := s.trackingRepo.GetByOrderID(ctx, order.OrderID)
			if err == nil {
				hasStatusAlready := false
				for _, ev := range existingEvents {
					if strings.EqualFold(ev.EventStatus, ghnStatus) {
						hasStatusAlready = true
						break
					}
				}
				if !hasStatusAlready {
					eventNote := fmt.Sprintf("GHN update: %s", trackingInfo.StatusText)
					_, _ = s.trackingRepo.InsertEvent(ctx, nil, sqlc.InsertTrackingEventParams{
						EventOrderID: order.OrderID,
						EventStatus:  ghnStatus,
						EventNote:    &eventNote,
						EventSource:  "ghn_cron",
					})
				}
			}
		}

		time.Sleep(200 * time.Millisecond)
	}

	logger.Log.Info().
		Int("scanned", stats.ScannedCount).
		Int("synced", stats.SyncedCount).
		Int("delivered", stats.DeliveredCount).
		Int("failed", stats.FailedCount).
		Msg("Shipping sync cronjob batch completed successfully")

	return stats, nil
}
