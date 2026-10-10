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
	customer := v1service.NewOrderCustomer(1, userUUID, "John Doe", "john@example.com", 3, 1)

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
			&testutils.MockInventoryService{},
			&testutils.MockOrderRepository{},
			&testutils.MockOrderItemRepository{},
			customerAdapter,
			&testutils.MockCartService{},
			&testutils.MockCouponService{},
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
			&testutils.MockInventoryService{},
			&testutils.MockOrderRepository{},
			&testutils.MockOrderItemRepository{},
			customerAdapter,
			&testutils.MockCartService{},
			&testutils.MockCouponService{},
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
		mockInv := &testutils.MockInventoryService{
			LockAndDeductStockWithRefFunc: func(ctx context.Context, items []v1service.DeductItem, refID, refType string, actorID *int32, note *string) (map[string]v1service.InventoryItemStock, error) {
				return nil, errors.New("insufficient inventory stock")
			},
		}
		svc := v1service.NewOrderCheckoutService(
			testutils.NewNoopTxManager(),
			&testutils.MockOrderPricingService{},
			mockInv,
			&testutils.MockOrderRepository{},
			&testutils.MockOrderItemRepository{},
			customerAdapter,
			&testutils.MockCartService{},
			&testutils.MockCouponService{},
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

		mockInv := &testutils.MockInventoryService{
			LockAndDeductStockWithRefFunc: func(ctx context.Context, items []v1service.DeductItem, refID, refType string, actorID *int32, note *string) (map[string]v1service.InventoryItemStock, error) {
				return map[string]v1service.InventoryItemStock{
					prodUUID1: {ProductID: 10, Price: 100000, Stock: 50, ProductName: "Product 1"},
					prodUUID2: {ProductID: 20, Price: 200000, Stock: 30, ProductName: "Product 2"},
				}, nil
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
					OrderID:             99,
					OrderUuid:           createdOrderUUID,
					OrderNumber:         params.OrderNumber,
					OrderUserID:         customer.UserID,
					OrderStatus:         v1service.OrderStatusPending,
					OrderPaymentStatus:  v1service.OrderPaymentStatusUnpaid,
					OrderPaymentMethod:  params.OrderPaymentMethod,
					OrderSubtotalAmount: params.OrderSubtotalAmount,
					OrderDiscountAmount: params.OrderDiscountAmount,
					OrderTotalAmount:    params.OrderTotalAmount,
					OrderShippingFee:    params.OrderShippingFee,
					OrderShippingName:   params.OrderShippingName,
					OrderShippingPhone:  params.OrderShippingPhone,
					OrderShippingAddress: params.OrderShippingAddress,
					OrderCreatedAt:      time.Now(),
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
			mockInv,
			mockOrderRepo,
			mockOrderItemRepo,
			customerAdapter,
			&testutils.MockCartService{},
			&testutils.MockCouponService{},
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
}
