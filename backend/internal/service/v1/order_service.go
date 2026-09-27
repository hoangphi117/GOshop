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
	"math/big"
	"slices"
	"strings"
	"time"

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
	RoleMember    int32 = 3

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
	productRepo         repository.ProductRepository
	userRepo            repository.UserRepository
	cartService         CartService
	cartRepo            repository.CartRepository
	couponRepo          repository.CouponRepository
	trackingRepo        repository.TrackingRepository
	paymentRepo         repository.PaymentRepository
	notificationService NotificationService
	cache               cache.RedisCacheService
	invoiceService      OrderInvoiceService
	adminService        OrderAdminService
}

func NewOrderService(
	txManager db.TxManager,
	pricingService OrderPricingService,
	inventoryService InventoryService,
	orderRepo repository.OrderRepository,
	orderItemRepo repository.OrderItemRepository,
	productRepo repository.ProductRepository,
	userRepo repository.UserRepository,
	cartService CartService,
	cartRepo repository.CartRepository,
	couponRepo repository.CouponRepository,
	trackingRepo repository.TrackingRepository,
	paymentRepo repository.PaymentRepository,
	notificationService NotificationService,
	redisClient *redis.Client,
) OrderService {
	var cacheService cache.RedisCacheService
	if redisClient != nil {
		cacheService = cache.NewRedisCacheService(redisClient)
	}
	svc := &orderService{
		txManager:           txManager,
		pricingService:      pricingService,
		inventoryService:    inventoryService,
		orderRepo:           orderRepo,
		orderItemRepo:       orderItemRepo,
		productRepo:         productRepo,
		userRepo:            userRepo,
		cartService:         cartService,
		cartRepo:            cartRepo,
		couponRepo:          couponRepo,
		trackingRepo:        trackingRepo,
		paymentRepo:         paymentRepo,
		notificationService: notificationService,
		cache:               cacheService,
	}
	svc.invoiceService = NewOrderInvoiceService(orderRepo, orderItemRepo, userRepo, cacheService)
	svc.adminService = NewOrderAdminService(svc)
	return svc
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

		orderNumber := GenerateOrderNumber(user.UserID)

		// 1. Lock products and deduct stock via InventoryService
		productMap, err := s.inventoryService.LockAndDeductStockWithRef(txCtx, deductItems, orderNumber, "order", &user.UserID, nil)
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
	s.InvalidateOrderCaches(user.UserUuid)

	// Send customer notification asynchronously via event queue
	notifyErr := s.notificationService.SendAsync(ctx, v1dto.SendNotificationInput{
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

	// Notify admins asynchronously via RabbitMQ
	if s.notificationService != nil {
		_ = s.notificationService.NotifyAdminsAsync(ctx, v1dto.AdminNotifyInput{
			Type:  "order",
			Title: "New Order Received",
			Body:  fmt.Sprintf("Order #%s placed for $%.2f", order.OrderNumber, utils.NumericToFloat(order.OrderTotalAmount)),
			Data: map[string]any{
				"order_uuid":   order.OrderUuid.String(),
				"order_number": order.OrderNumber,
				"link":         fmt.Sprintf("/orders/%s", order.OrderUuid),
			},
		})
	}

	return order, orderItemsWithDetails, nil
}

func (s *orderService) GetMyOrders(ctx context.Context, userUUID uuid.UUID, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error) {
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		return []v1dto.OrderSummaryDTO{}, 0, s.handleUserNotFoundError(err)
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

	totalOrders, err := s.orderRepo.CountOrders(ctx, &user.UserID, statusPtr, input.Search)
	if err != nil {
		return []v1dto.OrderSummaryDTO{}, 0, utils.WrapError(utils.ErrCodeInternal, "failed to count orders", err)
	}
	if totalOrders == 0 {
		return []v1dto.OrderSummaryDTO{}, 0, nil
	}

	offset := (input.Page - 1) * input.Limit
	orders, err := s.orderRepo.GetAll(ctx, &user.UserID, statusPtr, input.Search, input.OrderBy,
		input.Sort, input.Limit, offset)
	if err != nil {
		return []v1dto.OrderSummaryDTO{}, 0, utils.WrapError(utils.ErrCodeInternal, "failed to get orders", err)
	}

	ordersDTO := make([]v1dto.OrderSummaryDTO, len(orders))
	for i, o := range orders {
		ordersDTO[i] = v1dto.MapToOrderSummaryDTO(o)
		if items, err := s.orderItemRepo.GetByOrderID(ctx, o.OrderID); err == nil {
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

func (s *orderService) GetOrderByUUID(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (sqlc.GetOrderByUUIDRow, []sqlc.GetOrderItemsByOrderIDRow, error) {
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		return sqlc.GetOrderByUUIDRow{}, nil, s.handleUserNotFoundError(err)
	}

	order, err := s.orderRepo.GetByUUID(ctx, orderUUID)
	if err != nil {
		return sqlc.GetOrderByUUIDRow{}, nil, s.handleOrderNotFoundError(err)
	}

	isAdmin := user.UserLevel == RoleAdmin
	isModerator := user.UserLevel == RoleModerator

	if order.OrderUserID != user.UserID && !isAdmin && !isModerator {
		return sqlc.GetOrderByUUIDRow{}, nil, utils.NewError(utils.ErrCodeForbidden, "You are not authorized to view this order")
	}

	orderItems, err := s.orderItemRepo.GetByOrderID(ctx, order.OrderID)
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

func (s *orderService) GetAllOrders(ctx context.Context, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error) {
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

	totalRecords, err := s.orderRepo.CountOrders(ctx, targetUserID, statusPtr, input.Search)
	if err != nil {
		return []v1dto.OrderSummaryDTO{}, 0, utils.WrapError(utils.ErrCodeInternal, "failed to count orders", err)
	}
	if totalRecords == 0 {
		return []v1dto.OrderSummaryDTO{}, 0, nil
	}

	offset := (input.Page - 1) * input.Limit
	orders, err := s.orderRepo.GetAll(ctx, targetUserID, statusPtr, input.Search, input.OrderBy, input.Sort, input.Limit, offset)
	if err != nil {
		return []v1dto.OrderSummaryDTO{}, 0, utils.WrapError(utils.ErrCodeInternal, "failed to get orders", err)
	}

	orderDTOs := make([]v1dto.OrderSummaryDTO, len(orders))
	for i, o := range orders {
		orderDTOs[i] = v1dto.MapToOrderSummaryDTO(o)
		if items, err := s.orderItemRepo.GetByOrderID(ctx, o.OrderID); err == nil {
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

func (s *orderService) UpdateOrderStatus(ctx context.Context, input sqlc.UpdateOrderStatusParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {
	existingOrder, err := s.orderRepo.GetByUUID(ctx, input.OrderUuid)
	if err != nil {
		return sqlc.Order{}, nil, s.handleOrderNotFoundError(err)
	}

	if !isValidStatusTransition(existingOrder.OrderStatus, input.OrderStatus) {
		errMsg := fmt.Sprintf("Cannot change order status from '%s' to '%s'",
			v1dto.MapOrderStatusToString(existingOrder.OrderStatus),
			v1dto.MapOrderStatusToString(input.OrderStatus),
		)
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, errMsg)
	}

	updatedOrder, err := s.orderRepo.UpdateStatus(ctx, nil, input)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
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
		_, _ = s.trackingRepo.InsertEvent(ctx, nil, sqlc.InsertTrackingEventParams{
			EventOrderID: updatedOrder.OrderID,
			EventStatus:  v1dto.MapOrderStatusToString(input.OrderStatus),
			EventNote:    &statusNote,
			EventSource:  "admin",
		})
	}

	orderItems, err := s.orderItemRepo.GetByOrderID(ctx, updatedOrder.OrderID)
	if err != nil {
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "failed to get order items", err)
	}

	if s.cache != nil {
		if err := s.cache.Clear("orders:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear order cache")
		}
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

func (s *orderService) CancelOrder(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, reason *string) (sqlc.Order, error) {
	// 1. Lấy thông tin User & Order
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		return sqlc.Order{}, s.handleUserNotFoundError(err)
	}

	orderRow, err := s.orderRepo.GetByUUID(ctx, orderUUID)
	if err != nil {
		return sqlc.Order{}, s.handleOrderNotFoundError(err)
	}

	// 2. Validate điều kiện hủy đơn (Domain Rule)
	if err := s.validateOrderCancellation(user, orderRow); err != nil {
		return sqlc.Order{}, err
	}

	// 3. Lấy danh sách sản phẩm để chuẩn bị hoàn kho
	orderItems, err := s.orderItemRepo.GetByOrderID(ctx, orderRow.OrderID)
	if err != nil {
		return sqlc.Order{}, utils.WrapError(utils.ErrCodeInternal, "failed to get order items", err)
	}

	var cancelledOrder sqlc.Order

	// 4. Thực thi Transaction điều phối các Sub-services (Orchestration)
	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		tx, _ := db.GetTx(txCtx)

		// 4.1. Hủy trạng thái đơn
		updatedOrder, err := s.orderRepo.CancelOrder(txCtx, tx, orderUUID)
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "failed to cancel order", err)
		}
		cancelledOrder = updatedOrder

		// 4.2. Hoàn tồn kho qua InventoryService
		if err := s.inventoryService.RestoreStockWithRef(txCtx, toRestoreItems(orderItems), orderRow.OrderNumber, "order_cancel", &user.UserID, reason); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "failed to restore product stock", err)
		}

		// 4.3. Ghi nhận Tracking Event
		if err := s.recordCancellationTracking(txCtx, tx, user, orderRow, reason); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return sqlc.Order{}, err
	}

	// 5. Side-effects hậu commit (Cache & Notification)
	s.InvalidateOrderCaches(user.UserUuid)
	s.sendCancellationNotification(ctx, orderRow)

	return cancelledOrder, nil
}

func (s *orderService) validateOrderCancellation(user sqlc.User, order sqlc.GetOrderByUUIDRow) error {
	isAdminOrMod := user.UserLevel == RoleAdmin || user.UserLevel == RoleModerator
	if order.OrderUserID != user.UserID && !isAdminOrMod {
		return utils.NewError(utils.ErrCodeForbidden, "You are not authorized to cancel this order")
	}

	if order.OrderStatus != OrderStatusPending && order.OrderStatus != OrderStatusConfirmed {
		return utils.NewError(utils.ErrCodeBadRequest, "Only pending or confirmed orders can be cancelled")
	}

	return nil
}

func toRestoreItems(orderItems []sqlc.GetOrderItemsByOrderIDRow) []RestoreItem {
	items := make([]RestoreItem, len(orderItems))
	for i, item := range orderItems {
		items[i] = RestoreItem{
			ProductID: item.OrderItemProductID,
			Quantity:  item.OrderItemQuantity,
		}
	}
	return items
}

func (s *orderService) recordCancellationTracking(ctx context.Context, tx pgx.Tx, user sqlc.User, order sqlc.GetOrderByUUIDRow, reason *string) error {
	note := "Order cancelled."
	if reason != nil && strings.TrimSpace(*reason) != "" {
		note = fmt.Sprintf("Order cancelled. Reason: %s", strings.TrimSpace(*reason))
	}

	eventSource := "user"
	if (user.UserLevel == RoleAdmin || user.UserLevel == RoleModerator) && order.OrderUserID != user.UserID {
		eventSource = "admin"
	}

	_, err := s.trackingRepo.InsertEvent(ctx, tx, sqlc.InsertTrackingEventParams{
		EventOrderID: order.OrderID,
		EventStatus:  "cancelled",
		EventNote:    &note,
		EventSource:  eventSource,
	})
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "failed to insert tracking event", err)
	}
	return nil
}

func (s *orderService) sendCancellationNotification(ctx context.Context, order sqlc.GetOrderByUUIDRow) {
	err := s.notificationService.Send(ctx, v1dto.SendNotificationInput{
		UserID: order.OrderUserID,
		Type:   "order_cancelled",
		Title:  "Order Cancelled",
		Body:   fmt.Sprintf("Your order #%s has been cancelled.", order.OrderNumber),
		Data:   map[string]any{"order_uuid": order.OrderUuid.String()},
	})
	if err != nil {
		logger.Log.Warn().
			Err(err).
			Int32("user_id", order.OrderUserID).
			Str("order_number", order.OrderNumber).
			Msg("Failed to send order cancellation notification")
	}
}

func (s *orderService) UpdateOrderShippingInfo(ctx context.Context, userUUID uuid.UUID, input sqlc.UpdateOrderShippingInfoParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		return sqlc.Order{}, nil, s.handleUserNotFoundError(err)
	}

	existingOrder, err := s.orderRepo.GetByUUID(ctx, input.OrderUuid)
	if err != nil {
		return sqlc.Order{}, nil, s.handleOrderNotFoundError(err)
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

	updatedOrder, err := s.orderRepo.UpdateOrderShippingInfo(ctx, input)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			// Possible reasons: order does not exist, OR status has changed (race condition
			// was blocked at the right time) -> return a clear error, suggest reloading the page.
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeConflict, "Order status has changed or order not found, please refresh and try again")
		}
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "Failed to update shipping info", err)
	}

	orderItems, err := s.orderItemRepo.GetByOrderID(ctx, updatedOrder.OrderID)
	if err != nil {
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "Failed to get order items", err)
	}

	s.InvalidateOrderCaches(existingOrder.UserUuid)
	return updatedOrder, orderItems, nil
}

func (s *orderService) UpdateOrderPaymentStatus(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateOrderPaymentStatusInput) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {
	existingOrder, err := s.orderRepo.GetByUUID(ctx, orderUUID)
	if err != nil {
		return sqlc.Order{}, nil, s.handleOrderNotFoundError(err)
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

	updatedOrder, err := s.orderRepo.UpdateOrderPaymentStatus(ctx, nil, updateParams)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeConflict, "Payment status has changed by another request, please refresh")
		}
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "Failed to update payment status", err)
	}

	orderItems, err := s.orderItemRepo.GetByOrderID(ctx, updatedOrder.OrderID)
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

	if s.cache != nil {
		userKey := fmt.Sprintf("orders:user:%s:*", existingOrder.UserUuid.String())
		_ = s.cache.Clear(userKey)
		_ = s.cache.Clear("orders:admin:*")
		_ = s.cache.Del(fmt.Sprintf("invoice:pdf:%s", existingOrder.OrderUuid.String()))
	}

	return updatedOrder, orderItems, nil
}

func (s *orderService) GetOrderInvoicePDF(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) ([]byte, string, error) {
	if s.invoiceService != nil {
		return s.invoiceService.GetOrderInvoicePDF(ctx, userUUID, orderUUID)
	}
	return NewOrderInvoiceService(s.orderRepo, s.orderItemRepo, s.userRepo, s.cache).GetOrderInvoicePDF(ctx, userUUID, orderUUID)
}

func (s *orderService) BulkUpdateStatus(ctx context.Context, input v1dto.BulkUpdateOrderStatusInput) (*v1dto.BulkOperationResultDTO, error) {
	if s.adminService != nil {
		return s.adminService.BulkUpdateStatus(ctx, input)
	}
	return NewOrderAdminService(s).BulkUpdateStatus(ctx, input)
}

func (s *orderService) BulkCancel(ctx context.Context, userUUID uuid.UUID, input v1dto.BulkCancelOrdersInput) (*v1dto.BulkOperationResultDTO, error) {
	if s.adminService != nil {
		return s.adminService.BulkCancel(ctx, userUUID, input)
	}
	return NewOrderAdminService(s).BulkCancel(ctx, userUUID, input)
}
