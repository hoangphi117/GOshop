package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"goshop/contracts/events"
	"goshop/services/order/internal/db/sqlc"
	v1dto "goshop/services/order/internal/dto"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/db"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/utils"
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
	txManager      db.TxManager
	pricingService OrderPricingService
	catalogPort    CatalogPort
	stockPort      OrderStockPort
	orderStore     OrderStorePort
	orderItemStore OrderItemStorePort
	userReader     OrderCustomerReader
	cartPort       OrderCartPort
	couponPort     OrderCouponPort
	trackingStore  OrderTrackingPort
	cache          cache.RedisCacheService
	outboxStore    OrderOutboxPort
}

func NewOrderCheckoutService(
	txManager db.TxManager,
	pricingService OrderPricingService,
	catalogPort CatalogPort,
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
		catalogPort:    catalogPort,
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

	// Validate duplicate line items in the order and parse product UUIDs
	productMapInput := make(map[string]int32)
	uniqueProductUUIDMap := make(map[uuid.UUID]struct{})
	for _, item := range input.Items {
		itemKey := item.ProductUUID
		if item.VariantUUID != nil && *item.VariantUUID != "" {
			itemKey = item.ProductUUID + ":" + *item.VariantUUID
		}
		if _, exists := productMapInput[itemKey]; exists {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "Duplicate product in order")
		}
		productMapInput[itemKey] = item.Quantity

		parsed, err := uuid.Parse(item.ProductUUID)
		if err != nil {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "Invalid product UUID: "+item.ProductUUID)
		}
		uniqueProductUUIDMap[parsed] = struct{}{}
	}

	productUUIDs := make([]uuid.UUID, 0, len(uniqueProductUUIDMap))
	for pu := range uniqueProductUUIDMap {
		productUUIDs = append(productUUIDs, pu)
	}

	productSnapshots, err := s.catalogPort.GetProductsForCheckout(ctx, productUUIDs)
	if err != nil {
		return nil, err
	}
	if len(productSnapshots) < len(productUUIDs) {
		return nil, utils.NewError(utils.ErrCodeNotFound, "One or more products not found")
	}

	for _, item := range input.Items {
		p, ok := productSnapshots[item.ProductUUID]
		if !ok {
			return nil, utils.NewError(utils.ErrCodeNotFound, "Product not found: "+item.ProductUUID)
		}
		if p.Status != ProductStatusActive {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "Product is unavailable: "+item.ProductUUID)
		}
	}

	// Resolve variant details (price, sku, title, image) if variant_uuid is supplied
	type resolvedVariantInfo struct {
		variantUUID  uuid.UUID
		variantTitle string
		price        float64
		sku          string
		imageURL     *string
	}
	resolvedVariants := make(map[int]resolvedVariantInfo) // index in input.Items -> variant info

	for i, item := range input.Items {
		if item.VariantUUID != nil && strings.TrimSpace(*item.VariantUUID) != "" {
			vUUID, err := uuid.Parse(strings.TrimSpace(*item.VariantUUID))
			if err != nil {
				return nil, utils.NewError(utils.ErrCodeBadRequest, "Invalid variant UUID: "+*item.VariantUUID)
			}
			pUUID, _ := uuid.Parse(item.ProductUUID)
			variants, vErr := s.catalogPort.GetVariantsForCheckout(ctx, pUUID)
			if vErr != nil {
				logger.Log.Warn().Err(vErr).Str("product_uuid", item.ProductUUID).Msg("Failed to fetch product variants from catalog")
			}
			var matchedVariant *CheckoutVariantSnapshot
			for vi := range variants {
				if variants[vi].VariantUUID == vUUID {
					matchedVariant = &variants[vi]
					break
				}
			}
			if matchedVariant == nil {
				return nil, utils.NewError(utils.ErrCodeNotFound, "Product variant not found or unavailable")
			}
			resolvedVariants[i] = resolvedVariantInfo{
				variantUUID:  matchedVariant.VariantUUID,
				variantTitle: matchedVariant.Title,
				price:        matchedVariant.Price,
				sku:          matchedVariant.SKU,
				imageURL:     matchedVariant.ImageURL,
			}
		}
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

	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		tx, _ := db.GetTx(txCtx)

		orderNumber := GenerateOrderNumber(user.UserID)

		// 1. Lock products and deduct stock via StockPort (stock-only, no pricing info)
		if err := s.stockPort.LockAndDeductStockWithRef(txCtx, deductItems, orderNumber, "order", &user.UserID, nil); err != nil {
			return err
		}

		priceInputs := make([]ItemPriceInput, len(input.Items))
		prodIDToSnapshot := make(map[int32]CheckoutProductSnapshot, len(input.Items))
		for i, item := range input.Items {
			p := productSnapshots[item.ProductUUID]
			price := p.Price
			name := p.ProductName
			if rv, ok := resolvedVariants[i]; ok {
				price = rv.price
				name = p.ProductName + " (" + rv.variantTitle + ")"
			}
			priceInputs[i] = ItemPriceInput{
				ProductID:   p.ProductID,
				ProductName: name,
				Price:       price,
				Quantity:    item.Quantity,
			}
			prodIDToSnapshot[p.ProductID] = p
		}

		txCtxWithCust := WithOrderCustomer(txCtx, user)
		pricingResult, err := s.pricingService.CalculateOrderTotals(txCtxWithCust, priceInputs, input.CouponCode, user.UserID, input.ShippingFee, time.Now())
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
			if err := s.couponPort.LockAndApplyCoupon(txCtx, *pricingResult.AppliedCouponID, user.UserID, order.OrderID, &order.OrderUuid); err != nil {
				return err
			}
		}

		purchasedProductIDs = make([]int32, len(pricingResult.Items))
		// Product UUIDs travel alongside the IDs so the cart consumer can drop
		// the purchased lines by identity rather than by surrogate key. The saga
		// path has always sent both; checkout sent only the IDs, so a product
		// re-created under the same numeric id removed the wrong cart line.
		purchasedProductUUIDStrings := make([]string, 0, len(pricingResult.Items))
		createdOrderItems = make([]sqlc.GetOrderItemsByOrderIDRow, len(pricingResult.Items))

		for i, item := range pricingResult.Items {
			origItem := input.Items[i]
			snap := prodIDToSnapshot[item.ProductID]
			purchasedProductIDs[i] = item.ProductID
			if snap.ProductUUID != uuid.Nil {
				purchasedProductUUIDStrings = append(purchasedProductUUIDStrings, snap.ProductUUID.String())
			}

			var variantUUID *uuid.UUID
			var variantTitle *string
			itemImg := snap.ProductImageURL
			if rv, ok := resolvedVariants[i]; ok {
				variantUUID = &rv.variantUUID
				variantTitle = &rv.variantTitle
				if rv.imageURL != nil && *rv.imageURL != "" {
					itemImg = rv.imageURL
				}
			}

			createdItemParams := sqlc.CreateOrderItemParams{
				OrderItemOrderID:         order.OrderID,
				OrderItemProductID:       item.ProductID,
				OrderItemProductName:     item.ProductName,
				OrderItemPrice:           utils.FloatToNumeric(item.Price),
				OrderItemQuantity:        item.Quantity,
				OrderItemSubtotal:        utils.FloatToNumeric(item.Subtotal),
				OrderItemProductUuid:     snap.ProductUUID,
				OrderItemProductSlug:     snap.ProductSlug,
				OrderItemProductImageUrl: itemImg,
				OrderItemOrderUuid:       order.OrderUuid,
				OrderItemVariantUuid:     variantUUID,
				OrderItemVariantTitle:    variantTitle,
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

			createdOrderItems[i] = sqlc.GetOrderItemsByOrderIDRow{
				OrderItemID:              createdItem.OrderItemID,
				OrderItemUuid:            orderItemUuid,
				OrderItemOrderID:         order.OrderID,
				OrderItemProductID:       item.ProductID,
				OrderItemProductName:     prodName,
				OrderItemPrice:           price,
				OrderItemQuantity:        qty,
				OrderItemSubtotal:        subtotal,
				OrderItemCreatedAt:       createdAt,
				OrderItemProductUuid:     snap.ProductUUID,
				OrderItemProductSlug:     snap.ProductSlug,
				OrderItemProductImageUrl: itemImg,
				OrderItemOrderUuid:       order.OrderUuid,
				OrderItemVariantUuid:     variantUUID,
				OrderItemVariantTitle:    variantTitle,
				ProductUuid:              snap.ProductUUID,
				ProductSlug:              snap.ProductSlug,
				ProductImageUrl:          itemImg,
			}
			_ = origItem
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
			outboxPayload := events.NewOrderCreatedEvent(
				uuid.New().String(),
				order.OrderUuid.String(),
				order.OrderNumber,
				user.UserUuid.String(),
				order.OrderID,
				order.OrderUserID,
				utils.NumericToFloat(order.OrderTotalAmount),
				purchasedProductIDs,
			).WithItemUUIDs(purchasedProductUUIDStrings)
			items := make([]events.OrderCreatedItem, 0, len(createdOrderItems))
			for _, item := range createdOrderItems {
				items = append(items, events.OrderCreatedItem{
					ItemUUID:    item.OrderItemUuid.String(),
					ProductID:   item.OrderItemProductID,
					ProductUUID: item.OrderItemProductUuid.String(),
					ProductName: item.OrderItemProductName,
					Price:       utils.NumericToFloat(item.OrderItemPrice),
					Quantity:    item.OrderItemQuantity,
					Subtotal:    utils.NumericToFloat(item.OrderItemSubtotal),
				})
			}
			outboxPayload = outboxPayload.WithItems(items).WithOrderAmounts(
				utils.NumericToFloat(order.OrderDiscountAmount),
				utils.NumericToFloat(order.OrderShippingFee),
				order.OrderCouponID,
				order.OrderPaymentMethod,
			)
			_, err = s.outboxStore.Insert(txCtx, tx, "order", order.OrderUuid.String(), events.QueueOrderCreated, outboxPayload)
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
