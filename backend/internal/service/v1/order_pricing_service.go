package v1service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/utils"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type ItemPriceInput struct {
	ProductID   int32
	ProductName string
	Price       float64
	Quantity    int32
}

type ItemPriceResult struct {
	ProductID   int32
	ProductName string
	Price       float64
	Quantity    int32
	Subtotal    float64
}

type OrderPricingResult struct {
	SubtotalAmount    float64
	DiscountAmount    float64
	ShippingFee       float64
	FinalTotalAmount  float64
	AppliedCouponID   *int32
	AppliedCouponCode *string
	Items             []ItemPriceResult
}

// CouponLookup is a narrow read port consumed only by orderPricingService.
// Satisfied by any implementation providing code lookup and usage counts (e.g. CouponRepository).
type CouponLookup interface {
	GetByCode(ctx context.Context, code string) (sqlc.Coupon, error)
	CountUserUsages(ctx context.Context, userID, couponID int32) (int64, error)
}

type orderPricingService struct {
	couponLookup CouponLookup
}

func NewOrderPricingService(couponLookup CouponLookup) OrderPricingService {
	return &orderPricingService{
		couponLookup: couponLookup,
	}
}

// CalculateItems calculates the subtotal for each item and the total amount for the order.
func (s *orderPricingService) CalculateItems(items []ItemPriceInput) ([]ItemPriceResult, float64, error) {
	if len(items) == 0 {
		return nil, 0, utils.NewError(utils.ErrCodeBadRequest, "Order must contain at least one item")
	}

	results := make([]ItemPriceResult, 0, len(items))
	var totalSubtotal float64

	for _, item := range items {
		if item.Quantity <= 0 {
			return nil, 0, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Invalid quantity for product %s", item.ProductName))
		}
		if item.Price < 0 {
			return nil, 0, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Invalid price for product %s", item.ProductName))
		}

		itemSubtotal := utils.RoundFloat(float64(item.Quantity)*item.Price, 2)
		totalSubtotal = utils.RoundFloat(totalSubtotal+itemSubtotal, 2)

		results = append(results, ItemPriceResult{
			ProductID:   item.ProductID,
			ProductName: item.ProductName,
			Price:       item.Price,
			Quantity:    item.Quantity,
			Subtotal:    itemSubtotal,
		})
	}
	return results, totalSubtotal, nil
}

func parseCouponTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02 15:04:05", s)
}

func (s *orderPricingService) ValidateAndCalculateDiscount(coupon *v1dto.CouponDTO, subtotal float64, now time.Time) (float64, error) {
	if coupon == nil {
		return 0, nil
	}

	if !coupon.IsActive {
		return 0, utils.NewError(utils.ErrCodeBadRequest, "Coupon is not active")
	}

	if coupon.StartDate != "" {
		if startDate, err := parseCouponTime(coupon.StartDate); err == nil {
			if now.Before(startDate) {
				return 0, utils.NewError(utils.ErrCodeBadRequest, "Coupon is not yet active")
			}
		}
	}

	if coupon.EndDate != "" {
		if endDate, err := parseCouponTime(coupon.EndDate); err == nil {
			if now.After(endDate) {
				return 0, utils.NewError(utils.ErrCodeBadRequest, "Coupon has expired")
			}
		}
	}

	if coupon.UsageLimit != nil && coupon.UsedCount >= *coupon.UsageLimit {
		return 0, utils.NewError(utils.ErrCodeBadRequest, "Coupon has reached its maximum total usage limit")
	}

	if subtotal < coupon.MinOrderAmount {
		return 0, utils.NewError(utils.ErrCodeBadRequest,
			fmt.Sprintf("Order subtotal must be at least %.2f to use this coupon", coupon.MinOrderAmount))
	}

	var discount float64
	switch coupon.Type {
	case CouponTypePercentage:
		discount = utils.RoundFloat(subtotal*coupon.DiscountValue/100, 2)
		if coupon.MaxDiscountAmount != nil && *coupon.MaxDiscountAmount > 0 {
			if discount > *coupon.MaxDiscountAmount {
				discount = *coupon.MaxDiscountAmount
			}
		}
	case CouponTypeFixedAmount:
		discount = coupon.DiscountValue
		if discount > subtotal {
			discount = subtotal
		}
	default:
		return 0, utils.NewError(utils.ErrCodeBadRequest, "Invalid coupon type")
	}

	return discount, nil
}

func (s *orderPricingService) CalculateOrderTotals(
	ctx context.Context,
	items []ItemPriceInput,
	couponCode *string,
	userID int32,
	shippingFee float64,
	now time.Time,
) (*OrderPricingResult, error) {

	itemResults, subtotal, err := s.CalculateItems(items)
	if err != nil {
		return nil, err
	}

	var appliedCouponID *int32
	var appliedCouponCode *string
	var discount float64

	if couponCode != nil && strings.TrimSpace(*couponCode) != "" {
		if s.couponLookup == nil {
			return nil, utils.NewError(utils.ErrCodeInternal, "Coupon repository is not configured")
		}
		cleanCode := strings.ToUpper(strings.TrimSpace(*couponCode))
		coupon, err := s.couponLookup.GetByCode(ctx, cleanCode)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
				return nil, utils.NewError(utils.ErrCodeNotFound, "Coupon not found")
			}
			return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to retrieve coupon", err)
		}

		couponDTO := v1dto.MapToCouponDTO(coupon)
		calculatedDiscount, err := s.ValidateAndCalculateDiscount(&couponDTO, subtotal, now)
		if err != nil {
			return nil, err
		}

		userUsageCount, err := s.couponLookup.CountUserUsages(ctx, userID, coupon.CouponID)
		if err != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to check coupon user usage count", err)
		}

		if userUsageCount >= int64(coupon.CouponUserUsageLimit) {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "You have reached your usage limit for this coupon")
		}

		if calculatedDiscount > 0 {
			discount = calculatedDiscount
			appliedCouponID = &coupon.CouponID
			appliedCouponCode = &coupon.CouponCode
		}
	}

	roundedShippingFee := utils.RoundFloat(shippingFee, 2)
	if roundedShippingFee < 0 {
		roundedShippingFee = 0
	}

	finalTotal := utils.RoundFloat(subtotal-discount+roundedShippingFee, 2)
	if finalTotal < 0 {
		finalTotal = 0
	}

	return &OrderPricingResult{
		SubtotalAmount:    subtotal,
		DiscountAmount:    discount,
		ShippingFee:       roundedShippingFee,
		FinalTotalAmount:  finalTotal,
		AppliedCouponID:   appliedCouponID,
		AppliedCouponCode: appliedCouponCode,
		Items:             itemResults,
	}, nil
}
