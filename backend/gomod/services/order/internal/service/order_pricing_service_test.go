package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	orderservice "goshop/services/order/internal/service"
	"goshop/shared/pkg/utils"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderPricingService_CalculateItems(t *testing.T) {
	pricingSvc := orderservice.NewOrderPricingService(nil)

	t.Run("Success - Multiple items with rounding precision", func(t *testing.T) {
		items := []orderservice.ItemPriceInput{
			{ProductID: 1, ProductName: "Product A", Price: 19.99, Quantity: 2},
			{ProductID: 2, ProductName: "Product B", Price: 5.50, Quantity: 3},
		}

		results, subtotal, err := pricingSvc.CalculateItems(items)

		require.NoError(t, err)
		assert.Len(t, results, 2)
		assert.Equal(t, 39.98, results[0].Subtotal)
		assert.Equal(t, 16.50, results[1].Subtotal)
		assert.Equal(t, 56.48, subtotal)
	})

	t.Run("Error - Empty items list", func(t *testing.T) {
		_, _, err := pricingSvc.CalculateItems([]orderservice.ItemPriceInput{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one item")
	})

	t.Run("Error - Invalid quantity <= 0", func(t *testing.T) {
		items := []orderservice.ItemPriceInput{
			{ProductID: 1, ProductName: "Product A", Price: 10.0, Quantity: 0},
		}
		_, _, err := pricingSvc.CalculateItems(items)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid quantity")
	})

	t.Run("Error - Negative price", func(t *testing.T) {
		items := []orderservice.ItemPriceInput{
			{ProductID: 1, ProductName: "Product A", Price: -5.0, Quantity: 1},
		}
		_, _, err := pricingSvc.CalculateItems(items)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid price")
	})
}

func TestOrderPricingService_CalculateOrderTotals(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()

	items := []orderservice.ItemPriceInput{
		{ProductID: 1, ProductName: "Product 1", Price: 50.0, Quantity: 2}, // Subtotal = 100.0
	}

	couponCode := "DISCOUNT20"
	mockCouponPort := &MockOrderCouponPort{
		PreviewCouponFunc: func(ctx context.Context, code string, amount float64, userUUID uuid.UUID) (*orderservice.CouponDiscountResult, error) {
			if code == "DISCOUNT20" {
				return &orderservice.CouponDiscountResult{
					CouponID:         10,
					CouponCode:       "DISCOUNT20",
					DiscountAmount:   20.0,
					FinalTotalAmount: 80.0,
				}, nil
			}
			return nil, utils.NewError(utils.ErrCodeNotFound, "Coupon not found")
		},
	}

	pricingSvc := orderservice.NewOrderPricingService(mockCouponPort)

	t.Run("Success - Full calculation with shipping and valid coupon code", func(t *testing.T) {
		shippingFee := 5.0
		res, err := pricingSvc.CalculateOrderTotals(ctx, items, &couponCode, 1, shippingFee, now)

		require.NoError(t, err)
		assert.Equal(t, 100.0, res.SubtotalAmount)
		assert.Equal(t, 20.0, res.DiscountAmount)
		assert.Equal(t, 5.0, res.ShippingFee)
		assert.Equal(t, 85.0, res.FinalTotalAmount) // 100 - 20 + 5
		assert.NotNil(t, res.AppliedCouponID)
		assert.Equal(t, int32(10), *res.AppliedCouponID)
		assert.Equal(t, "DISCOUNT20", *res.AppliedCouponCode)
	})

	t.Run("Success - No coupon and negative shipping fee clamped to 0", func(t *testing.T) {
		res, err := pricingSvc.CalculateOrderTotals(ctx, items, nil, 1, -10.0, now)

		require.NoError(t, err)
		assert.Equal(t, 100.0, res.SubtotalAmount)
		assert.Equal(t, 0.0, res.DiscountAmount)
		assert.Equal(t, 0.0, res.ShippingFee)
		assert.Equal(t, 100.0, res.FinalTotalAmount)
		assert.Nil(t, res.AppliedCouponID)
		assert.Nil(t, res.AppliedCouponCode)
	})

	t.Run("Error - CalculateItems failure (empty items)", func(t *testing.T) {
		_, err := pricingSvc.CalculateOrderTotals(ctx, []orderservice.ItemPriceInput{}, &couponCode, 1, 5.0, now)
		require.Error(t, err)
	})

	t.Run("Error - Coupon not found", func(t *testing.T) {
		unknownCode := "NOT_FOUND"
		_, err := pricingSvc.CalculateOrderTotals(ctx, items, &unknownCode, 1, 5.0, now)
		require.Error(t, err)
		var appErr *utils.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, utils.ErrCodeNotFound, appErr.Code)
	})

	t.Run("Error - Coupon port is not configured", func(t *testing.T) {
		svc := orderservice.NewOrderPricingService(nil)
		_, err := svc.CalculateOrderTotals(ctx, items, &couponCode, 1, 5.0, now)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Coupon port is not configured")
	})
}
