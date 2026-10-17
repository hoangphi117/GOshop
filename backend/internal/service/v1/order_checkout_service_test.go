package v1service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/testutils"
	"goshop/internal/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderCheckoutService_CreateOrder(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	customer := &v1service.OrderCustomer{
		UserID:       1,
		UserUUID:     userUUID,
		FullName:     "John Doe",
		UserFullname: "John Doe",
		Email:        "john@example.com",
		UserEmail:    "john@example.com",
		Status:       1,
	}

	prodUUID1 := uuid.New().String()
	prodUUID2 := uuid.New().String()

	t.Run("User not found -> NotFound error", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}
		customerAdapter := v1service.NewOrderCustomerAdapter(mockUserRepo)
		svc := v1service.NewOrderCheckoutService(
			testutils.NewNoopTxManager(),
			&testutils.MockOrderPricingService{},
			&testutils.MockCatalogPort{},
			&testutils.MockOrderStockPort{},
			&testutils.MockOrderRepository{},
			&testutils.MockOrderItemRepository{},
			customerAdapter,
			&testutils.MockCartService{},
			&testutils.MockOrderCouponPort{},
			&testutils.MockTrackingRepository{},
			nil,
		)

		input := v1dto.CreateOrderInput{
			Items: []v1dto.CreateOrderItemInput{
				{ProductUUID: prodUUID1, Quantity: 2},
			},
		}
		res, err := svc.CreateOrder(ctx, userUUID, input)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "User not found")
	})

	t.Run("Duplicate products in order -> BadRequest error", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: customer.UserID, UserUuid: customer.UserUUID}, nil
			},
		}
		customerAdapter := v1service.NewOrderCustomerAdapter(mockUserRepo)
		svc := v1service.NewOrderCheckoutService(
			testutils.NewNoopTxManager(),
			&testutils.MockOrderPricingService{},
			&testutils.MockCatalogPort{},
			&testutils.MockOrderStockPort{},
			&testutils.MockOrderRepository{},
			&testutils.MockOrderItemRepository{},
			customerAdapter,
			&testutils.MockCartService{},
			&testutils.MockOrderCouponPort{},
			&testutils.MockTrackingRepository{},
			nil,
		)

		input := v1dto.CreateOrderInput{
			Items: []v1dto.CreateOrderItemInput{
				{ProductUUID: prodUUID1, Quantity: 1},
				{ProductUUID: prodUUID1, Quantity: 2},
			},
		}
		res, err := svc.CreateOrder(ctx, userUUID, input)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "Duplicate product in order")
	})

	t.Run("Stock deduction fails -> returns error", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: customer.UserID, UserUuid: customer.UserUUID}, nil
			},
		}
		customerAdapter := v1service.NewOrderCustomerAdapter(mockUserRepo)
		mockCatalog := &testutils.MockCatalogPort{
			GetProductsForCheckoutFunc: func(ctx context.Context, productUUIDs []uuid.UUID) (map[string]v1service.CheckoutProductSnapshot, error) {
				return map[string]v1service.CheckoutProductSnapshot{
					prodUUID1: {
						ProductID:   10,
						ProductUUID: uuid.MustParse(prodUUID1),
						ProductName: "P1",
						Price:       100,
						Status:      v1service.ProductStatusActive,
					},
				}, nil
			},
		}
		mockStock := &testutils.MockOrderStockPort{
			LockAndDeductStockWithRefFunc: func(ctx context.Context, items []v1service.DeductItem, refID, refType string, actorID *int32, note *string) error {
				return errors.New("insufficient inventory stock")
			},
		}
		svc := v1service.NewOrderCheckoutService(
			testutils.NewNoopTxManager(),
			&testutils.MockOrderPricingService{},
			mockCatalog,
			mockStock,
			&testutils.MockOrderRepository{},
			&testutils.MockOrderItemRepository{},
			customerAdapter,
			&testutils.MockCartService{},
			&testutils.MockOrderCouponPort{},
			&testutils.MockTrackingRepository{},
			nil,
		)

		input := v1dto.CreateOrderInput{
			Items: []v1dto.CreateOrderItemInput{
				{ProductUUID: prodUUID1, Quantity: 5},
			},
		}
		res, err := svc.CreateOrder(ctx, userUUID, input)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "insufficient inventory stock")
	})

	t.Run("Successful checkout with transactional outbox event", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: customer.UserID, UserUuid: customer.UserUUID}, nil
			},
		}
		customerAdapter := v1service.NewOrderCustomerAdapter(mockUserRepo)

		mockCatalog := &testutils.MockCatalogPort{
			GetProductsForCheckoutFunc: func(ctx context.Context, productUUIDs []uuid.UUID) (map[string]v1service.CheckoutProductSnapshot, error) {
				return map[string]v1service.CheckoutProductSnapshot{
					prodUUID1: {ProductID: 10, ProductUUID: uuid.MustParse(prodUUID1), Price: 100000, ProductName: "Product 1", ProductSlug: "product-1", Status: v1service.ProductStatusActive},
					prodUUID2: {ProductID: 20, ProductUUID: uuid.MustParse(prodUUID2), Price: 200000, ProductName: "Product 2", ProductSlug: "product-2", Status: v1service.ProductStatusActive},
				}, nil
			},
		}

		mockStock := &testutils.MockOrderStockPort{
			LockAndDeductStockWithRefFunc: func(ctx context.Context, items []v1service.DeductItem, refID, refType string, actorID *int32, note *string) error {
				return nil
			},
			RestoreStockWithRefFunc: func(ctx context.Context, items []v1service.RestoreItem, refID, refType string, actorID *int32, note *string) error {
				return nil
			},
		}

		mockPricing := &testutils.MockOrderPricingService{
			CalculateOrderTotalsFunc: func(ctx context.Context, items []v1service.ItemPriceInput, couponCode *string, userID int32, shippingFee float64, now time.Time) (*v1service.OrderPricingResult, error) {
				return &v1service.OrderPricingResult{
					SubtotalAmount:   400000,
					DiscountAmount:   0,
					ShippingFee:      30000,
					FinalTotalAmount: 430000,
					Items: []v1service.ItemPriceResult{
						{ProductID: 10, ProductName: "Product 1", Price: 100000, Quantity: 2, Subtotal: 200000},
						{ProductID: 20, ProductName: "Product 2", Price: 200000, Quantity: 1, Subtotal: 200000},
					},
				}, nil
			},
		}

		createdOrderUUID := uuid.New()
		mockOrderRepo := &testutils.MockOrderRepository{
			CreateFunc: func(ctx context.Context, params sqlc.CreateOrderParams) (sqlc.Order, error) {
				return sqlc.Order{
					OrderID:              99,
					OrderUuid:            createdOrderUUID,
					OrderNumber:          params.OrderNumber,
					OrderUserID:          customer.UserID,
					OrderStatus:          v1service.OrderStatusPending,
					OrderPaymentStatus:   v1service.OrderPaymentStatusUnpaid,
					OrderPaymentMethod:   params.OrderPaymentMethod,
					OrderSubtotalAmount:  params.OrderSubtotalAmount,
					OrderDiscountAmount:  params.OrderDiscountAmount,
					OrderTotalAmount:     params.OrderTotalAmount,
					OrderShippingFee:     params.OrderShippingFee,
					OrderShippingName:    params.OrderShippingName,
					OrderShippingPhone:   params.OrderShippingPhone,
					OrderShippingAddress: params.OrderShippingAddress,
					OrderCreatedAt:       time.Now(),
				}, nil
			},
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:             99,
					OrderUuid:           u,
					OrderNumber:         "ORD-TEST-99",
					OrderUserID:         customer.UserID,
					OrderStatus:         v1service.OrderStatusPending,
					OrderPaymentStatus:  v1service.OrderPaymentStatusUnpaid,
					OrderSubtotalAmount: utils.FloatToNumeric(400000),
					OrderTotalAmount:    utils.FloatToNumeric(430000),
				}, nil
			},
		}

		mockOrderItemRepo := &testutils.MockOrderItemRepository{
			CreateFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.CreateOrderItemParams) (sqlc.OrderItem, error) {
				return sqlc.OrderItem{
					OrderItemID:        1,
					OrderItemOrderID:   params.OrderItemOrderID,
					OrderItemProductID: params.OrderItemProductID,
					OrderItemQuantity:  params.OrderItemQuantity,
					OrderItemPrice:     params.OrderItemPrice,
					OrderItemSubtotal:  params.OrderItemSubtotal,
				}, nil
			},
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				return []sqlc.GetOrderItemsByOrderIDRow{
					{
						OrderItemID:          1,
						OrderItemProductID:   10,
						OrderItemProductName: "Product 1",
						OrderItemQuantity:    2,
						OrderItemPrice:       utils.FloatToNumeric(100000),
						OrderItemSubtotal:    utils.FloatToNumeric(200000),
					},
				}, nil
			},
		}

		outboxInserted := false
		mockOutbox := &testutils.MockOutboxRepository{
			InsertFunc: func(ctx context.Context, tx pgx.Tx, aggType, aggID, evtType string, payload any) (sqlc.OutboxEvent, error) {
				outboxInserted = true
				return sqlc.OutboxEvent{
					EventID:       1,
					AggregateType: aggType,
					AggregateID:   aggID,
					EventType:     evtType,
				}, nil
			},
		}

		mockTracking := &testutils.MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				return sqlc.OrderTrackingEvent{}, nil
			},
		}

		svc := v1service.NewOrderCheckoutService(
			testutils.NewNoopTxManager(),
			mockPricing,
			mockCatalog,
			mockStock,
			mockOrderRepo,
			mockOrderItemRepo,
			customerAdapter,
			&testutils.MockCartService{},
			&testutils.MockOrderCouponPort{},
			mockTracking,
			nil,
			mockOutbox,
		)

		input := v1dto.CreateOrderInput{
			ShippingName:    "John Doe",
			ShippingPhone:   "0901234567",
			ShippingAddress: "123 Street, District 1, HCMC",
			PaymentMethod:   v1service.OrderPaymentMethodCOD,
			Items: []v1dto.CreateOrderItemInput{
				{ProductUUID: prodUUID1, Quantity: 2},
				{ProductUUID: prodUUID2, Quantity: 1},
			},
		}

		res, err := svc.CreateOrder(ctx, userUUID, input)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, createdOrderUUID.String(), res.UUID)
		assert.Equal(t, float64(430000), res.TotalAmount)
		assert.True(t, outboxInserted, "Transactional outbox event must be inserted")
	})

	t.Run("Optimized checkout using user claims from context and memory items", func(t *testing.T) {
		userRepoCalled := false
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				userRepoCalled = true
				return sqlc.User{}, errors.New("user repo should not be called when claims are in context")
			},
		}
		customerAdapter := v1service.NewOrderCustomerAdapter(mockUserRepo)

		mockCatalog := &testutils.MockCatalogPort{
			GetProductsForCheckoutFunc: func(ctx context.Context, productUUIDs []uuid.UUID) (map[string]v1service.CheckoutProductSnapshot, error) {
				return map[string]v1service.CheckoutProductSnapshot{
					prodUUID1: {ProductID: 10, ProductUUID: uuid.MustParse(prodUUID1), Price: 100000, ProductName: "Product 1", ProductSlug: "product-1", Status: v1service.ProductStatusActive},
				}, nil
			},
		}

		mockStock := &testutils.MockOrderStockPort{
			LockAndDeductStockWithRefFunc: func(ctx context.Context, items []v1service.DeductItem, refID, refType string, actorID *int32, note *string) error {
				return nil
			},
		}

		mockPricing := &testutils.MockOrderPricingService{
			CalculateOrderTotalsFunc: func(ctx context.Context, items []v1service.ItemPriceInput, couponCode *string, userID int32, shippingFee float64, now time.Time) (*v1service.OrderPricingResult, error) {
				return &v1service.OrderPricingResult{
					SubtotalAmount:   200000,
					DiscountAmount:   0,
					ShippingFee:      0,
					FinalTotalAmount: 200000,
					Items: []v1service.ItemPriceResult{
						{ProductID: 10, ProductName: "Product 1", Price: 100000, Quantity: 2, Subtotal: 200000},
					},
				}, nil
			},
		}

		createdOrderUUID := uuid.New()
		mockOrderRepo := &testutils.MockOrderRepository{
			CreateFunc: func(ctx context.Context, params sqlc.CreateOrderParams) (sqlc.Order, error) {
				return sqlc.Order{
					OrderID:            100,
					OrderUuid:          createdOrderUUID,
					OrderNumber:        params.OrderNumber,
					OrderUserID:        customer.UserID,
					OrderStatus:        v1service.OrderStatusPending,
					OrderPaymentStatus: v1service.OrderPaymentStatusUnpaid,
					OrderPaymentMethod: params.OrderPaymentMethod,
					OrderTotalAmount:   params.OrderTotalAmount,
					OrderCreatedAt:     time.Now(),
				}, nil
			},
		}

		getByOrderIDCalled := false
		mockOrderItemRepo := &testutils.MockOrderItemRepository{
			CreateFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.CreateOrderItemParams) (sqlc.OrderItem, error) {
				return sqlc.OrderItem{
					OrderItemID:          15,
					OrderItemOrderID:     params.OrderItemOrderID,
					OrderItemProductID:   params.OrderItemProductID,
					OrderItemProductName: params.OrderItemProductName,
					OrderItemQuantity:    params.OrderItemQuantity,
					OrderItemPrice:       params.OrderItemPrice,
					OrderItemSubtotal:    params.OrderItemSubtotal,
				}, nil
			},
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				getByOrderIDCalled = true
				return nil, errors.New("GetByOrderID should not be called post-commit")
			},
		}

		cartRemovedSync := false
		mockCart := &testutils.MockCartService{
			RemovePurchasedItemsFunc: func(ctx context.Context, userID int32, productIDs []int32) error {
				cartRemovedSync = true
				return nil
			},
		}

		svc := v1service.NewOrderCheckoutService(
			testutils.NewNoopTxManager(),
			mockPricing,
			mockCatalog,
			mockStock,
			mockOrderRepo,
			mockOrderItemRepo,
			customerAdapter,
			mockCart,
			&testutils.MockOrderCouponPort{},
			&testutils.MockTrackingRepository{},
			nil,
		)

		ctxWithClaims := v1service.WithOrderCustomer(ctx, customer)
		input := v1dto.CreateOrderInput{
			ShippingName:    "Jane Doe",
			ShippingPhone:   "0909888777",
			ShippingAddress: "456 Avenue, District 3, HCMC",
			PaymentMethod:   v1service.OrderPaymentMethodCOD,
			Items: []v1dto.CreateOrderItemInput{
				{ProductUUID: prodUUID1, Quantity: 2},
			},
		}

		res, err := svc.CreateOrder(ctxWithClaims, userUUID, input)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, createdOrderUUID.String(), res.UUID)
		assert.Equal(t, 1, len(res.Items))
		assert.Equal(t, prodUUID1, res.Items[0].ProductUUID)
		assert.False(t, userRepoCalled, "userRepo.GetByUUID must NOT be called when customer claims exist in context")
		assert.False(t, getByOrderIDCalled, "orderItemStore.GetByOrderID must NOT be called post-commit")
		assert.False(t, cartRemovedSync, "cartPort.RemovePurchasedItems must NOT be called synchronously during checkout")
	})
}
