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

func TestOrderService_CreateOrder(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	productUUID1 := uuid.New()
	productUUID2 := uuid.New()

	baseUser := sqlc.User{
		UserID:    1,
		UserUuid:  userUUID,
		UserEmail: "test@example.com",
	}

	baseInput := v1dto.CreateOrderInput{
		ShippingName:    "Nguyen Van A",
		ShippingPhone:   "0987654321",
		ShippingAddress: "123 Le Loi, Q1, HCMC",
		ShippingFee:     30000,
		PaymentMethod:   v1service.OrderPaymentMethodCOD,
		Note:            "Please deliver during office hours",
		Items: []v1dto.CreateOrderItemInput{
			{ProductUUID: productUUID1.String(), Quantity: 2},
			{ProductUUID: productUUID2.String(), Quantity: 1},
		},
	}

	t.Run("Case 1: User Not Found - Returns 404 NotFound", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			&testutils.MockOrderPricingService{},
			&testutils.MockInventoryService{},
			&testutils.MockOrderRepository{},
			&testutils.MockOrderItemRepository{},
			nil,
			&testutils.MockProductRepository{},
			mockUserRepo,
			&testutils.MockCartService{},
			&testutils.MockCartRepository{},
			&testutils.MockCouponRepository{},
			&testutils.MockTrackingRepository{},
			nil,
			nil,
			nil,
			&testutils.MockNotificationService{},
			nil,
			nil,
			nil,
		)

		_, _, err := svc.CreateOrder(ctx, userUUID, baseInput)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "User not found")
	})

	t.Run("Case 2: User Repo Error - Returns Internal Error", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, errors.New("db connection timeout")
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			&testutils.MockOrderPricingService{},
			&testutils.MockInventoryService{},
			&testutils.MockOrderRepository{},
			&testutils.MockOrderItemRepository{},
			nil,
			&testutils.MockProductRepository{},
			mockUserRepo,
			&testutils.MockCartService{},
			&testutils.MockCartRepository{},
			&testutils.MockCouponRepository{},
			&testutils.MockTrackingRepository{},
			nil,
			nil,
			nil,
			&testutils.MockNotificationService{},
			nil,
			nil,
			nil,
		)

		_, _, err := svc.CreateOrder(ctx, userUUID, baseInput)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Failed to get user")
	})

	t.Run("Case 3: Duplicate Product in Order Items - Returns 400 BadRequest", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}

		inputWithDuplicate := baseInput
		inputWithDuplicate.Items = []v1dto.CreateOrderItemInput{
			{ProductUUID: productUUID1.String(), Quantity: 1},
			{ProductUUID: productUUID1.String(), Quantity: 2},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			&testutils.MockOrderPricingService{},
			&testutils.MockInventoryService{},
			&testutils.MockOrderRepository{},
			&testutils.MockOrderItemRepository{},
			nil,
			&testutils.MockProductRepository{},
			mockUserRepo,
			&testutils.MockCartService{},
			&testutils.MockCartRepository{},
			&testutils.MockCouponRepository{},
			&testutils.MockTrackingRepository{},
			nil,
			nil,
			nil,
			&testutils.MockNotificationService{},
			nil,
			nil,
			nil,
		)

		_, _, err := svc.CreateOrder(ctx, userUUID, inputWithDuplicate)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Duplicate product in order")
	})

	t.Run("Case 4: LockAndDeductStock Failure (Out of stock) - Transaction Aborts", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockInventory := &testutils.MockInventoryService{
			LockAndDeductStockFunc: func(ctx context.Context, items []v1service.DeductItem) (map[string]sqlc.GetByUUIDsForUpdateRow, error) {
				return nil, utils.NewError(utils.ErrCodeBadRequest, "Insufficient stock for product: iPhone 15")
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			&testutils.MockOrderPricingService{},
			mockInventory,
			&testutils.MockOrderRepository{},
			&testutils.MockOrderItemRepository{},
			nil,
			&testutils.MockProductRepository{},
			mockUserRepo,
			&testutils.MockCartService{},
			&testutils.MockCartRepository{},
			&testutils.MockCouponRepository{},
			&testutils.MockTrackingRepository{},
			nil,
			nil,
			nil,
			&testutils.MockNotificationService{},
			nil,
			nil,
			nil,
		)

		_, _, err := svc.CreateOrder(ctx, userUUID, baseInput)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Insufficient stock for product: iPhone 15")
	})

	t.Run("Case 5: CalculateOrderTotals Failure (Expired coupon) - Transaction Aborts", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockInventory := &testutils.MockInventoryService{
			LockAndDeductStockFunc: func(ctx context.Context, items []v1service.DeductItem) (map[string]sqlc.GetByUUIDsForUpdateRow, error) {
				return map[string]sqlc.GetByUUIDsForUpdateRow{
					productUUID1.String(): {ProductID: 10, ProductName: "P1", ProductPrice: utils.FloatToNumeric(100)},
					productUUID2.String(): {ProductID: 20, ProductName: "P2", ProductPrice: utils.FloatToNumeric(200)},
				}, nil
			},
		}
		mockPricing := &testutils.MockOrderPricingService{
			CalculateOrderTotalsFunc: func(ctx context.Context, items []v1service.ItemPriceInput, couponCode *string, userID int32, shippingFee float64, now time.Time) (*v1service.OrderPricingResult, error) {
				return nil, utils.NewError(utils.ErrCodeBadRequest, "Coupon has expired")
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			mockPricing,
			mockInventory,
			&testutils.MockOrderRepository{},
			&testutils.MockOrderItemRepository{},
			nil,
			&testutils.MockProductRepository{},
			mockUserRepo,
			&testutils.MockCartService{},
			&testutils.MockCartRepository{},
			&testutils.MockCouponRepository{},
			&testutils.MockTrackingRepository{},
			nil,
			nil,
			nil,
			&testutils.MockNotificationService{},
			nil,
			nil,
			nil,
		)

		_, _, err := svc.CreateOrder(ctx, userUUID, baseInput)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Coupon has expired")
	})

	t.Run("Case 6: Coupon IncrementUsedCount Failure - Transaction Aborts", func(t *testing.T) {
		couponID := int32(99)
		couponCode := "DISCOUNT10"
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockInventory := &testutils.MockInventoryService{
			LockAndDeductStockFunc: func(ctx context.Context, items []v1service.DeductItem) (map[string]sqlc.GetByUUIDsForUpdateRow, error) {
				return map[string]sqlc.GetByUUIDsForUpdateRow{
					productUUID1.String(): {ProductID: 10, ProductName: "P1", ProductPrice: utils.FloatToNumeric(100)},
					productUUID2.String(): {ProductID: 20, ProductName: "P2", ProductPrice: utils.FloatToNumeric(200)},
				}, nil
			},
		}
		mockPricing := &testutils.MockOrderPricingService{
			CalculateOrderTotalsFunc: func(ctx context.Context, items []v1service.ItemPriceInput, couponCodeIn *string, userID int32, shippingFee float64, now time.Time) (*v1service.OrderPricingResult, error) {
				return &v1service.OrderPricingResult{
					Items: []v1service.ItemPriceResult{
						{ProductID: 10, ProductName: "P1", Price: 100, Quantity: 2, Subtotal: 200},
						{ProductID: 20, ProductName: "P2", Price: 200, Quantity: 1, Subtotal: 200},
					},
					SubtotalAmount:    400,
					DiscountAmount:    40,
					FinalTotalAmount:  390,
					ShippingFee:       30,
					AppliedCouponID:   &couponID,
					AppliedCouponCode: &couponCode,
				}, nil
			},
		}
		mockCouponRepo := &testutils.MockCouponRepository{
			IncrementUsedCountFunc: func(ctx context.Context, tx pgx.Tx, cID int32) error {
				return errors.New("db deadlocked")
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			mockPricing,
			mockInventory,
			&testutils.MockOrderRepository{},
			&testutils.MockOrderItemRepository{},
			nil,
			&testutils.MockProductRepository{},
			mockUserRepo,
			&testutils.MockCartService{},
			&testutils.MockCartRepository{},
			mockCouponRepo,
			&testutils.MockTrackingRepository{},
			nil,
			nil,
			nil,
			&testutils.MockNotificationService{},
			nil,
			nil,
			nil,
		)

		_, _, err := svc.CreateOrder(ctx, userUUID, baseInput)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Failed to increment coupon usage count")
	})

	t.Run("Case 7: Order Repo Create Failure - Transaction Aborts", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockInventory := &testutils.MockInventoryService{
			LockAndDeductStockFunc: func(ctx context.Context, items []v1service.DeductItem) (map[string]sqlc.GetByUUIDsForUpdateRow, error) {
				return map[string]sqlc.GetByUUIDsForUpdateRow{
					productUUID1.String(): {ProductID: 10, ProductName: "P1", ProductPrice: utils.FloatToNumeric(100)},
					productUUID2.String(): {ProductID: 20, ProductName: "P2", ProductPrice: utils.FloatToNumeric(200)},
				}, nil
			},
		}
		mockPricing := &testutils.MockOrderPricingService{
			CalculateOrderTotalsFunc: func(ctx context.Context, items []v1service.ItemPriceInput, couponCode *string, userID int32, shippingFee float64, now time.Time) (*v1service.OrderPricingResult, error) {
				return &v1service.OrderPricingResult{
					Items:            []v1service.ItemPriceResult{{ProductID: 10, ProductName: "P1", Price: 100, Quantity: 2, Subtotal: 200}},
					SubtotalAmount:   200,
					FinalTotalAmount: 230,
					ShippingFee:      30,
				}, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			CreateFunc: func(ctx context.Context, params sqlc.CreateOrderParams) (sqlc.Order, error) {
				return sqlc.Order{}, errors.New("insert foreign key violation")
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			mockPricing,
			mockInventory,
			mockOrderRepo,
			&testutils.MockOrderItemRepository{},
			nil,
			&testutils.MockProductRepository{},
			mockUserRepo,
			&testutils.MockCartService{},
			&testutils.MockCartRepository{},
			&testutils.MockCouponRepository{},
			&testutils.MockTrackingRepository{},
			nil,
			nil,
			nil,
			&testutils.MockNotificationService{},
			nil,
			nil,
			nil,
		)

		_, _, err := svc.CreateOrder(ctx, userUUID, baseInput)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create order")
	})

	t.Run("Case 8: Coupon RecordUserUsage Failure - Transaction Aborts", func(t *testing.T) {
		couponID := int32(5)
		couponCode := "PROMO5"
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockInventory := &testutils.MockInventoryService{
			LockAndDeductStockFunc: func(ctx context.Context, items []v1service.DeductItem) (map[string]sqlc.GetByUUIDsForUpdateRow, error) {
				return map[string]sqlc.GetByUUIDsForUpdateRow{
					productUUID1.String(): {ProductID: 10, ProductName: "P1", ProductPrice: utils.FloatToNumeric(100)},
				}, nil
			},
		}
		mockPricing := &testutils.MockOrderPricingService{
			CalculateOrderTotalsFunc: func(ctx context.Context, items []v1service.ItemPriceInput, cCode *string, userID int32, shippingFee float64, now time.Time) (*v1service.OrderPricingResult, error) {
				return &v1service.OrderPricingResult{
					Items:             []v1service.ItemPriceResult{{ProductID: 10, ProductName: "P1", Price: 100, Quantity: 2, Subtotal: 200}},
					SubtotalAmount:    200,
					FinalTotalAmount:  210,
					ShippingFee:       30,
					DiscountAmount:    20,
					AppliedCouponID:   &couponID,
					AppliedCouponCode: &couponCode,
				}, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			CreateFunc: func(ctx context.Context, params sqlc.CreateOrderParams) (sqlc.Order, error) {
				return sqlc.Order{OrderID: 1001, OrderNumber: params.OrderNumber, OrderUserID: baseUser.UserID}, nil
			},
		}
		mockCouponRepo := &testutils.MockCouponRepository{
			IncrementUsedCountFunc: func(ctx context.Context, tx pgx.Tx, cID int32) error {
				return nil
			},
			RecordUserUsageFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.RecordCouponUsageParams) (sqlc.CouponUsage, error) {
				return sqlc.CouponUsage{}, errors.New("db error recording usage")
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			mockPricing,
			mockInventory,
			mockOrderRepo,
			&testutils.MockOrderItemRepository{},
			nil,
			&testutils.MockProductRepository{},
			mockUserRepo,
			&testutils.MockCartService{},
			&testutils.MockCartRepository{},
			mockCouponRepo,
			&testutils.MockTrackingRepository{},
			nil,
			nil,
			nil,
			&testutils.MockNotificationService{},
			nil,
			nil,
			nil,
		)

		_, _, err := svc.CreateOrder(ctx, userUUID, baseInput)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to record coupon usage")
	})

	t.Run("Case 9: OrderItem Repo Create Failure - Transaction Aborts", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockInventory := &testutils.MockInventoryService{
			LockAndDeductStockFunc: func(ctx context.Context, items []v1service.DeductItem) (map[string]sqlc.GetByUUIDsForUpdateRow, error) {
				return map[string]sqlc.GetByUUIDsForUpdateRow{
					productUUID1.String(): {ProductID: 10, ProductName: "P1", ProductPrice: utils.FloatToNumeric(100)},
				}, nil
			},
		}
		mockPricing := &testutils.MockOrderPricingService{
			CalculateOrderTotalsFunc: func(ctx context.Context, items []v1service.ItemPriceInput, cCode *string, userID int32, shippingFee float64, now time.Time) (*v1service.OrderPricingResult, error) {
				return &v1service.OrderPricingResult{
					Items:            []v1service.ItemPriceResult{{ProductID: 10, ProductName: "P1", Price: 100, Quantity: 2, Subtotal: 200}},
					SubtotalAmount:   200,
					FinalTotalAmount: 230,
					ShippingFee:      30,
				}, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			CreateFunc: func(ctx context.Context, params sqlc.CreateOrderParams) (sqlc.Order, error) {
				return sqlc.Order{OrderID: 1001, OrderUserID: baseUser.UserID}, nil
			},
		}
		mockOrderItemRepo := &testutils.MockOrderItemRepository{
			CreateFunc: func(ctx context.Context, tx pgx.Tx, input sqlc.CreateOrderItemParams) (sqlc.OrderItem, error) {
				return sqlc.OrderItem{}, errors.New("cannot create order item")
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			mockPricing,
			mockInventory,
			mockOrderRepo,
			mockOrderItemRepo,
			nil,
			&testutils.MockProductRepository{},
			mockUserRepo,
			&testutils.MockCartService{},
			&testutils.MockCartRepository{},
			&testutils.MockCouponRepository{},
			&testutils.MockTrackingRepository{},
			nil,
			nil,
			nil,
			&testutils.MockNotificationService{},
			nil,
			nil,
			nil,
		)

		_, _, err := svc.CreateOrder(ctx, userUUID, baseInput)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create order item")
	})

	t.Run("Case 10: Tracking Repo InsertEvent Failure - Transaction Aborts", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockInventory := &testutils.MockInventoryService{
			LockAndDeductStockFunc: func(ctx context.Context, items []v1service.DeductItem) (map[string]sqlc.GetByUUIDsForUpdateRow, error) {
				return map[string]sqlc.GetByUUIDsForUpdateRow{
					productUUID1.String(): {ProductID: 10, ProductName: "P1", ProductPrice: utils.FloatToNumeric(100)},
				}, nil
			},
		}
		mockPricing := &testutils.MockOrderPricingService{
			CalculateOrderTotalsFunc: func(ctx context.Context, items []v1service.ItemPriceInput, cCode *string, userID int32, shippingFee float64, now time.Time) (*v1service.OrderPricingResult, error) {
				return &v1service.OrderPricingResult{
					Items:            []v1service.ItemPriceResult{{ProductID: 10, ProductName: "P1", Price: 100, Quantity: 2, Subtotal: 200}},
					SubtotalAmount:   200,
					FinalTotalAmount: 230,
					ShippingFee:      30,
				}, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			CreateFunc: func(ctx context.Context, params sqlc.CreateOrderParams) (sqlc.Order, error) {
				return sqlc.Order{OrderID: 1001, OrderUserID: baseUser.UserID}, nil
			},
		}
		mockOrderItemRepo := &testutils.MockOrderItemRepository{
			CreateFunc: func(ctx context.Context, tx pgx.Tx, input sqlc.CreateOrderItemParams) (sqlc.OrderItem, error) {
				return sqlc.OrderItem{OrderItemID: 1}, nil
			},
		}
		mockTrackingRepo := &testutils.MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				return sqlc.OrderTrackingEvent{}, errors.New("failed tracking insert")
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			mockPricing,
			mockInventory,
			mockOrderRepo,
			mockOrderItemRepo,
			nil,
			&testutils.MockProductRepository{},
			mockUserRepo,
			&testutils.MockCartService{},
			&testutils.MockCartRepository{},
			&testutils.MockCouponRepository{},
			mockTrackingRepo,
			nil,
			nil,
			nil,
			&testutils.MockNotificationService{},
			nil,
			nil,
			nil,
		)

		_, _, err := svc.CreateOrder(ctx, userUUID, baseInput)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to insert tracking event")
	})

	t.Run("Case 11: OrderItem Repo GetByOrderID Failure Post-Commit", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockInventory := &testutils.MockInventoryService{
			LockAndDeductStockFunc: func(ctx context.Context, items []v1service.DeductItem) (map[string]sqlc.GetByUUIDsForUpdateRow, error) {
				return map[string]sqlc.GetByUUIDsForUpdateRow{
					productUUID1.String(): {ProductID: 10, ProductName: "P1", ProductPrice: utils.FloatToNumeric(100)},
				}, nil
			},
		}
		mockPricing := &testutils.MockOrderPricingService{
			CalculateOrderTotalsFunc: func(ctx context.Context, items []v1service.ItemPriceInput, cCode *string, userID int32, shippingFee float64, now time.Time) (*v1service.OrderPricingResult, error) {
				return &v1service.OrderPricingResult{
					Items:            []v1service.ItemPriceResult{{ProductID: 10, ProductName: "P1", Price: 100, Quantity: 2, Subtotal: 200}},
					SubtotalAmount:   200,
					FinalTotalAmount: 230,
					ShippingFee:      30,
				}, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			CreateFunc: func(ctx context.Context, params sqlc.CreateOrderParams) (sqlc.Order, error) {
				return sqlc.Order{OrderID: 1001, OrderUserID: baseUser.UserID}, nil
			},
		}
		mockOrderItemRepo := &testutils.MockOrderItemRepository{
			CreateFunc: func(ctx context.Context, tx pgx.Tx, input sqlc.CreateOrderItemParams) (sqlc.OrderItem, error) {
				return sqlc.OrderItem{OrderItemID: 1}, nil
			},
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				return nil, errors.New("post-commit select failed")
			},
		}
		mockTrackingRepo := &testutils.MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				return sqlc.OrderTrackingEvent{EventID: 1}, nil
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			mockPricing,
			mockInventory,
			mockOrderRepo,
			mockOrderItemRepo,
			nil,
			&testutils.MockProductRepository{},
			mockUserRepo,
			&testutils.MockCartService{},
			&testutils.MockCartRepository{},
			&testutils.MockCouponRepository{},
			mockTrackingRepo,
			nil,
			nil,
			nil,
			&testutils.MockNotificationService{},
			nil,
			nil,
			nil,
		)

		_, _, err := svc.CreateOrder(ctx, userUUID, baseInput)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get order items")
	})

	t.Run("Case 12: Success WITHOUT Coupon - Complete End-to-End Flow", func(t *testing.T) {
		orderUUIDCreated := uuid.New()
		var notifiedInput v1dto.SendNotificationInput
		var deletedCartProductIDs []int32
		var createdTrackingEvent sqlc.InsertTrackingEventParams

		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				assert.Equal(t, userUUID, u)
				return baseUser, nil
			},
		}

		mockInventory := &testutils.MockInventoryService{
			LockAndDeductStockFunc: func(ctx context.Context, items []v1service.DeductItem) (map[string]sqlc.GetByUUIDsForUpdateRow, error) {
				require.Len(t, items, 2)
				assert.Equal(t, productUUID1.String(), items[0].ProductUUID)
				assert.Equal(t, int32(2), items[0].Quantity)
				assert.Equal(t, productUUID2.String(), items[1].ProductUUID)
				assert.Equal(t, int32(1), items[1].Quantity)

				return map[string]sqlc.GetByUUIDsForUpdateRow{
					productUUID1.String(): {ProductID: 101, ProductName: "Keyboard", ProductPrice: utils.FloatToNumeric(500000)},
					productUUID2.String(): {ProductID: 102, ProductName: "Mouse", ProductPrice: utils.FloatToNumeric(250000)},
				}, nil
			},
		}

		mockPricing := &testutils.MockOrderPricingService{
			CalculateOrderTotalsFunc: func(ctx context.Context, items []v1service.ItemPriceInput, couponCode *string, userID int32, shippingFee float64, now time.Time) (*v1service.OrderPricingResult, error) {
				assert.Nil(t, couponCode)
				assert.Equal(t, baseUser.UserID, userID)
				assert.Equal(t, 30000.0, shippingFee)
				assert.Len(t, items, 2)

				return &v1service.OrderPricingResult{
					Items: []v1service.ItemPriceResult{
						{ProductID: 101, ProductName: "Keyboard", Price: 500000, Quantity: 2, Subtotal: 1000000},
						{ProductID: 102, ProductName: "Mouse", Price: 250000, Quantity: 1, Subtotal: 250000},
					},
					SubtotalAmount:    1250000,
					DiscountAmount:    0,
					FinalTotalAmount:  1280000,
					ShippingFee:       30000,
					AppliedCouponID:   nil,
					AppliedCouponCode: nil,
				}, nil
			},
		}

		mockOrderRepo := &testutils.MockOrderRepository{
			CreateFunc: func(ctx context.Context, params sqlc.CreateOrderParams) (sqlc.Order, error) {
				assert.Equal(t, baseUser.UserID, params.OrderUserID)
				assert.Equal(t, "Nguyen Van A", params.OrderShippingName)
				assert.Equal(t, "0987654321", params.OrderShippingPhone)
				assert.Equal(t, "123 Le Loi, Q1, HCMC", params.OrderShippingAddress)
				assert.Equal(t, v1service.OrderStatusPending, params.OrderStatus)
				assert.Equal(t, v1service.OrderPaymentStatusUnpaid, params.OrderPaymentStatus)
				assert.Equal(t, 1280000.0, utils.NumericToFloat(params.OrderTotalAmount))
				assert.Equal(t, 1250000.0, utils.NumericToFloat(params.OrderSubtotalAmount))
				assert.Equal(t, 0.0, utils.NumericToFloat(params.OrderDiscountAmount))
				assert.Nil(t, params.OrderCouponID)

				return sqlc.Order{
					OrderID:              501,
					OrderUuid:            orderUUIDCreated,
					OrderNumber:          params.OrderNumber,
					OrderUserID:          baseUser.UserID,
					OrderTotalAmount:     params.OrderTotalAmount,
					OrderSubtotalAmount:  params.OrderSubtotalAmount,
					OrderShippingFee:     params.OrderShippingFee,
					OrderStatus:          params.OrderStatus,
					OrderPaymentStatus:   params.OrderPaymentStatus,
					OrderShippingName:    params.OrderShippingName,
					OrderShippingPhone:   params.OrderShippingPhone,
					OrderShippingAddress: params.OrderShippingAddress,
				}, nil
			},
		}

		createdItemParamsList := make([]sqlc.CreateOrderItemParams, 0)
		mockOrderItemRepo := &testutils.MockOrderItemRepository{
			CreateFunc: func(ctx context.Context, tx pgx.Tx, input sqlc.CreateOrderItemParams) (sqlc.OrderItem, error) {
				createdItemParamsList = append(createdItemParamsList, input)
				return sqlc.OrderItem{OrderItemID: int32(len(createdItemParamsList))}, nil
			},
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				assert.Equal(t, int32(501), orderID)
				return []sqlc.GetOrderItemsByOrderIDRow{
					{
						OrderItemID:          1,
						OrderItemProductID:   101,
						OrderItemProductName: "Keyboard",
						OrderItemPrice:       utils.FloatToNumeric(500000),
						OrderItemQuantity:    2,
						OrderItemSubtotal:    utils.FloatToNumeric(1000000),
					},
					{
						OrderItemID:          2,
						OrderItemProductID:   102,
						OrderItemProductName: "Mouse",
						OrderItemPrice:       utils.FloatToNumeric(250000),
						OrderItemQuantity:    1,
						OrderItemSubtotal:    utils.FloatToNumeric(250000),
					},
				}, nil
			},
		}

		mockCartRepo := &testutils.MockCartRepository{
			GetCartByUserIDFunc: func(ctx context.Context, userID int32) (sqlc.Cart, error) {
				assert.Equal(t, baseUser.UserID, userID)
				return sqlc.Cart{CartID: 77}, nil
			},
			DeleteCartItemsByProductIDsFunc: func(ctx context.Context, tx pgx.Tx, cartID int32, productIDs []int32) error {
				assert.Equal(t, int32(77), cartID)
				deletedCartProductIDs = productIDs
				return nil
			},
		}

		mockTrackingRepo := &testutils.MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				createdTrackingEvent = params
				return sqlc.OrderTrackingEvent{EventID: 1, EventOrderID: params.EventOrderID}, nil
			},
		}

		mockNotification := &testutils.MockNotificationService{
			SendFunc: func(ctx context.Context, input v1dto.SendNotificationInput) error {
				notifiedInput = input
				return nil
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			mockPricing,
			mockInventory,
			mockOrderRepo,
			mockOrderItemRepo,
			nil,
			&testutils.MockProductRepository{},
			mockUserRepo,
			&testutils.MockCartService{},
			mockCartRepo,
			&testutils.MockCouponRepository{},
			mockTrackingRepo,
			nil,
			nil,
			nil,
			mockNotification,
			nil,
			nil,
			nil,
		)

		order, items, err := svc.CreateOrder(ctx, userUUID, baseInput)
		require.NoError(t, err)

		// Assert Order details
		assert.Equal(t, int32(501), order.OrderID)
		assert.Equal(t, orderUUIDCreated, order.OrderUuid)
		assert.Equal(t, baseUser.UserID, order.OrderUserID)
		assert.Equal(t, 1280000.0, utils.NumericToFloat(order.OrderTotalAmount))

		// Assert Order items
		require.Len(t, items, 2)
		assert.Equal(t, int32(101), items[0].OrderItemProductID)
		assert.Equal(t, "Keyboard", items[0].OrderItemProductName)
		assert.Equal(t, int32(2), items[0].OrderItemQuantity)

		// Assert Created items params
		require.Len(t, createdItemParamsList, 2)
		assert.Equal(t, int32(101), createdItemParamsList[0].OrderItemProductID)
		assert.Equal(t, int32(102), createdItemParamsList[1].OrderItemProductID)

		// Assert Cart cleanup
		assert.Equal(t, []int32{101, 102}, deletedCartProductIDs)

		// Assert Tracking event
		assert.Equal(t, int32(501), createdTrackingEvent.EventOrderID)
		assert.Equal(t, "pending", createdTrackingEvent.EventStatus)
		assert.Equal(t, "user", createdTrackingEvent.EventSource)

		// Assert Notification
		assert.Equal(t, baseUser.UserID, notifiedInput.UserID)
		assert.Equal(t, "order_created", notifiedInput.Type)
		assert.Equal(t, "Order Placed Successfully", notifiedInput.Title)
		assert.Equal(t, orderUUIDCreated.String(), notifiedInput.Data["order_uuid"])
	})

	t.Run("Case 13: Success WITH Applied Coupon - Increments Coupon and Records User Usage", func(t *testing.T) {
		appliedCouponID := int32(88)
		appliedCouponCode := "SAVE10"
		couponInput := baseInput
		couponInput.CouponCode = &appliedCouponCode

		var recordedUsage sqlc.RecordCouponUsageParams
		var incrementedCouponID int32

		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}

		mockInventory := &testutils.MockInventoryService{
			LockAndDeductStockFunc: func(ctx context.Context, items []v1service.DeductItem) (map[string]sqlc.GetByUUIDsForUpdateRow, error) {
				return map[string]sqlc.GetByUUIDsForUpdateRow{
					productUUID1.String(): {ProductID: 101, ProductName: "Keyboard", ProductPrice: utils.FloatToNumeric(500000)},
					productUUID2.String(): {ProductID: 102, ProductName: "Mouse", ProductPrice: utils.FloatToNumeric(250000)},
				}, nil
			},
		}

		mockPricing := &testutils.MockOrderPricingService{
			CalculateOrderTotalsFunc: func(ctx context.Context, items []v1service.ItemPriceInput, couponCode *string, userID int32, shippingFee float64, now time.Time) (*v1service.OrderPricingResult, error) {
				require.NotNil(t, couponCode)
				assert.Equal(t, "SAVE10", *couponCode)

				return &v1service.OrderPricingResult{
					Items: []v1service.ItemPriceResult{
						{ProductID: 101, ProductName: "Keyboard", Price: 500000, Quantity: 2, Subtotal: 1000000},
						{ProductID: 102, ProductName: "Mouse", Price: 250000, Quantity: 1, Subtotal: 250000},
					},
					SubtotalAmount:    1250000,
					DiscountAmount:    125000, // 10% off
					FinalTotalAmount:  1155000, // 1250000 - 125000 + 30000
					ShippingFee:       30000,
					AppliedCouponID:   &appliedCouponID,
					AppliedCouponCode: &appliedCouponCode,
				}, nil
			},
		}

		mockCouponRepo := &testutils.MockCouponRepository{
			IncrementUsedCountFunc: func(ctx context.Context, tx pgx.Tx, cID int32) error {
				incrementedCouponID = cID
				return nil
			},
			RecordUserUsageFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.RecordCouponUsageParams) (sqlc.CouponUsage, error) {
				recordedUsage = params
				return sqlc.CouponUsage{CouponUsageID: 1}, nil
			},
		}

		mockOrderRepo := &testutils.MockOrderRepository{
			CreateFunc: func(ctx context.Context, params sqlc.CreateOrderParams) (sqlc.Order, error) {
				assert.Equal(t, &appliedCouponID, params.OrderCouponID)
				assert.Equal(t, &appliedCouponCode, params.OrderCouponCode)
				assert.Equal(t, 125000.0, utils.NumericToFloat(params.OrderDiscountAmount))
				assert.Equal(t, 1155000.0, utils.NumericToFloat(params.OrderTotalAmount))

				return sqlc.Order{
					OrderID:             601,
					OrderUuid:           uuid.New(),
					OrderNumber:         params.OrderNumber,
					OrderUserID:         baseUser.UserID,
					OrderCouponID:       params.OrderCouponID,
					OrderCouponCode:     params.OrderCouponCode,
					OrderDiscountAmount: params.OrderDiscountAmount,
					OrderTotalAmount:    params.OrderTotalAmount,
				}, nil
			},
		}

		mockOrderItemRepo := &testutils.MockOrderItemRepository{
			CreateFunc: func(ctx context.Context, tx pgx.Tx, input sqlc.CreateOrderItemParams) (sqlc.OrderItem, error) {
				return sqlc.OrderItem{OrderItemID: 1}, nil
			},
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				return []sqlc.GetOrderItemsByOrderIDRow{{OrderItemID: 1, OrderItemProductID: 101}}, nil
			},
		}

		mockCartRepo := &testutils.MockCartRepository{
			GetCartByUserIDFunc: func(ctx context.Context, userID int32) (sqlc.Cart, error) {
				return sqlc.Cart{CartID: 77}, nil
			},
			DeleteCartItemsByProductIDsFunc: func(ctx context.Context, tx pgx.Tx, cartID int32, productIDs []int32) error {
				return nil
			},
		}

		mockTrackingRepo := &testutils.MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				return sqlc.OrderTrackingEvent{EventID: 1}, nil
			},
		}

		mockNotification := &testutils.MockNotificationService{
			SendFunc: func(ctx context.Context, input v1dto.SendNotificationInput) error {
				return nil
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			mockPricing,
			mockInventory,
			mockOrderRepo,
			mockOrderItemRepo,
			nil,
			&testutils.MockProductRepository{},
			mockUserRepo,
			&testutils.MockCartService{},
			mockCartRepo,
			mockCouponRepo,
			mockTrackingRepo,
			nil,
			nil,
			nil,
			mockNotification,
			nil,
			nil,
			nil,
		)

		order, items, err := svc.CreateOrder(ctx, userUUID, couponInput)
		require.NoError(t, err)
		assert.Equal(t, int32(601), order.OrderID)
		assert.Equal(t, &appliedCouponID, order.OrderCouponID)
		assert.Len(t, items, 1)

		// Assert coupon operations
		assert.Equal(t, appliedCouponID, incrementedCouponID)
		assert.Equal(t, appliedCouponID, recordedUsage.CouponUsageCouponID)
		assert.Equal(t, baseUser.UserID, recordedUsage.CouponUsageUserID)
		assert.Equal(t, int32(601), *recordedUsage.CouponUsageOrderID)
	})

	t.Run("Case 14: Notification Failure - Does NOT Fail Order Creation", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockInventory := &testutils.MockInventoryService{
			LockAndDeductStockFunc: func(ctx context.Context, items []v1service.DeductItem) (map[string]sqlc.GetByUUIDsForUpdateRow, error) {
				return map[string]sqlc.GetByUUIDsForUpdateRow{
					productUUID1.String(): {ProductID: 10, ProductName: "P1", ProductPrice: utils.FloatToNumeric(100)},
				}, nil
			},
		}
		mockPricing := &testutils.MockOrderPricingService{
			CalculateOrderTotalsFunc: func(ctx context.Context, items []v1service.ItemPriceInput, cCode *string, userID int32, shippingFee float64, now time.Time) (*v1service.OrderPricingResult, error) {
				return &v1service.OrderPricingResult{
					Items:            []v1service.ItemPriceResult{{ProductID: 10, ProductName: "P1", Price: 100, Quantity: 2, Subtotal: 200}},
					SubtotalAmount:   200,
					FinalTotalAmount: 230,
					ShippingFee:      30,
				}, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			CreateFunc: func(ctx context.Context, params sqlc.CreateOrderParams) (sqlc.Order, error) {
				return sqlc.Order{OrderID: 1001, OrderUserID: baseUser.UserID}, nil
			},
		}
		mockOrderItemRepo := &testutils.MockOrderItemRepository{
			CreateFunc: func(ctx context.Context, tx pgx.Tx, input sqlc.CreateOrderItemParams) (sqlc.OrderItem, error) {
				return sqlc.OrderItem{OrderItemID: 1}, nil
			},
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				return []sqlc.GetOrderItemsByOrderIDRow{{OrderItemID: 1, OrderItemProductID: 10}}, nil
			},
		}
		mockTrackingRepo := &testutils.MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				return sqlc.OrderTrackingEvent{EventID: 1}, nil
			},
		}
		mockCartRepo := &testutils.MockCartRepository{
			GetCartByUserIDFunc: func(ctx context.Context, userID int32) (sqlc.Cart, error) {
				return sqlc.Cart{CartID: 1}, nil
			},
			DeleteCartItemsByProductIDsFunc: func(ctx context.Context, tx pgx.Tx, cartID int32, productIDs []int32) error {
				return nil
			},
		}
		mockNotification := &testutils.MockNotificationService{
			SendFunc: func(ctx context.Context, input v1dto.SendNotificationInput) error {
				return errors.New("fcm service unavailable")
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			mockPricing,
			mockInventory,
			mockOrderRepo,
			mockOrderItemRepo,
			nil,
			&testutils.MockProductRepository{},
			mockUserRepo,
			&testutils.MockCartService{},
			mockCartRepo,
			&testutils.MockCouponRepository{},
			mockTrackingRepo,
			nil,
			nil,
			nil,
			mockNotification,
			nil,
			nil,
			nil,
		)

		order, items, err := svc.CreateOrder(ctx, userUUID, baseInput)
		require.NoError(t, err)
		assert.Equal(t, int32(1001), order.OrderID)
		assert.Len(t, items, 1)
	})

	t.Run("Case 15: Cart Missing or Delete Error - Does NOT Fail Order Creation", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockInventory := &testutils.MockInventoryService{
			LockAndDeductStockFunc: func(ctx context.Context, items []v1service.DeductItem) (map[string]sqlc.GetByUUIDsForUpdateRow, error) {
				return map[string]sqlc.GetByUUIDsForUpdateRow{
					productUUID1.String(): {ProductID: 10, ProductName: "P1", ProductPrice: utils.FloatToNumeric(100)},
				}, nil
			},
		}
		mockPricing := &testutils.MockOrderPricingService{
			CalculateOrderTotalsFunc: func(ctx context.Context, items []v1service.ItemPriceInput, cCode *string, userID int32, shippingFee float64, now time.Time) (*v1service.OrderPricingResult, error) {
				return &v1service.OrderPricingResult{
					Items:            []v1service.ItemPriceResult{{ProductID: 10, ProductName: "P1", Price: 100, Quantity: 2, Subtotal: 200}},
					SubtotalAmount:   200,
					FinalTotalAmount: 230,
					ShippingFee:      30,
				}, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			CreateFunc: func(ctx context.Context, params sqlc.CreateOrderParams) (sqlc.Order, error) {
				return sqlc.Order{OrderID: 1001, OrderUserID: baseUser.UserID}, nil
			},
		}
		mockOrderItemRepo := &testutils.MockOrderItemRepository{
			CreateFunc: func(ctx context.Context, tx pgx.Tx, input sqlc.CreateOrderItemParams) (sqlc.OrderItem, error) {
				return sqlc.OrderItem{OrderItemID: 1}, nil
			},
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				return []sqlc.GetOrderItemsByOrderIDRow{{OrderItemID: 1, OrderItemProductID: 10}}, nil
			},
		}
		mockTrackingRepo := &testutils.MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				return sqlc.OrderTrackingEvent{EventID: 1}, nil
			},
		}
		mockCartRepo := &testutils.MockCartRepository{
			GetCartByUserIDFunc: func(ctx context.Context, userID int32) (sqlc.Cart, error) {
				return sqlc.Cart{}, pgx.ErrNoRows // Cart not found
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			mockPricing,
			mockInventory,
			mockOrderRepo,
			mockOrderItemRepo,
			nil,
			&testutils.MockProductRepository{},
			mockUserRepo,
			&testutils.MockCartService{},
			mockCartRepo,
			&testutils.MockCouponRepository{},
			mockTrackingRepo,
			nil,
			nil,
			nil,
			&testutils.MockNotificationService{},
			nil,
			nil,
			nil,
		)

		order, items, err := svc.CreateOrder(ctx, userUUID, baseInput)
		require.NoError(t, err)
		assert.Equal(t, int32(1001), order.OrderID)
		assert.Len(t, items, 1)
	})
}

func TestOrderService_CancelOrder(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	adminUUID := uuid.New()
	orderUUID := uuid.New()

	baseUser := sqlc.User{
		UserID:    1,
		UserUuid:  userUUID,
		UserEmail: "customer@example.com",
		UserLevel: v1service.RoleMember,
	}

	adminUser := sqlc.User{
		UserID:    99,
		UserUuid:  adminUUID,
		UserEmail: "admin@example.com",
		UserLevel: v1service.RoleAdmin,
	}

	baseOrderRow := sqlc.GetOrderByUUIDRow{
		OrderID:          500,
		OrderUuid:        orderUUID,
		OrderNumber:      "260914-01-AB12CD",
		OrderUserID:      baseUser.UserID,
		OrderStatus:      v1service.OrderStatusPending,
		OrderTotalAmount: utils.FloatToNumeric(500000),
	}

	baseOrderItems := []sqlc.GetOrderItemsByOrderIDRow{
		{
			OrderItemID:        1,
			OrderItemProductID: 101,
			OrderItemQuantity:  2,
		},
		{
			OrderItemID:        2,
			OrderItemProductID: 102,
			OrderItemQuantity:  1,
		},
	}

	t.Run("Case 1: User Not Found - Returns 404 NotFound", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			nil, nil, nil, nil, nil, nil,
			mockUserRepo,
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		)

		_, err := svc.CancelOrder(ctx, userUUID, orderUUID, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "User not found")
	})

	t.Run("Case 2: Order Not Found - Returns 404 NotFound", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			nil, nil,
			mockOrderRepo,
			nil, nil, nil,
			mockUserRepo,
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		)

		_, err := svc.CancelOrder(ctx, userUUID, orderUUID, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Order not found")
	})

	t.Run("Case 3: Forbidden - Other Member Cannot Cancel Someone Else's Order", func(t *testing.T) {
		otherUserUUID := uuid.New()
		otherUser := sqlc.User{
			UserID:    2,
			UserUuid:  otherUserUUID,
			UserLevel: v1service.RoleMember,
		}

		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return otherUser, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return baseOrderRow, nil // Owned by userID 1
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			nil, nil,
			mockOrderRepo,
			nil, nil, nil,
			mockUserRepo,
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		)

		_, err := svc.CancelOrder(ctx, otherUserUUID, orderUUID, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "You are not authorized to cancel this order")
	})

	t.Run("Case 4: Invalid Status - Shipped Order Cannot Be Cancelled", func(t *testing.T) {
		shippedOrder := baseOrderRow
		shippedOrder.OrderStatus = v1service.OrderStatusShipped

		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return shippedOrder, nil
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			nil, nil,
			mockOrderRepo,
			nil, nil, nil,
			mockUserRepo,
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		)

		_, err := svc.CancelOrder(ctx, userUUID, orderUUID, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Only pending or confirmed orders can be cancelled")
	})

	t.Run("Case 5: Invalid Status - Delivered Order Cannot Be Cancelled", func(t *testing.T) {
		deliveredOrder := baseOrderRow
		deliveredOrder.OrderStatus = v1service.OrderStatusDelivered

		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return deliveredOrder, nil
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			nil, nil,
			mockOrderRepo,
			nil, nil, nil,
			mockUserRepo,
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		)

		_, err := svc.CancelOrder(ctx, userUUID, orderUUID, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Only pending or confirmed orders can be cancelled")
	})

	t.Run("Case 6: OrderItemRepo GetByOrderID Failure - Returns Error", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return baseOrderRow, nil
			},
		}
		mockOrderItemRepo := &testutils.MockOrderItemRepository{
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				return nil, errors.New("db query timeout")
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			nil, nil,
			mockOrderRepo,
			mockOrderItemRepo,
			nil, nil,
			mockUserRepo,
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		)

		_, err := svc.CancelOrder(ctx, userUUID, orderUUID, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get order items")
	})

	t.Run("Case 7: OrderRepo CancelOrder Failure - Transaction Aborts", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return baseOrderRow, nil
			},
			CancelOrderFunc: func(ctx context.Context, tx pgx.Tx, u uuid.UUID) (sqlc.Order, error) {
				return sqlc.Order{}, errors.New("update deadlock")
			},
		}
		mockOrderItemRepo := &testutils.MockOrderItemRepository{
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				return baseOrderItems, nil
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			nil, nil,
			mockOrderRepo,
			mockOrderItemRepo,
			nil, nil,
			mockUserRepo,
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		)

		_, err := svc.CancelOrder(ctx, userUUID, orderUUID, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to cancel order")
	})

	t.Run("Case 8: InventoryService RestoreStock Failure - Transaction Aborts", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return baseOrderRow, nil
			},
			CancelOrderFunc: func(ctx context.Context, tx pgx.Tx, u uuid.UUID) (sqlc.Order, error) {
				return sqlc.Order{OrderID: baseOrderRow.OrderID, OrderStatus: v1service.OrderStatusCancelled}, nil
			},
		}
		mockOrderItemRepo := &testutils.MockOrderItemRepository{
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				return baseOrderItems, nil
			},
		}
		mockInventory := &testutils.MockInventoryService{
			RestoreStockFunc: func(ctx context.Context, items []v1service.RestoreItem) error {
				return errors.New("cannot restore stock for deleted product")
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			nil,
			mockInventory,
			mockOrderRepo,
			mockOrderItemRepo,
			nil, nil,
			mockUserRepo,
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		)

		_, err := svc.CancelOrder(ctx, userUUID, orderUUID, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to restore product stock")
	})

	t.Run("Case 9: TrackingRepo InsertEvent Failure - Transaction Aborts", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return baseOrderRow, nil
			},
			CancelOrderFunc: func(ctx context.Context, tx pgx.Tx, u uuid.UUID) (sqlc.Order, error) {
				return sqlc.Order{OrderID: baseOrderRow.OrderID, OrderStatus: v1service.OrderStatusCancelled}, nil
			},
		}
		mockOrderItemRepo := &testutils.MockOrderItemRepository{
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				return baseOrderItems, nil
			},
		}
		mockInventory := &testutils.MockInventoryService{
			RestoreStockFunc: func(ctx context.Context, items []v1service.RestoreItem) error {
				return nil
			},
		}
		mockTracking := &testutils.MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				return sqlc.OrderTrackingEvent{}, errors.New("tracking insert failed")
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			nil,
			mockInventory,
			mockOrderRepo,
			mockOrderItemRepo,
			nil, nil,
			mockUserRepo,
			nil, nil, nil,
			mockTracking,
			nil, nil, nil, nil, nil, nil, nil,
		)

		_, err := svc.CancelOrder(ctx, userUUID, orderUUID, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to insert tracking event")
	})

	t.Run("Case 10: Happy Path - Owner Cancels Pending Order With Reason", func(t *testing.T) {
		reason := "Bought by mistake"
		var restoredItems []v1service.RestoreItem
		var trackingEvent sqlc.InsertTrackingEventParams
		var notifiedInput v1dto.SendNotificationInput

		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return baseOrderRow, nil
			},
			CancelOrderFunc: func(ctx context.Context, tx pgx.Tx, u uuid.UUID) (sqlc.Order, error) {
				return sqlc.Order{
					OrderID:          baseOrderRow.OrderID,
					OrderUuid:        baseOrderRow.OrderUuid,
					OrderNumber:      baseOrderRow.OrderNumber,
					OrderUserID:      baseOrderRow.OrderUserID,
					OrderStatus:      v1service.OrderStatusCancelled,
					OrderTotalAmount: baseOrderRow.OrderTotalAmount,
				}, nil
			},
		}
		mockOrderItemRepo := &testutils.MockOrderItemRepository{
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				return baseOrderItems, nil
			},
		}
		mockInventory := &testutils.MockInventoryService{
			RestoreStockFunc: func(ctx context.Context, items []v1service.RestoreItem) error {
				restoredItems = items
				return nil
			},
		}
		mockTracking := &testutils.MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				trackingEvent = params
				return sqlc.OrderTrackingEvent{EventID: 1}, nil
			},
		}
		mockNotification := &testutils.MockNotificationService{
			SendFunc: func(ctx context.Context, input v1dto.SendNotificationInput) error {
				notifiedInput = input
				return nil
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			nil,
			mockInventory,
			mockOrderRepo,
			mockOrderItemRepo,
			nil, nil,
			mockUserRepo,
			nil, nil, nil,
			mockTracking,
			nil, nil, nil,
			mockNotification,
			nil, nil, nil,
		)

		cancelledOrder, err := svc.CancelOrder(ctx, userUUID, orderUUID, &reason)
		require.NoError(t, err)

		// Assert Order
		assert.Equal(t, int32(500), cancelledOrder.OrderID)
		assert.Equal(t, v1service.OrderStatusCancelled, cancelledOrder.OrderStatus)

		// Assert Restored Stock
		require.Len(t, restoredItems, 2)
		assert.Equal(t, int32(101), restoredItems[0].ProductID)
		assert.Equal(t, int32(2), restoredItems[0].Quantity)
		assert.Equal(t, int32(102), restoredItems[1].ProductID)
		assert.Equal(t, int32(1), restoredItems[1].Quantity)

		// Assert Tracking Event
		assert.Equal(t, int32(500), trackingEvent.EventOrderID)
		assert.Equal(t, "cancelled", trackingEvent.EventStatus)
		assert.Equal(t, "user", trackingEvent.EventSource)
		require.NotNil(t, trackingEvent.EventNote)
		assert.Equal(t, "Order cancelled. Reason: Bought by mistake", *trackingEvent.EventNote)

		// Assert Notification
		assert.Equal(t, baseUser.UserID, notifiedInput.UserID)
		assert.Equal(t, "order_cancelled", notifiedInput.Type)
		assert.Equal(t, "Order Cancelled", notifiedInput.Title)
		assert.Equal(t, orderUUID.String(), notifiedInput.Data["order_uuid"])
	})

	t.Run("Case 11: Happy Path - Admin Cancels Confirmed Order Without Reason", func(t *testing.T) {
		confirmedOrder := baseOrderRow
		confirmedOrder.OrderStatus = v1service.OrderStatusConfirmed

		var trackingEvent sqlc.InsertTrackingEventParams
		var notifiedInput v1dto.SendNotificationInput

		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return adminUser, nil // Admin with UserID 99
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return confirmedOrder, nil // Owned by UserID 1
			},
			CancelOrderFunc: func(ctx context.Context, tx pgx.Tx, u uuid.UUID) (sqlc.Order, error) {
				return sqlc.Order{
					OrderID:     confirmedOrder.OrderID,
					OrderNumber: confirmedOrder.OrderNumber,
					OrderUserID: confirmedOrder.OrderUserID,
					OrderStatus: v1service.OrderStatusCancelled,
				}, nil
			},
		}
		mockOrderItemRepo := &testutils.MockOrderItemRepository{
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				return baseOrderItems, nil
			},
		}
		mockInventory := &testutils.MockInventoryService{
			RestoreStockFunc: func(ctx context.Context, items []v1service.RestoreItem) error {
				return nil
			},
		}
		mockTracking := &testutils.MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				trackingEvent = params
				return sqlc.OrderTrackingEvent{EventID: 1}, nil
			},
		}
		mockNotification := &testutils.MockNotificationService{
			SendFunc: func(ctx context.Context, input v1dto.SendNotificationInput) error {
				notifiedInput = input
				return nil
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			nil,
			mockInventory,
			mockOrderRepo,
			mockOrderItemRepo,
			nil, nil,
			mockUserRepo,
			nil, nil, nil,
			mockTracking,
			nil, nil, nil,
			mockNotification,
			nil, nil, nil,
		)

		cancelledOrder, err := svc.CancelOrder(ctx, adminUUID, orderUUID, nil)
		require.NoError(t, err)
		assert.Equal(t, v1service.OrderStatusCancelled, cancelledOrder.OrderStatus)

		// Assert Admin source in tracking event
		assert.Equal(t, "admin", trackingEvent.EventSource)
		require.NotNil(t, trackingEvent.EventNote)
		assert.Equal(t, "Order cancelled.", *trackingEvent.EventNote)

		// Notification is sent to customer (UserID 1), NOT the admin (UserID 99)
		assert.Equal(t, baseUser.UserID, notifiedInput.UserID)
	})

	t.Run("Case 12: Notification Failure Does NOT Fail Cancellation", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return baseOrderRow, nil
			},
			CancelOrderFunc: func(ctx context.Context, tx pgx.Tx, u uuid.UUID) (sqlc.Order, error) {
				return sqlc.Order{OrderID: baseOrderRow.OrderID, OrderStatus: v1service.OrderStatusCancelled}, nil
			},
		}
		mockOrderItemRepo := &testutils.MockOrderItemRepository{
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				return baseOrderItems, nil
			},
		}
		mockInventory := &testutils.MockInventoryService{
			RestoreStockFunc: func(ctx context.Context, items []v1service.RestoreItem) error {
				return nil
			},
		}
		mockTracking := &testutils.MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				return sqlc.OrderTrackingEvent{EventID: 1}, nil
			},
		}
		mockNotification := &testutils.MockNotificationService{
			SendFunc: func(ctx context.Context, input v1dto.SendNotificationInput) error {
				return errors.New("notification provider timeout")
			},
		}

		svc := v1service.NewOrderService(
			testutils.NewNoopTxManager(),
			nil,
			mockInventory,
			mockOrderRepo,
			mockOrderItemRepo,
			nil, nil,
			mockUserRepo,
			nil, nil, nil,
			mockTracking,
			nil, nil, nil,
			mockNotification,
			nil, nil, nil,
		)

		cancelledOrder, err := svc.CancelOrder(ctx, userUUID, orderUUID, nil)
		require.NoError(t, err)
		assert.Equal(t, v1service.OrderStatusCancelled, cancelledOrder.OrderStatus)
	})
}

