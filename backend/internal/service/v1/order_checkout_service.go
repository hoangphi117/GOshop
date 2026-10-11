package v1service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/rabbitmq"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const base32Alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func GenerateOrderNumber(userID int32) string {
	now := time.Now().UTC()
	datePrefix := now.Format("060102") // 6 characters: YYMMDD

	userFragment := fmt.Sprintf("%02X", uint32(userID%256))

	const entropyLen = 6
	randomSuffix := make([]byte, entropyLen)
	alphabetLen := big.NewInt(int64(len(base32Alphabet)))

	for i := 0; i < entropyLen; i++ {
		num, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			nano := now.UnixNano() + int64(i*1000)
			randomSuffix[i] = base32Alphabet[nano%int64(len(base32Alphabet))]
			continue
		}
		randomSuffix[i] = base32Alphabet[num.Int64()]
	}

	return fmt.Sprintf("%s-%s-%s", datePrefix, userFragment, string(randomSuffix))
}

type checkoutService struct {
	txManager           db.TxManager
	pricingService      OrderPricingService
	stockPort           OrderStockPort
	orderStore          OrderStorePort
	orderItemStore      OrderItemStorePort
	userReader          OrderCustomerReader
	cartPort            OrderCartPort
	couponPort          OrderCouponPort
	trackingStore       OrderTrackingPort
	cache               cache.RedisCacheService
	outboxStore         OrderOutboxPort
}

func NewOrderCheckoutService(
	txManager db.TxManager,
	pricingService OrderPricingService,
	stockPort OrderStockPort,
	orderStore OrderStorePort,
	orderItemStore OrderItemStorePort,
	userReader OrderCustomerReader,
	cartPort OrderCartPort,
	couponPort OrderCouponPort,
	trackingStore OrderTrackingPort,
	cacheService cache.RedisCacheService,
	outboxStore ...OrderOutboxPort,
) OrderCheckoutService {
	var outboxPort OrderOutboxPort
	if len(outboxStore) > 0 {
		outboxPort = outboxStore[0]
	}
	return &checkoutService{
		txManager:      txManager,
		pricingService: pricingService,
		stockPort:      stockPort,
		orderStore:     orderStore,
		orderItemStore: orderItemStore,
		userReader:     userReader,
		cartPort:       cartPort,
		couponPort:     couponPort,
		trackingStore:  trackingStore,
		cache:          cacheService,
		outboxStore:    outboxPort,
	}
}

func (s *checkoutService) CreateOrder(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateOrderInput) (*v1dto.OrderDTO, error) {
	var user *OrderCustomer
	if cust, ok := GetOrderCustomerFromContext(ctx); ok && cust.UserUUID == userUUID && cust.UserID > 0 && cust.UserFullname != "" && cust.UserEmail != "" {
		user = cust
	} else {
		var err error
		user, err = s.userReader.GetByUUID(ctx, userUUID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, utils.NewError(utils.ErrCodeNotFound, "User not found")
			}
			return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to get user", err)
		}
	}

	// Validate duplicate products in the order
	productMapInput := make(map[string]int32)
	for _, item := range input.Items {
		if _, exists := productMapInput[item.ProductUUID]; exists {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "Duplicate product in order")
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

	var couponLockKey string
	if input.CouponCode != nil && strings.TrimSpace(*input.CouponCode) != "" && s.cache != nil {
		cleanCouponCode := strings.ToUpper(strings.TrimSpace(*input.CouponCode))
		couponLockKey = fmt.Sprintf("lock:coupon:%s:%d", cleanCouponCode, user.UserID)
		acquired, err := s.cache.SetNX(couponLockKey, "1", 10*time.Second)
		if err != nil {
			logger.Log.Warn().Err(err).Str("coupon_lock_key", couponLockKey).Msg("Failed to check coupon lock")
		} else if !acquired {
			return nil, utils.NewError(utils.ErrCodeConflict, "Coupon is currently being applied in another checkout request")
		}
		defer func() {
			if couponLockKey != "" {
				_ = s.cache.Del(couponLockKey)
			}
		}()
	}

	/* ====================== TRANSACTION START ====================== */
	var order sqlc.Order
	var purchasedProductIDs []int32
	var createdOrderItems []sqlc.GetOrderItemsByOrderIDRow

	err := s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		tx, _ := db.GetTx(txCtx)

		orderNumber := GenerateOrderNumber(user.UserID)

		// 1. Lock products and deduct stock via StockPort
		productMap, err := s.stockPort.LockAndDeductStockWithRef(txCtx, deductItems, orderNumber, "order", &user.UserID, nil)
		if err != nil {
			return err
		}

		priceInputs := make([]ItemPriceInput, len(input.Items))
		prodIDToUUID := make(map[int32]uuid.UUID, len(input.Items))
		for i, item := range input.Items {
			p := productMap[item.ProductUUID]
			priceInputs[i] = ItemPriceInput{
				ProductID:   p.ProductID,
				ProductName: p.ProductName,
				Price:       p.Price,
				Quantity:    item.Quantity,
			}
			prodIDToUUID[p.ProductID] = p.ProductUUID
		}

		pricingResult, err := s.pricingService.CalculateOrderTotals(txCtx, priceInputs, input.CouponCode, user.UserID, input.ShippingFee, time.Now())
		if err != nil {
			return err
		}

		createOrderParams := sqlc.CreateOrderParams{
			OrderNumber:          orderNumber,
			OrderUserID:          user.UserID,
			OrderUserUuid:        user.UserUuid,
			OrderCustomerName:    user.UserFullname,
			OrderCustomerEmail:   user.UserEmail,
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

		createdOrder, err := s.orderStore.Create(txCtx, createOrderParams)
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "failed to create order", err)
		}
		order = createdOrder

		// Lock and apply coupon via CouponPort
		if pricingResult.AppliedCouponID != nil && s.couponPort != nil {
			if err := s.couponPort.LockAndApplyCoupon(txCtx, *pricingResult.AppliedCouponID, user.UserID, order.OrderID); err != nil {
				return err
			}
		}

		purchasedProductIDs = make([]int32, len(pricingResult.Items))
		createdOrderItems = make([]sqlc.GetOrderItemsByOrderIDRow, len(pricingResult.Items))

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

			createdItem, err := s.orderItemStore.Create(txCtx, tx, createdItemParams)
			if err != nil {
				return utils.WrapError(utils.ErrCodeInternal, "failed to create order item", err)
			}

			orderItemUuid := createdItem.OrderItemUuid
			if orderItemUuid == uuid.Nil {
				orderItemUuid = uuid.New()
			}
			prodName := createdItem.OrderItemProductName
			if prodName == "" {
				prodName = item.ProductName
			}
			price := createdItem.OrderItemPrice
			if !price.Valid {
				price = createdItemParams.OrderItemPrice
			}
			qty := createdItem.OrderItemQuantity
			if qty == 0 {
				qty = item.Quantity
			}
			subtotal := createdItem.OrderItemSubtotal
			if !subtotal.Valid {
				subtotal = createdItemParams.OrderItemSubtotal
			}
			createdAt := createdItem.OrderItemCreatedAt
			if createdAt.IsZero() {
				createdAt = time.Now()
			}

			prodUUID := prodIDToUUID[item.ProductID]
			createdOrderItems[i] = sqlc.GetOrderItemsByOrderIDRow{
				OrderItemID:          createdItem.OrderItemID,
				OrderItemUuid:        orderItemUuid,
				OrderItemOrderID:     order.OrderID,
				OrderItemProductID:   item.ProductID,
				OrderItemProductName: prodName,
				OrderItemPrice:       price,
				OrderItemQuantity:    qty,
				OrderItemSubtotal:    subtotal,
				OrderItemCreatedAt:   createdAt,
				ProductUuid:          prodUUID,
			}
		}

		// Record the initial order tracking event
		createNote := "Order created and pending confirmation"
		_, err = s.trackingStore.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
			EventOrderID: order.OrderID,
			EventStatus:  "pending",
			EventNote:    &createNote,
			EventSource:  "user",
		})
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "failed to insert tracking event", err)
		}

		// Enqueue OrderCreated event to transactional outbox atomically
		if s.outboxStore != nil {
			outboxPayload := rabbitmq.NewOrderCreatedEvent(
				uuid.New().String(),
				order.OrderUuid.String(),
				order.OrderNumber,
				user.UserUuid.String(),
				order.OrderID,
				order.OrderUserID,
				utils.NumericToFloat(order.OrderTotalAmount),
				purchasedProductIDs,
			)
			_, err = s.outboxStore.Insert(txCtx, tx, "order", order.OrderUuid.String(), rabbitmq.QueueOrderCreated, outboxPayload)
			if err != nil {
				return utils.WrapError(utils.ErrCodeInternal, "failed to record order outbox event", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	/* ====================== TRANSACTION END ====================== */

	// Invalidate related order caches
	InvalidateOrderCachesWithCache(s.cache, user.UserUuid)

	dto := v1dto.MapToOrderDTO(order, createdOrderItems)
	return &dto, nil
}
