package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"goshop/services/order/internal/db/sqlc"
	v1dto "goshop/services/order/internal/dto"
	orderservice "goshop/services/order/internal/service"
	"goshop/internal/testutils"
	"goshop/internal/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSagaTestDependencies() (
	*MockSagaRepository,
	*MockOrderStockPort,
	*MockOrderCouponPort,
	*MockOrderRepository,
	*MockOrderItemRepository,
	*MockTrackingRepository,
	*MockOutboxRepository,
	*MockCartService,
	*orderservice.OrderCustomer,
	string,
	string,
) {
	mockSagaRepo := &MockSagaRepository{}
	mockStockPort := &MockOrderStockPort{}
	mockCouponPort := &MockOrderCouponPort{}
	mockOrderStore := &MockOrderRepository{}
	mockOrderItemStore := &MockOrderItemRepository{}
	mockTrackingStore := &MockTrackingRepository{}
	mockOutboxStore := &MockOutboxRepository{}
	mockCartPort := &MockCartService{}

	userUUID := uuid.New()
	customer := &orderservice.OrderCustomer{
		UserID:       101,
		UserUUID:     userUUID,
		UserFullname: "Test User",
		UserEmail:    "test@example.com",
		Status:       1,
	}

	prod1UUID := uuid.New().String()
	prod2UUID := uuid.New().String()

	return mockSagaRepo, mockStockPort, mockCouponPort, mockOrderStore, mockOrderItemStore, mockTrackingStore, mockOutboxStore, mockCartPort, customer, prod1UUID, prod2UUID
}

func TestOrderSagaOrchestrator_Execute_Success(t *testing.T) {
	ctx := context.Background()
	mockSagaRepo, mockStockPort, mockCouponPort, mockOrderStore, mockOrderItemStore, mockTrackingStore, mockOutboxStore, mockCartPort, customer, prod1UUID, _ := setupSagaTestDependencies()

	ctx = orderservice.WithOrderCustomer(ctx, customer)

	// Saga mock tracking
	var startedSaga sqlc.OrderSaga
	var confirmedOrderUUID uuid.UUID
	mockSagaRepo.GetByIdempotencyKeyFunc = func(ctx context.Context, userUUID uuid.UUID, key string) (sqlc.OrderSaga, error) {
		return sqlc.OrderSaga{}, pgx.ErrNoRows
	}
	mockSagaRepo.StartFunc = func(ctx context.Context, tx pgx.Tx, params sqlc.StartSagaParams) (sqlc.OrderSaga, error) {
		startedSaga = sqlc.OrderSaga{
			OrderUuid:      params.OrderUuid,
			UserUuid:       params.UserUuid,
			IdempotencyKey: params.IdempotencyKey,
			State:          orderservice.SagaStateStarted,
		}
		return startedSaga, nil
	}
	mockSagaRepo.ConfirmFunc = func(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (bool, error) {
		confirmedOrderUUID = orderUUID
		return true, nil
	}

	// Stock mock
	var deductedItems []orderservice.DeductItem
	mockStockPort.LockAndDeductStockWithRefFunc = func(ctx context.Context, items []orderservice.DeductItem, refID, refType string, actorID *int32, note *string) error {
		deductedItems = items
		return nil
	}

	// Catalog mock
	mockCatalogPort := &MockCatalogPort{
		GetProductsForCheckoutFunc: func(ctx context.Context, uuids []uuid.UUID) (map[string]orderservice.CheckoutProductSnapshot, error) {
			return map[string]orderservice.CheckoutProductSnapshot{
				prod1UUID: {
					ProductID:   1,
					ProductUUID: uuid.MustParse(prod1UUID),
					ProductName: "Mechanical Keyboard",
					Price:       150.0,
					Status:      orderservice.ProductStatusActive,
				},
			}, nil
		},
	}

	// Pricing mock
	mockPricingService := &MockOrderPricingService{
		CalculateOrderTotalsFunc: func(ctx context.Context, items []orderservice.ItemPriceInput, couponCode *string, userID int32, shippingFee float64, now time.Time) (*orderservice.OrderPricingResult, error) {
			return &orderservice.OrderPricingResult{
				SubtotalAmount:   300.0,
				ShippingFee:      15.0,
				DiscountAmount:   0.0,
				FinalTotalAmount: 315.0,
				Items: []orderservice.ItemPriceResult{
					{ProductID: 1, ProductName: "Mechanical Keyboard", Price: 150.0, Quantity: 2, Subtotal: 300.0},
				},
			}, nil
		},
	}

	// Order store mock
	mockOrderStore.CreateFunc = func(ctx context.Context, params sqlc.CreateOrderParams) (sqlc.Order, error) {
		return sqlc.Order{
			OrderID:              1001,
			OrderUuid:            uuid.New(),
			OrderNumber:          params.OrderNumber,
			OrderUserID:          params.OrderUserID,
			OrderUserUuid:        params.OrderUserUuid,
			OrderTotalAmount:     params.OrderTotalAmount,
			OrderSubtotalAmount:  params.OrderSubtotalAmount,
			OrderShippingFee:     params.OrderShippingFee,
			OrderShippingName:    params.OrderShippingName,
			OrderShippingPhone:   params.OrderShippingPhone,
			OrderShippingAddress: params.OrderShippingAddress,
			OrderStatus:          params.OrderStatus,
		}, nil
	}

	mockOrderItemStore.CreateFunc = func(ctx context.Context, tx pgx.Tx, params sqlc.CreateOrderItemParams) (sqlc.OrderItem, error) {
		return sqlc.OrderItem{
			OrderItemID:          1,
			OrderItemOrderID:     params.OrderItemOrderID,
			OrderItemProductID:   params.OrderItemProductID,
			OrderItemProductName: params.OrderItemProductName,
			OrderItemPrice:       params.OrderItemPrice,
			OrderItemQuantity:    params.OrderItemQuantity,
			OrderItemSubtotal:    params.OrderItemSubtotal,
		}, nil
	}

	mockTrackingStore.InsertEventFunc = func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
		return sqlc.OrderTrackingEvent{EventID: 1}, nil
	}

	customerAdapter := &MockCustomerReader{}

	orchestrator := orderservice.NewOrderSagaOrchestrator(
		mockSagaRepo,
		testutils.NewNoopTxManager(),
		mockPricingService,
		mockCatalogPort,
		mockStockPort,
		mockOrderStore,
		mockOrderItemStore,
		customerAdapter,
		mockCartPort,
		mockCouponPort,
		mockTrackingStore,
		nil,
		mockOutboxStore,
	)

	input := v1dto.CreateOrderInput{
		Items: []v1dto.CreateOrderItemInput{
			{ProductUUID: prod1UUID, Quantity: 2},
		},
		ShippingName:    "John Doe",
		ShippingPhone:   "0901234567",
		ShippingAddress: "123 Street",
		PaymentMethod:   1,
	}

	idemKey := "idem-test-1"
	orderDTO, err := orchestrator.ExecuteCheckoutSaga(ctx, customer.UserUUID, idemKey, input)
	require.NoError(t, err)
	require.NotNil(t, orderDTO)
	assert.NotEmpty(t, orderDTO.UUID)
	assert.Equal(t, startedSaga.OrderUuid, confirmedOrderUUID)
	assert.Len(t, deductedItems, 1)
	assert.Equal(t, int32(2), deductedItems[0].Quantity)
}

func TestOrderSagaOrchestrator_Idempotency_Confirmed(t *testing.T) {
	ctx := context.Background()
	mockSagaRepo, mockStockPort, mockCouponPort, mockOrderStore, mockOrderItemStore, mockTrackingStore, mockOutboxStore, mockCartPort, customer, prod1UUID, _ := setupSagaTestDependencies()

	ctx = orderservice.WithOrderCustomer(ctx, customer)

	existingOrderUUID := uuid.New()
	idemKey := "idem-existing-1"

	// Mock saga already CONFIRMED
	mockSagaRepo.GetByIdempotencyKeyFunc = func(ctx context.Context, userUUID uuid.UUID, key string) (sqlc.OrderSaga, error) {
		return sqlc.OrderSaga{
			OrderUuid:      existingOrderUUID,
			UserUuid:       userUUID,
			IdempotencyKey: key,
			State:          orderservice.SagaStateConfirmed,
		}, nil
	}

	mockOrderStore.GetByUUIDFunc = func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
		return sqlc.GetOrderByUUIDRow{
			OrderID:              999,
			OrderUuid:            existingOrderUUID,
			OrderNumber:          "261018-01-XYZ123",
			OrderUserID:          customer.UserID,
			OrderUserUuid:        customer.UserUUID,
			OrderShippingName:    "Alice",
			OrderShippingPhone:   "0912345678",
			OrderShippingAddress: "Existing Address",
			OrderTotalAmount:     utils.FloatToNumeric(500.0),
			OrderStatus:          1,
		}, nil
	}

	mockOrderItemStore.GetByOrderIDFunc = func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
		return []sqlc.GetOrderItemsByOrderIDRow{
			{
				OrderItemID:          1,
				OrderItemOrderID:     orderID,
				OrderItemProductName: "Existing Product",
				OrderItemQuantity:    1,
				OrderItemPrice:       utils.FloatToNumeric(500.0),
				OrderItemSubtotal:    utils.FloatToNumeric(500.0),
			},
		}, nil
	}

	// Stock must NOT be deducted again
	stockDeducted := false
	mockStockPort.LockAndDeductStockWithRefFunc = func(ctx context.Context, items []orderservice.DeductItem, refID, refType string, actorID *int32, note *string) error {
		stockDeducted = true
		return nil
	}

	customerAdapter := &MockCustomerReader{}
	orchestrator := orderservice.NewOrderSagaOrchestrator(
		mockSagaRepo,
		testutils.NewNoopTxManager(),
		&MockOrderPricingService{},
		&MockCatalogPort{},
		mockStockPort,
		mockOrderStore,
		mockOrderItemStore,
		customerAdapter,
		mockCartPort,
		mockCouponPort,
		mockTrackingStore,
		nil,
		mockOutboxStore,
	)

	input := v1dto.CreateOrderInput{
		Items: []v1dto.CreateOrderItemInput{
			{ProductUUID: prod1UUID, Quantity: 1},
		},
	}

	dto, err := orchestrator.ExecuteCheckoutSaga(ctx, customer.UserUUID, idemKey, input)
	require.NoError(t, err)
	assert.Equal(t, existingOrderUUID.String(), dto.UUID)
	assert.False(t, stockDeducted, "Stock should NOT be deducted on idempotent replay")
}

func TestOrderSagaOrchestrator_Compensation_CouponFailure(t *testing.T) {
	ctx := context.Background()
	mockSagaRepo, mockStockPort, mockCouponPort, mockOrderStore, mockOrderItemStore, mockTrackingStore, mockOutboxStore, mockCartPort, customer, prod1UUID, _ := setupSagaTestDependencies()

	ctx = orderservice.WithOrderCustomer(ctx, customer)

	// Saga tracking
	var currentSaga sqlc.OrderSaga
	mockSagaRepo.GetByIdempotencyKeyFunc = func(ctx context.Context, userUUID uuid.UUID, key string) (sqlc.OrderSaga, error) {
		return sqlc.OrderSaga{}, pgx.ErrNoRows
	}
	mockSagaRepo.StartFunc = func(ctx context.Context, tx pgx.Tx, params sqlc.StartSagaParams) (sqlc.OrderSaga, error) {
		currentSaga = sqlc.OrderSaga{
			OrderUuid:      params.OrderUuid,
			UserUuid:       params.UserUuid,
			IdempotencyKey: params.IdempotencyKey,
			State:          orderservice.SagaStateStarted,
			Snapshot:       params.Snapshot,
		}
		return currentSaga, nil
	}
	mockSagaRepo.UpdateStepFunc = func(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, lastStep *string, snapshot []byte) (bool, error) {
		currentSaga.LastStep = lastStep
		currentSaga.Snapshot = snapshot
		return true, nil
	}
	mockSagaRepo.ClaimForCompensationFunc = func(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, reason *string) (bool, error) {
		currentSaga.State = orderservice.SagaStateCompensating
		currentSaga.FailureReason = reason
		return true, nil
	}
	mockSagaRepo.GetByOrderUUIDFunc = func(ctx context.Context, orderUUID uuid.UUID) (sqlc.OrderSaga, error) {
		return currentSaga, nil
	}
	mockSagaRepo.MarkFailedFunc = func(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, reason *string) (bool, error) {
		currentSaga.State = orderservice.SagaStateFailed
		currentSaga.FailureReason = reason
		return true, nil
	}

	// Stock deduct succeeds, but we watch for RestoreStock
	var restoredItems []orderservice.RestoreItem
	mockStockPort.LockAndDeductStockWithRefFunc = func(ctx context.Context, items []orderservice.DeductItem, refID, refType string, actorID *int32, note *string) error {
		return nil
	}
	mockStockPort.RestoreStockWithRefFunc = func(ctx context.Context, items []orderservice.RestoreItem, refID, refType string, actorID *int32, note *string) error {
		restoredItems = items
		return nil
	}

	// Coupon lock and apply FAILS
	mockCouponPort.LockAndApplyCouponFunc = func(ctx context.Context, couponID, userID, orderID int32, orderUUID *uuid.UUID) error {
		return errors.New("coupon usage limit exceeded")
	}

	couponID := int32(42)
	couponCode := "PROMO50"
	mockPricingService := &MockOrderPricingService{
		CalculateOrderTotalsFunc: func(ctx context.Context, items []orderservice.ItemPriceInput, code *string, userID int32, fee float64, now time.Time) (*orderservice.OrderPricingResult, error) {
			return &orderservice.OrderPricingResult{
				SubtotalAmount:    200.0,
				FinalTotalAmount:  150.0,
				AppliedCouponID:   &couponID,
				AppliedCouponCode: &couponCode,
				Items: []orderservice.ItemPriceResult{
					{ProductID: 1, ProductName: "Item", Price: 100.0, Quantity: 2, Subtotal: 200.0},
				},
			}, nil
		},
	}

	mockCatalogPort := &MockCatalogPort{
		GetProductsForCheckoutFunc: func(ctx context.Context, uuids []uuid.UUID) (map[string]orderservice.CheckoutProductSnapshot, error) {
			return map[string]orderservice.CheckoutProductSnapshot{
				prod1UUID: {
					ProductID:   1,
					ProductUUID: uuid.MustParse(prod1UUID),
					ProductName: "Item",
					Price:       100.0,
					Status:      orderservice.ProductStatusActive,
				},
			}, nil
		},
	}

	customerAdapter := &MockCustomerReader{}
	orchestrator := orderservice.NewOrderSagaOrchestrator(
		mockSagaRepo,
		testutils.NewNoopTxManager(),
		mockPricingService,
		mockCatalogPort,
		mockStockPort,
		mockOrderStore,
		mockOrderItemStore,
		customerAdapter,
		mockCartPort,
		mockCouponPort,
		mockTrackingStore,
		nil,
		mockOutboxStore,
	)

	input := v1dto.CreateOrderInput{
		Items: []v1dto.CreateOrderItemInput{
			{ProductUUID: prod1UUID, Quantity: 2},
		},
		CouponCode: &couponCode,
	}

	_, err := orchestrator.ExecuteCheckoutSaga(ctx, customer.UserUUID, "idem-fail-coupon", input)
	assert.Error(t, err)
	assert.Equal(t, orderservice.SagaStateFailed, currentSaga.State)
	assert.Len(t, restoredItems, 1, "Stock must be restored when coupon step fails")
	assert.Equal(t, int32(2), restoredItems[0].Quantity)
}

func TestOrderSagaOrchestrator_Compensation_OrderPersistFailure(t *testing.T) {
	ctx := context.Background()
	mockSagaRepo, mockStockPort, mockCouponPort, mockOrderStore, mockOrderItemStore, mockTrackingStore, mockOutboxStore, mockCartPort, customer, prod1UUID, _ := setupSagaTestDependencies()

	ctx = orderservice.WithOrderCustomer(ctx, customer)

	var currentSaga sqlc.OrderSaga
	mockSagaRepo.GetByIdempotencyKeyFunc = func(ctx context.Context, userUUID uuid.UUID, key string) (sqlc.OrderSaga, error) {
		return sqlc.OrderSaga{}, pgx.ErrNoRows
	}
	mockSagaRepo.StartFunc = func(ctx context.Context, tx pgx.Tx, params sqlc.StartSagaParams) (sqlc.OrderSaga, error) {
		currentSaga = sqlc.OrderSaga{
			OrderUuid:      params.OrderUuid,
			UserUuid:       params.UserUuid,
			IdempotencyKey: params.IdempotencyKey,
			State:          orderservice.SagaStateStarted,
			Snapshot:       params.Snapshot,
		}
		return currentSaga, nil
	}
	mockSagaRepo.UpdateStepFunc = func(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, lastStep *string, snapshot []byte) (bool, error) {
		currentSaga.LastStep = lastStep
		currentSaga.Snapshot = snapshot
		return true, nil
	}
	mockSagaRepo.ClaimForCompensationFunc = func(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, reason *string) (bool, error) {
		currentSaga.State = orderservice.SagaStateCompensating
		return true, nil
	}
	mockSagaRepo.GetByOrderUUIDFunc = func(ctx context.Context, orderUUID uuid.UUID) (sqlc.OrderSaga, error) {
		return currentSaga, nil
	}
	mockSagaRepo.MarkFailedFunc = func(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, reason *string) (bool, error) {
		currentSaga.State = orderservice.SagaStateFailed
		return true, nil
	}

	// Stock deduct succeeds
	stockRestored := false
	mockStockPort.LockAndDeductStockWithRefFunc = func(ctx context.Context, items []orderservice.DeductItem, refID, refType string, actorID *int32, note *string) error {
		return nil
	}
	mockStockPort.RestoreStockWithRefFunc = func(ctx context.Context, items []orderservice.RestoreItem, refID, refType string, actorID *int32, note *string) error {
		stockRestored = true
		return nil
	}

	// Coupon succeeds
	couponReleased := false
	couponID := int32(50)
	couponCode := "VIP100"
	mockCouponPort.LockAndApplyCouponFunc = func(ctx context.Context, cID, uID, oID int32, oUUID *uuid.UUID) error {
		return nil
	}
	mockCouponPort.ReleaseCouponFunc = func(ctx context.Context, cID, oID int32, oUUID *uuid.UUID) error {
		couponReleased = true
		return nil
	}

	// Order store Create FAILS (DB error)
	mockOrderStore.CreateFunc = func(ctx context.Context, params sqlc.CreateOrderParams) (sqlc.Order, error) {
		return sqlc.Order{}, errors.New("deadlock detected on orders table")
	}

	mockCatalogPort := &MockCatalogPort{
		GetProductsForCheckoutFunc: func(ctx context.Context, uuids []uuid.UUID) (map[string]orderservice.CheckoutProductSnapshot, error) {
			return map[string]orderservice.CheckoutProductSnapshot{
				prod1UUID: {
					ProductID:   1,
					ProductUUID: uuid.MustParse(prod1UUID),
					ProductName: "Item",
					Price:       100.0,
					Status:      orderservice.ProductStatusActive,
				},
			}, nil
		},
	}

	mockPricingService := &MockOrderPricingService{
		CalculateOrderTotalsFunc: func(ctx context.Context, items []orderservice.ItemPriceInput, code *string, userID int32, fee float64, now time.Time) (*orderservice.OrderPricingResult, error) {
			return &orderservice.OrderPricingResult{
				SubtotalAmount:    100.0,
				FinalTotalAmount:  80.0,
				AppliedCouponID:   &couponID,
				AppliedCouponCode: &couponCode,
				Items: []orderservice.ItemPriceResult{
					{ProductID: 1, ProductName: "Item", Price: 100.0, Quantity: 1, Subtotal: 100.0},
				},
			}, nil
		},
	}

	customerAdapter := &MockCustomerReader{}
	orchestrator := orderservice.NewOrderSagaOrchestrator(
		mockSagaRepo,
		testutils.NewNoopTxManager(),
		mockPricingService,
		mockCatalogPort,
		mockStockPort,
		mockOrderStore,
		mockOrderItemStore,
		customerAdapter,
		mockCartPort,
		mockCouponPort,
		mockTrackingStore,
		nil,
		mockOutboxStore,
	)

	input := v1dto.CreateOrderInput{
		Items: []v1dto.CreateOrderItemInput{
			{ProductUUID: prod1UUID, Quantity: 1},
		},
		CouponCode: &couponCode,
	}

	_, err := orchestrator.ExecuteCheckoutSaga(ctx, customer.UserUUID, "idem-fail-order-create", input)
	assert.Error(t, err)
	assert.True(t, couponReleased, "Coupon must be released on backwards compensation")
	assert.True(t, stockRestored, "Stock must be restored on backwards compensation")
	assert.Equal(t, orderservice.SagaStateFailed, currentSaga.State)
}

func TestOrderSagaOrchestrator_Compensation_PartialFailure_ScheduleRetry(t *testing.T) {
	ctx := context.Background()
	mockSagaRepo, mockStockPort, mockCouponPort, mockOrderStore, mockOrderItemStore, mockTrackingStore, mockOutboxStore, mockCartPort, customer, prod1UUID, _ := setupSagaTestDependencies()

	ctx = orderservice.WithOrderCustomer(ctx, customer)

	orderUUID := uuid.New()
	snapshot := orderservice.SagaSnapshot{
		OrderUUID:     orderUUID,
		OrderNumber:   "261018-01-FAIL",
		StockReserved: true,
		DeductItems: []orderservice.SagaDeductItem{
			{ProductUUID: prod1UUID, ProductID: 1, Quantity: 1},
		},
	}
	snapBytes, _ := json.Marshal(snapshot)

	saga := sqlc.OrderSaga{
		OrderUuid: orderUUID,
		UserUuid:  customer.UserUUID,
		State:     orderservice.SagaStateStarted,
		Snapshot:  snapBytes,
	}

	mockSagaRepo.ClaimForCompensationFunc = func(ctx context.Context, tx pgx.Tx, u uuid.UUID, reason *string) (bool, error) {
		saga.State = orderservice.SagaStateCompensating
		return true, nil
	}
	mockSagaRepo.GetByOrderUUIDFunc = func(ctx context.Context, u uuid.UUID) (sqlc.OrderSaga, error) {
		return saga, nil
	}

	var scheduledRetryReason string
	mockSagaRepo.ScheduleRetryFunc = func(ctx context.Context, tx pgx.Tx, u uuid.UUID, reason *string) (bool, error) {
		if reason != nil {
			scheduledRetryReason = *reason
		}
		return true, nil
	}

	// Stock restoration fails (e.g. gRPC network timeout)
	mockStockPort.RestoreStockWithRefFunc = func(ctx context.Context, items []orderservice.RestoreItem, refID, refType string, actorID *int32, note *string) error {
		return errors.New("inventory service connection timeout")
	}

	customerAdapter := &MockCustomerReader{}
	orchestrator := orderservice.NewOrderSagaOrchestrator(
		mockSagaRepo,
		testutils.NewNoopTxManager(),
		&MockOrderPricingService{},
		&MockCatalogPort{},
		mockStockPort,
		mockOrderStore,
		mockOrderItemStore,
		customerAdapter,
		mockCartPort,
		mockCouponPort,
		mockTrackingStore,
		nil,
		mockOutboxStore,
	)

	err := orchestrator.CompensateSaga(ctx, orderUUID, "Simulated order crash")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scheduled for retry")
	assert.Contains(t, scheduledRetryReason, "inventory service connection timeout")
}

func TestOrderSagaOrchestrator_G2_G3_UUIDPropagation(t *testing.T) {
	ctx := context.Background()
	mockSagaRepo, mockStockPort, mockCouponPort, mockOrderStore, mockOrderItemStore, mockTrackingStore, mockOutboxStore, mockCartPort, customer, prod1UUID, _ := setupSagaTestDependencies()

	ctx = orderservice.WithOrderCustomer(ctx, customer)

	couponCode := "DISCOUNT10"
	couponID := int32(88)

	var capturedStockRefID string
	var capturedCouponOrderUUID *uuid.UUID

	mockSagaRepo.GetByIdempotencyKeyFunc = func(ctx context.Context, userUUID uuid.UUID, key string) (sqlc.OrderSaga, error) {
		return sqlc.OrderSaga{}, pgx.ErrNoRows
	}
	mockSagaRepo.StartFunc = func(ctx context.Context, tx pgx.Tx, params sqlc.StartSagaParams) (sqlc.OrderSaga, error) {
		return sqlc.OrderSaga{
			OrderUuid:      params.OrderUuid,
			UserUuid:       params.UserUuid,
			IdempotencyKey: params.IdempotencyKey,
			State:          orderservice.SagaStateStarted,
		}, nil
	}
	mockSagaRepo.ConfirmFunc = func(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (bool, error) {
		return true, nil
	}

	mockStockPort.LockAndDeductStockWithRefFunc = func(ctx context.Context, items []orderservice.DeductItem, refID, refType string, actorID *int32, note *string) error {
		capturedStockRefID = refID
		return nil
	}

	mockCouponPort.LockAndApplyCouponFunc = func(ctx context.Context, cID, uID, oID int32, oUUID *uuid.UUID) error {
		capturedCouponOrderUUID = oUUID
		return nil
	}

	mockCatalogPort := &MockCatalogPort{
		GetProductsForCheckoutFunc: func(ctx context.Context, uuids []uuid.UUID) (map[string]orderservice.CheckoutProductSnapshot, error) {
			return map[string]orderservice.CheckoutProductSnapshot{
				prod1UUID: {
					ProductID:   1,
					ProductUUID: uuid.MustParse(prod1UUID),
					ProductName: "Mechanical Keyboard",
					Price:       150.0,
					Status:      orderservice.ProductStatusActive,
				},
			}, nil
		},
	}

	mockPricingService := &MockOrderPricingService{
		CalculateOrderTotalsFunc: func(ctx context.Context, items []orderservice.ItemPriceInput, code *string, userID int32, fee float64, now time.Time) (*orderservice.OrderPricingResult, error) {
			return &orderservice.OrderPricingResult{
				SubtotalAmount:    150.0,
				ShippingFee:       15.0,
				DiscountAmount:    15.0,
				FinalTotalAmount:  150.0,
				AppliedCouponID:   &couponID,
				AppliedCouponCode: &couponCode,
				Items: []orderservice.ItemPriceResult{
					{ProductID: 1, ProductName: "Mechanical Keyboard", Price: 150.0, Quantity: 1, Subtotal: 150.0},
				},
			}, nil
		},
	}

	mockOrderStore.CreateFunc = func(ctx context.Context, params sqlc.CreateOrderParams) (sqlc.Order, error) {
		return sqlc.Order{
			OrderID:          1001,
			OrderUuid:        params.OrderUuid,
			OrderNumber:      params.OrderNumber,
			OrderUserID:      params.OrderUserID,
			OrderUserUuid:    params.OrderUserUuid,
			OrderTotalAmount: params.OrderTotalAmount,
		}, nil
	}

	mockOrderItemStore.CreateFunc = func(ctx context.Context, tx pgx.Tx, params sqlc.CreateOrderItemParams) (sqlc.OrderItem, error) {
		return sqlc.OrderItem{OrderItemID: 2001}, nil
	}

	customerAdapter := &MockCustomerReader{}
	orchestrator := orderservice.NewOrderSagaOrchestrator(
		mockSagaRepo,
		testutils.NewNoopTxManager(),
		mockPricingService,
		mockCatalogPort,
		mockStockPort,
		mockOrderStore,
		mockOrderItemStore,
		customerAdapter,
		mockCartPort,
		mockCouponPort,
		mockTrackingStore,
		nil,
		mockOutboxStore,
	)

	input := v1dto.CreateOrderInput{
		Items: []v1dto.CreateOrderItemInput{
			{ProductUUID: prod1UUID, Quantity: 1},
		},
		ShippingName:    "Alice",
		ShippingPhone:   "0123456789",
		ShippingAddress: "123 Street",
		PaymentMethod:   1,
		CouponCode:      &couponCode,
	}

	idemKey := "idem-test-g2-g3"
	res, err := orchestrator.ExecuteCheckoutSaga(ctx, customer.UserUUID, idemKey, input)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	// G2 Verification: LockAndDeductStockWithRef must receive orderUUID as refID
	assert.Equal(t, res.UUID, capturedStockRefID, "Stock refID must be orderUUID")

	// G3 Verification: LockAndApplyCoupon must receive orderUUID
	assert.NotNil(t, capturedCouponOrderUUID)
	assert.Equal(t, res.UUID, capturedCouponOrderUUID.String(), "Coupon orderUUID must match orderUUID")
}
