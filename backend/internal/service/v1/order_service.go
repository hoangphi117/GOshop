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
	"goshop/internal/middleware"
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
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	txManager           db.TxManager
	pricingService      OrderPricingService
	inventoryService    InventoryService
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

func NewOrderService(txManager db.TxManager,
	pricingService OrderPricingService,
	inventoryService InventoryService,
	orderRepo repository.OrderRepository,
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
		txManager:           txManager,
		pricingService:      pricingService,
		inventoryService:    inventoryService,
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
	return middleware.GetUserFromContext(ctx, s.userRepo)
}

func (s *orderService) getUserUUID(ctx *gin.Context) (uuid.UUID, error) {
	return middleware.GetUserUUID(ctx)
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

func (s *orderService) CreateOrder(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateOrderInput) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {

	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeNotFound, "User not found")
		}
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "Failed to get user", err)
	}

	// Validate duplicate products in the order
	productMapInput := make(map[string]int32)
	for _, item := range input.Items {
		if _, exists := productMapInput[item.ProductUUID]; exists {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Duplicate product in order")
		}
		productMapInput[item.ProductUUID] = item.Quantity
	}

	deductItems := make([]DeductItem, len(input.Items))
	for i, item := range input.Items {
		deductItems[i] = DeductItem{
			ProductUUID: item.ProductUUID,
			Quantity:    item.Quantity,
		}
	}

	/* ======================TRANSACTION START ====================== */
	var order sqlc.Order

	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		tx, _ := db.GetTx(txCtx)

		// 1. Lock products and deduct stock via InventoryService
		productMap, err := s.inventoryService.LockAndDeductStock(txCtx, deductItems)
		if err != nil {
			return err
		}

		priceInputs := make([]ItemPriceInput, len(input.Items))
		for i, item := range input.Items {
			p := productMap[item.ProductUUID]
			priceInputs[i] = ItemPriceInput{
				ProductID:   p.ProductID,
				ProductName: p.ProductName,
				Price:       utils.NumericToFloat(p.ProductPrice),
				Quantity:    item.Quantity,
			}
		}

		pricingResult, err := s.pricingService.CalculateOrderTotals(txCtx, priceInputs, input.CouponCode, user.UserID, input.ShippingFee, time.Now())
		if err != nil {
			return err
		}

		if pricingResult.AppliedCouponID != nil {
			if err := s.couponRepo.IncrementUsedCount(txCtx, tx, *pricingResult.AppliedCouponID); err != nil {
				return utils.WrapError(utils.ErrCodeInternal, "Failed to increment coupon usage count", err)
			}
		}

		orderNumber := GenerateOrderNumber(user.UserID)

		createOrderParams := sqlc.CreateOrderParams{
			OrderNumber:          orderNumber,
			OrderUserID:          user.UserID,
			OrderShippingName:    input.ShippingName,
			OrderShippingPhone:   input.ShippingPhone,
			OrderShippingAddress: input.ShippingAddress,
			OrderShippingFee:     utils.FloatToNumeric(pricingResult.ShippingFee),
			OrderCouponID:        pricingResult.AppliedCouponID,
			OrderCouponCode:      pricingResult.AppliedCouponCode,
			OrderSubtotalAmount:  utils.FloatToNumeric(pricingResult.SubtotalAmount),
			OrderDiscountAmount:  utils.FloatToNumeric(pricingResult.DiscountAmount),
			OrderTotalAmount:     utils.FloatToNumeric(pricingResult.FinalTotalAmount),
			OrderPaymentMethod:   input.PaymentMethod,
			OrderPaymentStatus:   OrderPaymentStatusUnpaid,
			OrderNote:            &input.Note,
			OrderStatus:          OrderStatusPending,
		}

		createdOrder, err := s.orderRepo.Create(txCtx, createOrderParams)
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "failed to create order", err)
		}
		order = createdOrder

		if pricingResult.AppliedCouponID != nil {
			_, err = s.couponRepo.RecordUserUsage(txCtx, tx, sqlc.RecordCouponUsageParams{
				CouponUsageCouponID: *pricingResult.AppliedCouponID,
				CouponUsageUserID:   user.UserID,
				CouponUsageOrderID:  &order.OrderID,
			})
			if err != nil {
				return utils.WrapError(utils.ErrCodeInternal, "failed to record coupon usage", err)
			}
		}

		purchasedProductIDs := make([]int32, len(pricingResult.Items))

		for i, item := range pricingResult.Items {
			purchasedProductIDs[i] = item.ProductID
			createdItemParams := sqlc.CreateOrderItemParams{
				OrderItemOrderID:     order.OrderID,
				OrderItemProductID:   item.ProductID,
				OrderItemProductName: item.ProductName,
				OrderItemPrice:       utils.FloatToNumeric(item.Price),
				OrderItemQuantity:    item.Quantity,
				OrderItemSubtotal:    utils.FloatToNumeric(item.Subtotal),
			}

			_, err := s.orderItemRepo.Create(txCtx, tx, createdItemParams)
			if err != nil {
				return utils.WrapError(utils.ErrCodeInternal, "failed to create order item", err)
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
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "failed to insert tracking event", err)
		}

		return nil
	})

	if err != nil {
		return sqlc.Order{}, nil, err
	}
	/* ====================== TRANSACTION END ====================== */

	// Get order items with details after the transaction is committed
	orderItemsWithDetails, err := s.orderItemRepo.GetByOrderID(ctx, order.OrderID)
	if err != nil {
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "failed to get order items", err)
	}

	// Clear related caches
	s.invalidateOrderCreationCaches(user.UserUuid)

	// Send order creation notification asynchronously
	notifyErr := s.notificationService.Send(ctx, v1dto.SendNotificationInput{
		UserID: order.OrderUserID,
		Type:   "order_created",
		Title:  "Order Placed Successfully",
		Body:   fmt.Sprintf("Your order #%s has been placed successfully.", order.OrderNumber),
		Data:   map[string]any{"order_uuid": order.OrderUuid.String()},
	})
	if notifyErr != nil {
		// Log the error but do not fail the order creation process
		logger.Log.Warn().
			Err(notifyErr).
			Int32("user_id", order.OrderUserID).
			Str("order_number", order.OrderNumber).
			Msg("Failed to send order creation notification")
	}

	return order, orderItemsWithDetails, nil
}

func (s *orderService) invalidateOrderCreationCaches(userUUID uuid.UUID) {
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
}

func (s *orderService) GetMyOrders(ctx *gin.Context, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error) {
	context := ctx.Request.Context()

	user, err := middleware.GetUserFromContext(ctx, s.userRepo)
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
		if items, err := s.orderItemRepo.GetByOrderID(context, o.OrderID); err == nil {
			itemsDTO := make([]v1dto.OrderItemDTO, len(items))
			for j, item := range items {
				itemsDTO[j] = v1dto.MapToOrderItemRowDTO(item)
			}
			ordersDTO[i].Items = itemsDTO
		}
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

	user, err := middleware.GetUserFromContext(ctx, s.userRepo)
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
		if items, err := s.orderItemRepo.GetByOrderID(reqCtx, o.OrderID); err == nil {
			itemsDTO := make([]v1dto.OrderItemDTO, len(items))
			for j, item := range items {
				itemsDTO[j] = v1dto.MapToOrderItemRowDTO(item)
			}
			orderDTOs[i].Items = itemsDTO
		}
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

	user, err := middleware.GetUserFromContext(ctx, s.userRepo)
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

	restoreItems := make([]RestoreItem, len(orderItems))
	for i, item := range orderItems {
		restoreItems[i] = RestoreItem{
			ProductID: item.OrderItemProductID,
			Quantity:  item.OrderItemQuantity,
		}
	}

	if err := s.inventoryService.RestoreStock(txCtx, restoreItems); err != nil {
		return err
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

	user, err := middleware.GetUserFromContext(ctx, s.userRepo)
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

	userUUID, err := middleware.GetUserUUID(ctx)
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

	user, err := middleware.GetUserFromContext(ctx, s.userRepo)
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
