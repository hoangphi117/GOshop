package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"goshop/internal/db"
	"goshop/pkg/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/rabbitmq"
	"goshop/services/order/internal/db/sqlc"
	v1dto "goshop/services/order/internal/dto"
	"goshop/services/order/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Saga Step Constants
const (
	SagaStepValidate     = "VALIDATE"
	SagaStepReserveStock = "RESERVE_STOCK"
	SagaStepApplyCoupon  = "APPLY_COUPON"
	SagaStepPersistOrder = "PERSIST_ORDER"
	SagaStepClearCart    = "CLEAR_CART"
	SagaStepEmitOutbox   = "EMIT_OUTBOX"
	SagaStepComplete     = "COMPLETE"
)

// Saga State Constants
const (
	SagaStateStarted      = "STARTED"
	SagaStateCompensating = "COMPENSATING"
	SagaStateConfirmed    = "CONFIRMED"
	SagaStateFailed       = "FAILED"
)

// SagaDeductItem stores product mapping for stock deduction and compensation.
type SagaDeductItem struct {
	ProductUUID string `json:"product_uuid"`
	ProductID   int32  `json:"product_id"`
	Quantity    int32  `json:"quantity"`
}

// SagaSnapshot holds checkpoint data for forward progress and backwards compensation.
type SagaSnapshot struct {
	OrderUUID           uuid.UUID        `json:"order_uuid"`
	OrderNumber         string           `json:"order_number"`
	PersistedOrderUUID  *uuid.UUID       `json:"persisted_order_uuid,omitempty"`
	UserUUID            uuid.UUID        `json:"user_uuid"`
	UserID              int32            `json:"user_id"`
	StockReserved       bool             `json:"stock_reserved"`
	DeductItems         []SagaDeductItem `json:"deduct_items,omitempty"`
	CouponApplied       bool             `json:"coupon_applied"`
	AppliedCouponID     *int32           `json:"applied_coupon_id,omitempty"`
	AppliedCouponCode   *string          `json:"applied_coupon_code,omitempty"`
	OrderPersisted      bool             `json:"order_persisted"`
	OrderID             int32            `json:"order_id,omitempty"`
	PurchasedProductIDs []int32          `json:"purchased_product_ids,omitempty"`
	CartCleared         bool             `json:"cart_cleared"`
	OutboxEmitted       bool             `json:"outbox_emitted"`
	FailureStep         string           `json:"failure_step,omitempty"`
	FailureReason       string           `json:"failure_reason,omitempty"`
}

type OrderSagaOrchestrator interface {
	OrderCheckoutService
	ExecuteCheckoutSaga(ctx context.Context, userUUID uuid.UUID, idempotencyKey string, input v1dto.CreateOrderInput) (*v1dto.OrderDTO, error)
	CompensateSaga(ctx context.Context, orderUUID uuid.UUID, failureReason string) error
	GetSagaStatus(ctx context.Context, orderUUID uuid.UUID) (*sqlc.OrderSaga, error)
	WithShippingValidator(v OrderShippingValidatorPort) OrderSagaOrchestrator
}

type orderSagaOrchestrator struct {
	sagaRepo          repository.SagaRepository
	txManager         db.TxManager
	pricingService    OrderPricingService
	catalogPort       CatalogPort
	stockPort         OrderStockPort
	orderStore        OrderStorePort
	orderItemStore    OrderItemStorePort
	userReader        OrderCustomerReader
	cartPort          OrderCartPort
	couponPort        OrderCouponPort
	trackingStore     OrderTrackingPort
	cache             cache.RedisCacheService
	outboxStore       OrderOutboxPort
	shippingValidator OrderShippingValidatorPort
}

func (s *orderSagaOrchestrator) WithShippingValidator(v OrderShippingValidatorPort) OrderSagaOrchestrator {
	s.shippingValidator = v
	return s
}

func NewOrderSagaOrchestrator(
	sagaRepo repository.SagaRepository,
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
) OrderSagaOrchestrator {
	var outboxPort OrderOutboxPort
	if len(outboxStore) > 0 {
		outboxPort = outboxStore[0]
	}
	return &orderSagaOrchestrator{
		sagaRepo:       sagaRepo,
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

var _ OrderCheckoutService = (*orderSagaOrchestrator)(nil)

func (s *orderSagaOrchestrator) GetSagaStatus(ctx context.Context, orderUUID uuid.UUID) (*sqlc.OrderSaga, error) {
	saga, err := s.sagaRepo.GetByOrderUUID(ctx, orderUUID)
	if err != nil {
		return nil, err
	}
	return &saga, nil
}

// CreateOrder satisfies the OrderCheckoutService port.
func (s *orderSagaOrchestrator) CreateOrder(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateOrderInput) (*v1dto.OrderDTO, error) {
	return s.ExecuteCheckoutSaga(ctx, userUUID, input.IdempotencyKey, input)
}

func (s *orderSagaOrchestrator) ExecuteCheckoutSaga(
	ctx context.Context,
	userUUID uuid.UUID,
	idempotencyKey string,
	input v1dto.CreateOrderInput,
) (*v1dto.OrderDTO, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		idempotencyKey = uuid.New().String()
	}

	// 0. Idempotency Gate
	existing, err := s.sagaRepo.GetByIdempotencyKey(ctx, userUUID, idempotencyKey)
	if err == nil {
		switch existing.State {
		case SagaStateConfirmed:
			return s.fetchExistingOrderDTO(ctx, existing)
		case SagaStateStarted:
			return nil, utils.NewError(utils.ErrCodeConflict, "A checkout request with this idempotency key is already in progress")
		case SagaStateCompensating, SagaStateFailed:
			reason := "checkout failed"
			if existing.FailureReason != nil && *existing.FailureReason != "" {
				reason = *existing.FailureReason
			}
			return nil, utils.NewError(utils.ErrCodeConflict, fmt.Sprintf("Checkout previously failed with: %s", reason))
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to check saga idempotency", err)
	}

	// Resolve Customer
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

	orderUUID := uuid.New()
	orderNumber := GenerateOrderNumber(user.UserID)

	payloadBytes, err := json.Marshal(input)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeBadRequest, "invalid order payload", err)
	}

	stepValidate := SagaStepValidate
	snapshot := SagaSnapshot{
		OrderUUID:   orderUUID,
		OrderNumber: orderNumber,
		UserUUID:    userUUID,
		UserID:      user.UserID,
	}
	snapBytes, _ := json.Marshal(snapshot)

	// Persist Initial Saga Record (CAS Start)
	_, err = s.sagaRepo.Start(ctx, nil, sqlc.StartSagaParams{
		OrderUuid:      orderUUID,
		UserUuid:       userUUID,
		IdempotencyKey: idempotencyKey,
		LastStep:       &stepValidate,
		Payload:        payloadBytes,
		Snapshot:       snapBytes,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Race condition: another concurrent call inserted the same idempotency key
			existingRace, raceErr := s.sagaRepo.GetByIdempotencyKey(ctx, userUUID, idempotencyKey)
			if raceErr == nil && existingRace.State == SagaStateConfirmed {
				return s.fetchExistingOrderDTO(ctx, existingRace)
			}
			return nil, utils.NewError(utils.ErrCodeConflict, "Concurrent checkout with this idempotency key in progress")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to initialize order saga", err)
	}

	checkpoint := func(step string) error {
		sBytes, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		_, err = s.sagaRepo.UpdateStep(ctx, nil, orderUUID, &step, sBytes)
		if err != nil {
			logger.Log.Warn().Err(err).Str("step", step).Str("order_uuid", orderUUID.String()).Msg("Failed to update saga step checkpoint")
		}
		return err
	}

	/* ====================== STEP 1: VALIDATE ====================== */
	productMapInput := make(map[string]int32, len(input.Items))
	productUUIDs := make([]uuid.UUID, len(input.Items))
	for i, item := range input.Items {
		if _, exists := productMapInput[item.ProductUUID]; exists {
			s.compensateAndFail(ctx, orderUUID, "Duplicate product in order", SagaStepValidate)
			return nil, utils.NewError(utils.ErrCodeBadRequest, "Duplicate product in order")
		}
		productMapInput[item.ProductUUID] = item.Quantity

		parsed, err := uuid.Parse(item.ProductUUID)
		if err != nil {
			s.compensateAndFail(ctx, orderUUID, "Invalid product UUID: "+item.ProductUUID, SagaStepValidate)
			return nil, utils.NewError(utils.ErrCodeBadRequest, "Invalid product UUID: "+item.ProductUUID)
		}
		productUUIDs[i] = parsed
	}

	productSnapshots, err := s.catalogPort.GetProductsForCheckout(ctx, productUUIDs)
	if err != nil {
		s.compensateAndFail(ctx, orderUUID, "Product catalog lookup failed", SagaStepValidate)
		return nil, err
	}
	if len(productSnapshots) != len(input.Items) {
		s.compensateAndFail(ctx, orderUUID, "One or more products not found", SagaStepValidate)
		return nil, utils.NewError(utils.ErrCodeNotFound, "One or more products not found")
	}

	for _, item := range input.Items {
		p, ok := productSnapshots[item.ProductUUID]
		if !ok {
			s.compensateAndFail(ctx, orderUUID, "Product not found: "+item.ProductUUID, SagaStepValidate)
			return nil, utils.NewError(utils.ErrCodeNotFound, "Product not found: "+item.ProductUUID)
		}
		if p.Status != ProductStatusActive {
			s.compensateAndFail(ctx, orderUUID, "Product unavailable: "+item.ProductUUID, SagaStepValidate)
			return nil, utils.NewError(utils.ErrCodeBadRequest, "Product is unavailable: "+item.ProductUUID)
		}
	}

	deductItems := make([]DeductItem, len(input.Items))
	sagaDeductItems := make([]SagaDeductItem, len(input.Items))
	priceInputs := make([]ItemPriceInput, len(input.Items))
	prodIDToSnapshot := make(map[int32]CheckoutProductSnapshot, len(input.Items))
	for i, item := range input.Items {
		p := productSnapshots[item.ProductUUID]
		deductItems[i] = DeductItem{
			ProductUUID: item.ProductUUID,
			Quantity:    item.Quantity,
		}
		sagaDeductItems[i] = SagaDeductItem{
			ProductUUID: item.ProductUUID,
			ProductID:   p.ProductID,
			Quantity:    item.Quantity,
		}
		priceInputs[i] = ItemPriceInput{
			ProductID:   p.ProductID,
			ProductName: p.ProductName,
			Price:       p.Price,
			Quantity:    item.Quantity,
		}
		prodIDToSnapshot[p.ProductID] = p
	}
	snapshot.DeductItems = sagaDeductItems

	// Pricing calculation
	ctxWithCust := WithOrderCustomer(ctx, user)

	// G7 FIX: Validate or calculate shipping fee server-side via carrier
	effectiveShippingFee := input.ShippingFee
	if s.shippingValidator != nil && input.DistrictID != nil && input.WardCode != nil && *input.DistrictID > 0 && strings.TrimSpace(*input.WardCode) != "" {
		provider := "ghn"
		if input.ShippingProvider != nil && *input.ShippingProvider != "" {
			provider = *input.ShippingProvider
		}
		validatedFee, vErr := s.shippingValidator.ValidateOrCalculateShippingFee(
			ctx, provider, *input.DistrictID, *input.WardCode, 500, 0, input.ShippingFee,
		)
		if vErr == nil {
			effectiveShippingFee = validatedFee
		} else {
			logger.Log.Warn().Err(vErr).Msg("Failed to validate shipping fee via carrier, using sanitized client fee")
		}
	} else if effectiveShippingFee < 0 {
		effectiveShippingFee = 0
	}

	pricingResult, err := s.pricingService.CalculateOrderTotals(ctxWithCust, priceInputs, input.CouponCode, user.UserID, effectiveShippingFee, time.Now())
	if err != nil {
		s.compensateAndFail(ctx, orderUUID, fmt.Sprintf("Pricing calculation failed: %v", err), SagaStepValidate)
		return nil, err
	}
	snapshot.AppliedCouponID = pricingResult.AppliedCouponID
	snapshot.AppliedCouponCode = pricingResult.AppliedCouponCode

	// G4 FIX: Record intent checkpoint with items before initiating stock deduction side-effects
	if err := checkpoint(SagaStepReserveStock); err != nil {
		s.compensateAndFail(ctx, orderUUID, fmt.Sprintf("Failed to checkpoint before stock reservation: %v", err), SagaStepValidate)
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to checkpoint saga state", err)
	}

	/* ====================== STEP 2: RESERVE_STOCK ====================== */
	if err := s.stockPort.LockAndDeductStockWithRef(ctx, deductItems, orderUUID.String(), "order", &user.UserID, nil); err != nil {
		s.compensateAndFail(ctx, orderUUID, fmt.Sprintf("Stock deduction failed: %v", err), SagaStepReserveStock)
		return nil, err
	}
	snapshot.StockReserved = true
	_ = checkpoint(SagaStepReserveStock)

	/* ====================== STEP 3: APPLY_COUPON ====================== */
	if pricingResult.AppliedCouponID != nil && s.couponPort != nil {
		if err := s.couponPort.LockAndApplyCoupon(ctx, *pricingResult.AppliedCouponID, user.UserID, 0, &orderUUID); err != nil {
			s.compensateAndFail(ctx, orderUUID, fmt.Sprintf("Coupon application failed: %v", err), SagaStepApplyCoupon)
			return nil, err
		}
		snapshot.CouponApplied = true
		_ = checkpoint(SagaStepApplyCoupon)
	}

	/* ====================== STEP 4: PERSIST_ORDER & ATOMIC CONFIRM ====================== */
	var order sqlc.Order
	var purchasedProductIDs []int32
	var createdOrderItems []sqlc.GetOrderItemsByOrderIDRow

	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		tx, _ := db.GetTx(txCtx)

		// G1 FIX: Explicitly supply orderUUID to eliminate PersistedOrderUUID divergence
		createOrderParams := sqlc.CreateOrderParams{
			OrderUuid:            orderUUID,
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

		purchasedProductIDs = make([]int32, len(pricingResult.Items))
		purchasedProductUUIDStrings := make([]string, 0, len(pricingResult.Items))
		createdOrderItems = make([]sqlc.GetOrderItemsByOrderIDRow, len(pricingResult.Items))

		for i, item := range pricingResult.Items {
			snap := prodIDToSnapshot[item.ProductID]
			purchasedProductIDs[i] = item.ProductID
			if snap.ProductUUID != uuid.Nil {
				purchasedProductUUIDStrings = append(purchasedProductUUIDStrings, snap.ProductUUID.String())
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
				OrderItemProductImageUrl: snap.ProductImageURL,
				OrderItemOrderUuid:       orderUUID,
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
				OrderItemProductImageUrl: snap.ProductImageURL,
				OrderItemOrderUuid:       orderUUID,
				ProductUuid:              snap.ProductUUID,
				ProductSlug:              snap.ProductSlug,
				ProductImageUrl:          snap.ProductImageURL,
			}
		}

		// Initial order tracking event
		createNote := "Order created and confirmed (Saga orchestrated)"
		_, err = s.trackingStore.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
			EventOrderID: order.OrderID,
			EventStatus:  "pending",
			EventNote:    &createNote,
			EventSource:  "user",
		})
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "failed to insert tracking event", err)
		}

		// Enqueue OrderCreated outbox event atomically (G6 FIX: includes PurchasedItemUUIDs)
		if s.outboxStore != nil {
			outboxPayload := rabbitmq.NewOrderCreatedEvent(
				uuid.New().String(),
				orderUUID.String(),
				order.OrderNumber,
				user.UserUuid.String(),
				order.OrderID,
				order.OrderUserID,
				utils.NumericToFloat(order.OrderTotalAmount),
				purchasedProductIDs,
			).WithItemUUIDs(purchasedProductUUIDStrings)
			items := make([]rabbitmq.OrderCreatedItem, 0, len(createdOrderItems))
			for _, item := range createdOrderItems {
				items = append(items, rabbitmq.OrderCreatedItem{
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
			_, err = s.outboxStore.Insert(txCtx, tx, "order", orderUUID.String(), rabbitmq.QueueOrderCreated, outboxPayload)
			if err != nil {
				return utils.WrapError(utils.ErrCodeInternal, "failed to record order outbox event", err)
			}
		}

		// G5 FIX: Atomic CAS Confirm INSIDE the same database transaction
		won, err := s.sagaRepo.Confirm(txCtx, tx, orderUUID)
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "failed to atomic confirm saga in tx", err)
		}
		if !won {
			return utils.NewError(utils.ErrCodeConflict, "saga already in terminal state or concurrent update conflict")
		}

		return nil
	})

	if err != nil {
		s.compensateAndFail(ctx, orderUUID, fmt.Sprintf("Order persistence failed: %v", err), SagaStepPersistOrder)
		return nil, err
	}

	snapshot.OrderPersisted = true
	snapshot.OrderID = order.OrderID
	snapshot.PersistedOrderUUID = &orderUUID
	snapshot.PurchasedProductIDs = purchasedProductIDs
	snapshot.OutboxEmitted = true
	_ = checkpoint(SagaStepComplete)

	// Việc dọn giỏ được chuyển giao hoàn toàn cho event OrderCreated -> OrderConsumer(cartService).

	InvalidateOrderCachesWithCache(s.cache, user.UserUuid)
	dto := v1dto.MapToOrderDTO(order, createdOrderItems)
	return &dto, nil
}

func (s *orderSagaOrchestrator) compensateAndFail(ctx context.Context, orderUUID uuid.UUID, reason string, failedStep string) {
	logger.Log.Warn().Str("order_uuid", orderUUID.String()).Str("failed_step", failedStep).Str("reason", reason).Msg("Saga checkout failed: initiating compensation")
	if compErr := s.CompensateSaga(ctx, orderUUID, reason); compErr != nil {
		logger.Log.Error().Err(compErr).Str("order_uuid", orderUUID.String()).Msg("Saga compensation encountered failure, scheduled for retry")
	}
}

func (s *orderSagaOrchestrator) CompensateSaga(ctx context.Context, orderUUID uuid.UUID, failureReason string) error {
	saga, err := s.sagaRepo.GetByOrderUUID(ctx, orderUUID)
	if err != nil {
		return fmt.Errorf("failed to load saga state for compensation: %w", err)
	}

	//  If already CONFIRMED, do not compensate!
	if saga.State == SagaStateConfirmed {
		logger.Log.Info().Str("order_uuid", orderUUID.String()).Msg("Saga is already CONFIRMED, skipping compensation")
		return nil
	}

	if saga.State == SagaStateFailed {
		return nil
	}

	var snapshot SagaSnapshot
	if len(saga.Snapshot) > 0 {
		_ = json.Unmarshal(saga.Snapshot, &snapshot)
	}

	// If order was already persisted in DB, roll forward to CONFIRMED rather than cancelling
	if snapshot.OrderPersisted {
		logger.Log.Info().Str("order_uuid", orderUUID.String()).Msg("Order was already persisted, rolling forward saga to CONFIRMED")
		_, err := s.sagaRepo.Confirm(ctx, nil, orderUUID)
		return err
	}

	// CAS claim transition: STARTED -> COMPENSATING
	_, err = s.sagaRepo.ClaimForCompensation(ctx, nil, orderUUID, &failureReason)
	if err != nil {
		logger.Log.Warn().Err(err).Str("order_uuid", orderUUID.String()).Msg("ClaimForCompensation CAS check")
	}

	var compErrors []string

	// 1. Compensate APPLY_COUPON -> Release coupon usage
	if (snapshot.CouponApplied || snapshot.AppliedCouponID != nil) && s.couponPort != nil {
		if snapshot.AppliedCouponID != nil {
			if err := s.couponPort.ReleaseCoupon(ctx, *snapshot.AppliedCouponID, snapshot.OrderID, &orderUUID); err != nil {
				compErrors = append(compErrors, fmt.Sprintf("release_coupon: %v", err))
			}
		}
	}

	// 2. Compensate RESERVE_STOCK -> Restore stock (Unconditional if deduct items recorded)
	if len(snapshot.DeductItems) > 0 && s.stockPort != nil {
		restoreItems := make([]RestoreItem, len(snapshot.DeductItems))
		for i, it := range snapshot.DeductItems {
			restoreItems[i] = RestoreItem{
				ProductID: it.ProductID,
				Quantity:  it.Quantity,
			}
		}
		note := "Saga compensation for order " + orderUUID.String()
		if err := s.stockPort.RestoreStockWithRef(ctx, restoreItems, orderUUID.String(), "order", &snapshot.UserID, &note); err != nil {
			compErrors = append(compErrors, fmt.Sprintf("restore_stock: %v", err))
		}
	}

	// 4. Update saga terminal state
	if len(compErrors) == 0 {
		_, err := s.sagaRepo.MarkFailed(ctx, nil, orderUUID, &failureReason)
		return err
	}

	// Partial failure during compensation: schedule exponential retry
	errSummary := fmt.Sprintf("%s | compensation_failures: %s", failureReason, strings.Join(compErrors, "; "))
	_, retryErr := s.sagaRepo.ScheduleRetry(ctx, nil, orderUUID, &errSummary)
	if retryErr != nil {
		return fmt.Errorf("compensation failed (%s) and schedule retry failed: %w", errSummary, retryErr)
	}

	return fmt.Errorf("compensation incomplete, scheduled for retry: %s", errSummary)
}

func (s *orderSagaOrchestrator) fetchExistingOrderDTO(ctx context.Context, saga sqlc.OrderSaga) (*v1dto.OrderDTO, error) {
	targetUUID := saga.OrderUuid
	var targetID int32

	if len(saga.Snapshot) > 0 {
		var snapshot SagaSnapshot
		if err := json.Unmarshal(saga.Snapshot, &snapshot); err == nil {
			if snapshot.PersistedOrderUUID != nil && *snapshot.PersistedOrderUUID != uuid.Nil {
				targetUUID = *snapshot.PersistedOrderUUID
			}
			if snapshot.OrderID > 0 {
				targetID = snapshot.OrderID
			}
		}
	}

	orderRow, err := s.orderStore.GetByUUID(ctx, targetUUID)
	if err != nil && targetID > 0 {
		if rowByID, idErr := s.orderStore.GetByID(ctx, targetID); idErr == nil {
			orderRow = sqlc.GetOrderByUUIDRow{
				OrderID:              rowByID.OrderID,
				OrderUuid:            rowByID.OrderUuid,
				OrderNumber:          rowByID.OrderNumber,
				OrderUserID:          rowByID.OrderUserID,
				OrderUserUuid:        rowByID.OrderUserUuid,
				OrderCustomerName:    rowByID.OrderCustomerName,
				OrderCustomerEmail:   rowByID.OrderCustomerEmail,
				OrderShippingName:    rowByID.OrderShippingName,
				OrderShippingPhone:   rowByID.OrderShippingPhone,
				OrderShippingAddress: rowByID.OrderShippingAddress,
				OrderTotalAmount:     rowByID.OrderTotalAmount,
				OrderRefundedAmount:  rowByID.OrderRefundedAmount,
				OrderPaymentMethod:   rowByID.OrderPaymentMethod,
				OrderPaymentStatus:   rowByID.OrderPaymentStatus,
				OrderNote:            rowByID.OrderNote,
				OrderStatus:          rowByID.OrderStatus,
				OrderCancelledAt:     rowByID.OrderCancelledAt,
				OrderCreatedAt:       rowByID.OrderCreatedAt,
				OrderUpdatedAt:       rowByID.OrderUpdatedAt,
				OrderCouponID:        rowByID.OrderCouponID,
				OrderCouponCode:      rowByID.OrderCouponCode,
				OrderSubtotalAmount:  rowByID.OrderSubtotalAmount,
				OrderDiscountAmount:  rowByID.OrderDiscountAmount,
				OrderShippingFee:     rowByID.OrderShippingFee,
				UserUuid:             rowByID.UserUuid,
				UserFullname:         rowByID.UserFullname,
			}
			err = nil
		}
	}
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeNotFound, "order not found for idempotency key", err)
	}

	items, err := s.orderItemStore.GetByOrderID(ctx, orderRow.OrderID)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to retrieve order items", err)
	}

	orderObj := sqlc.Order{
		OrderID:              orderRow.OrderID,
		OrderUuid:            orderRow.OrderUuid,
		OrderNumber:          orderRow.OrderNumber,
		OrderUserID:          orderRow.OrderUserID,
		OrderUserUuid:        orderRow.OrderUserUuid,
		OrderCustomerName:    orderRow.OrderCustomerName,
		OrderCustomerEmail:   orderRow.OrderCustomerEmail,
		OrderShippingName:    orderRow.OrderShippingName,
		OrderShippingPhone:   orderRow.OrderShippingPhone,
		OrderShippingAddress: orderRow.OrderShippingAddress,
		OrderTotalAmount:     orderRow.OrderTotalAmount,
		OrderRefundedAmount:  orderRow.OrderRefundedAmount,
		OrderPaymentMethod:   orderRow.OrderPaymentMethod,
		OrderPaymentStatus:   orderRow.OrderPaymentStatus,
		OrderNote:            orderRow.OrderNote,
		OrderStatus:          orderRow.OrderStatus,
		OrderCancelledAt:     orderRow.OrderCancelledAt,
		OrderCreatedAt:       orderRow.OrderCreatedAt,
		OrderUpdatedAt:       orderRow.OrderUpdatedAt,
		OrderCouponID:        orderRow.OrderCouponID,
		OrderCouponCode:      orderRow.OrderCouponCode,
		OrderSubtotalAmount:  orderRow.OrderSubtotalAmount,
		OrderDiscountAmount:  orderRow.OrderDiscountAmount,
		OrderShippingFee:     orderRow.OrderShippingFee,
	}

	dto := v1dto.MapToOrderDTO(orderObj, items)
	return &dto, nil
}
