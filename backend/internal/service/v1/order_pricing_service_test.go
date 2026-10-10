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

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderPricingService_CalculateItems(t *testing.T) {
	pricingSvc := v1service.NewOrderPricingService(nil)

	t.Run("Success - Multiple items with rounding precision", func(t *testing.T) {
		items := []v1service.ItemPriceInput{
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
		_, _, err := pricingSvc.CalculateItems([]v1service.ItemPriceInput{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one item")
	})

	t.Run("Error - Invalid quantity <= 0", func(t *testing.T) {
		items := []v1service.ItemPriceInput{
			{ProductID: 1, ProductName: "Product A", Price: 10.0, Quantity: 0},
		}
		_, _, err := pricingSvc.CalculateItems(items)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid quantity")
	})

	t.Run("Error - Negative price", func(t *testing.T) {
		items := []v1service.ItemPriceInput{
			{ProductID: 1, ProductName: "Product A", Price: -5.0, Quantity: 1},
		}
		_, _, err := pricingSvc.CalculateItems(items)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid price")
	})
}

func TestOrderPricingService_ValidateAndCalculateDiscount(t *testing.T) {
	pricingSvc := v1service.NewOrderPricingService(nil)
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

	t.Run("Nil Coupon - Zero discount", func(t *testing.T) {
		discount, err := pricingSvc.ValidateAndCalculateDiscount(nil, 100.0, now)
		require.NoError(t, err)
		assert.Equal(t, 0.0, discount)
	})

	t.Run("Success - Percentage coupon without cap", func(t *testing.T) {
		coupon := &v1dto.CouponDTO{
			Code:           "SAVE10",
			Type:           v1service.CouponTypePercentage,
			DiscountValue:  10.0, // 10%
			IsActive:       true,
			StartDate:      now.Add(-1 * time.Hour).Format(time.RFC3339),
			EndDate:        now.Add(1 * time.Hour).Format(time.RFC3339),
			MinOrderAmount: 50.0,
		}

		discount, err := pricingSvc.ValidateAndCalculateDiscount(coupon, 200.0, now)
		require.NoError(t, err)
		assert.Equal(t, 20.0, discount)
	})

	t.Run("Success - Percentage coupon with max cap reached", func(t *testing.T) {
		maxDiscount := 25.0
		coupon := &v1dto.CouponDTO{
			Code:              "SAVE50",
			Type:              v1service.CouponTypePercentage,
			DiscountValue:     50.0, // 50%
			MaxDiscountAmount: &maxDiscount,
			IsActive:          true,
			StartDate:         now.Add(-1 * time.Hour).Format(time.RFC3339),
			EndDate:           now.Add(1 * time.Hour).Format(time.RFC3339),
			MinOrderAmount:    0,
		}

		discount, err := pricingSvc.ValidateAndCalculateDiscount(coupon, 100.0, now)
		require.NoError(t, err)
		assert.Equal(t, 25.0, discount) // Capped at $25 instead of $50
	})

	t.Run("Success - Percentage coupon with max cap NOT reached", func(t *testing.T) {
		maxDiscount := 50.0
		coupon := &v1dto.CouponDTO{
			Code:              "SAVE10",
			Type:              v1service.CouponTypePercentage,
			DiscountValue:     10.0, // 10%
			MaxDiscountAmount: &maxDiscount,
			IsActive:          true,
			StartDate:         now.Add(-1 * time.Hour).Format(time.RFC3339),
			EndDate:           now.Add(1 * time.Hour).Format(time.RFC3339),
			MinOrderAmount:    0,
		}

		discount, err := pricingSvc.ValidateAndCalculateDiscount(coupon, 100.0, now)
		require.NoError(t, err)
		assert.Equal(t, 10.0, discount) // $10 < $50 cap
	})

	t.Run("Success - Fixed amount coupon", func(t *testing.T) {
		coupon := &v1dto.CouponDTO{
			Code:           "FIXED15",
			Type:           v1service.CouponTypeFixedAmount,
			DiscountValue:  15.0,
			IsActive:       true,
			StartDate:      now.Add(-1 * time.Hour).Format(time.RFC3339),
			EndDate:        now.Add(1 * time.Hour).Format(time.RFC3339),
			MinOrderAmount: 20.0,
		}

		discount, err := pricingSvc.ValidateAndCalculateDiscount(coupon, 100.0, now)
		require.NoError(t, err)
		assert.Equal(t, 15.0, discount)
	})

	t.Run("Success - Fixed amount capped at subtotal (cannot be negative total)", func(t *testing.T) {
		coupon := &v1dto.CouponDTO{
			Code:           "BIG100",
			Type:           v1service.CouponTypeFixedAmount,
			DiscountValue:  100.0,
			IsActive:       true,
			StartDate:      now.Add(-1 * time.Hour).Format(time.RFC3339),
			EndDate:        now.Add(1 * time.Hour).Format(time.RFC3339),
			MinOrderAmount: 0,
		}

		discount, err := pricingSvc.ValidateAndCalculateDiscount(coupon, 40.0, now)
		require.NoError(t, err)
		assert.Equal(t, 40.0, discount)
	})

	t.Run("Error - Inactive coupon", func(t *testing.T) {
		coupon := &v1dto.CouponDTO{IsActive: false}
		_, err := pricingSvc.ValidateAndCalculateDiscount(coupon, 100.0, now)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not active")
	})

	t.Run("Error - Future coupon (not yet started)", func(t *testing.T) {
		coupon := &v1dto.CouponDTO{
			IsActive:  true,
			StartDate: now.Add(2 * time.Hour).Format(time.RFC3339),
			EndDate:   now.Add(24 * time.Hour).Format(time.RFC3339),
		}
		_, err := pricingSvc.ValidateAndCalculateDiscount(coupon, 100.0, now)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not yet active")
	})

	t.Run("Error - Expired coupon", func(t *testing.T) {
		coupon := &v1dto.CouponDTO{
			IsActive:  true,
			StartDate: now.Add(-24 * time.Hour).Format(time.RFC3339),
			EndDate:   now.Add(-1 * time.Hour).Format(time.RFC3339),
		}
		_, err := pricingSvc.ValidateAndCalculateDiscount(coupon, 100.0, now)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expired")
	})

	t.Run("Error - Subtotal below minimum order amount", func(t *testing.T) {
		coupon := &v1dto.CouponDTO{
			Code:           "MIN100",
			IsActive:       true,
			StartDate:      now.Add(-1 * time.Hour).Format(time.RFC3339),
			EndDate:        now.Add(1 * time.Hour).Format(time.RFC3339),
			MinOrderAmount: 100.0,
		}
		_, err := pricingSvc.ValidateAndCalculateDiscount(coupon, 50.0, now)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least")
	})

	t.Run("Error - Invalid coupon type", func(t *testing.T) {
		coupon := &v1dto.CouponDTO{
			Code:           "UNKNOWN",
			Type:           999, // Unknown type
			IsActive:       true,
			StartDate:      now.Add(-1 * time.Hour).Format(time.RFC3339),
			EndDate:        now.Add(1 * time.Hour).Format(time.RFC3339),
			MinOrderAmount: 0,
		}
		_, err := pricingSvc.ValidateAndCalculateDiscount(coupon, 50.0, now)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid coupon type")
	})
}

func TestOrderPricingService_CalculateOrderTotals(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()

	items := []v1service.ItemPriceInput{
		{ProductID: 1, ProductName: "Product 1", Price: 50.0, Quantity: 2}, // Subtotal = 100.0
	}

	couponCode := "DISCOUNT20"
	mockCoupon := sqlc.Coupon{
		CouponID:             10,
		CouponCode:           couponCode,
		CouponType:           v1service.CouponTypeFixedAmount,
		CouponDiscountValue:  utils.FloatToNumeric(20.0),
		CouponIsActive:       true,
		CouponStartDate:      now.Add(-1 * time.Hour),
		CouponEndDate:        now.Add(1 * time.Hour),
		CouponMinOrderAmount: utils.FloatToNumeric(50.0),
		CouponUserUsageLimit: 1,
	}

	mockCouponRepo := &testutils.MockCouponRepository{
		GetByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
			if code == "DISCOUNT20" {
				return mockCoupon, nil
			}
			return sqlc.Coupon{}, pgx.ErrNoRows
		},
	}

	pricingSvc := v1service.NewOrderPricingService(mockCouponRepo)

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
		_, err := pricingSvc.CalculateOrderTotals(ctx, []v1service.ItemPriceInput{}, &couponCode, 1, 5.0, now)
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

	t.Run("Error - ValidateAndCalculateDiscount failure (expired coupon)", func(t *testing.T) {
		expiredCode := "EXPIRED"
		expiredMockRepo := &testutils.MockCouponRepository{
			GetByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return sqlc.Coupon{
					CouponIsActive:  true,
					CouponStartDate: now.Add(-24 * time.Hour),
					CouponEndDate:   now.Add(-1 * time.Hour),
				}, nil
			},
		}
		svc := v1service.NewOrderPricingService(expiredMockRepo)
		_, err := svc.CalculateOrderTotals(ctx, items, &expiredCode, 1, 5.0, now)
		require.Error(t, err)
	})

	t.Run("Error - User usage limit reached", func(t *testing.T) {
		limitRepo := &testutils.MockCouponRepository{
			GetByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return mockCoupon, nil
			},
			CountUserUsagesFunc: func(ctx context.Context, userID, couponID int32) (int64, error) {
				return 1, nil // Already used once, limit is 1
			},
		}
		svc := v1service.NewOrderPricingService(limitRepo)
		_, err := svc.CalculateOrderTotals(ctx, items, &couponCode, 1, 5.0, now)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "You have reached your usage limit")
	})

	t.Run("Error - Coupon total usage limit reached", func(t *testing.T) {
		limit := int32(5)
		maxedCoupon := mockCoupon
		maxedCoupon.CouponUsageLimit = &limit
		maxedCoupon.CouponUsedCount = 5
		limitRepo := &testutils.MockCouponRepository{
			GetByCodeFunc: func(ctx context.Context, code string) (sqlc.Coupon, error) {
				return maxedCoupon, nil
			},
		}
		svc := v1service.NewOrderPricingService(limitRepo)
		_, err := svc.CalculateOrderTotals(ctx, items, &couponCode, 1, 5.0, now)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "maximum total usage limit")
	})
}
