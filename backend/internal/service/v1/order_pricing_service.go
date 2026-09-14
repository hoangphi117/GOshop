package v1service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"goshop/internal/db/sqlc"
	"goshop/internal/repository"
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

type orderPricingService struct {
	couponRepo repository.CouponRepository
}

func NewOrderPricingService(couponRepo repository.CouponRepository) OrderPricingService {
	return &orderPricingService{
		couponRepo: couponRepo,
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

func (s *orderPricingService) ValidateAndCalculateDiscount(coupon *sqlc.Coupon, subtotal float64, now time.Time) (float64, error) {
	if coupon == nil {
		return 0, nil
	}

	if !coupon.CouponIsActive {
		return 0, utils.NewError(utils.ErrCodeBadRequest, "Coupon is not active")
	}

	if now.Before(coupon.CouponStartDate) {
		return 0, utils.NewError(utils.ErrCodeBadRequest, "Coupon is not yet active")
	}

	if now.After(coupon.CouponEndDate) {
		return 0, utils.NewError(utils.ErrCodeBadRequest, "Coupon has expired")
	}

	minOrderAmount := utils.NumericToFloat(coupon.CouponMinOrderAmount)
	if subtotal < minOrderAmount {
		return 0, utils.NewError(utils.ErrCodeBadRequest,
			fmt.Sprintf("Order subtotal must be at least %.2f to use this coupon", minOrderAmount))
	}

	var discount float64
	switch coupon.CouponType {
	case CouponTypePercentage:
		discount = utils.RoundFloat(subtotal*utils.NumericToFloat(coupon.CouponDiscountValue)/100, 2)
		if coupon.CouponMaxDiscountAmount.Valid {
			maxDiscount := utils.NumericToFloat(coupon.CouponMaxDiscountAmount)
			if discount > maxDiscount {
				discount = maxDiscount
			}
		}
	case CouponTypeFixedAmount:
		discount = utils.NumericToFloat(coupon.CouponDiscountValue)
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
		if s.couponRepo == nil {
			return nil, utils.NewError(utils.ErrCodeInternal, "Coupon repository is not configured")
		}
		cleanCode := strings.ToUpper(strings.TrimSpace(*couponCode))
		coupon, err := s.couponRepo.GetByCode(ctx, cleanCode)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
				return nil, utils.NewError(utils.ErrCodeNotFound, "Coupon not found")
			}
			return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to retrieve coupon", err)
		}

		calculatedDiscount, err := s.ValidateAndCalculateDiscount(&coupon, subtotal, now)
		if err != nil {
			return nil, err
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
