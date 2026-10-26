package service

import (
	"context"
	"fmt"
	"goshop/pkg/utils"
	"strings"
	"time"

	"github.com/google/uuid"
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
	couponPort OrderCouponPort
}

// NewOrderPricingService creates an OrderPricingService backed by OrderCouponPort.
func NewOrderPricingService(couponPort OrderCouponPort) OrderPricingService {
	return &orderPricingService{
		couponPort: couponPort,
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
		cleanCode := strings.ToUpper(strings.TrimSpace(*couponCode))

		if s.couponPort != nil {
			var uUUID uuid.UUID
			if cust, ok := GetOrderCustomerFromContext(ctx); ok {
				uUUID = cust.UserUUID
			}
			preview, err := s.couponPort.PreviewCoupon(ctx, cleanCode, subtotal, uUUID)
			if err != nil {
				return nil, err
			}
			if preview.DiscountAmount > 0 {
				discount = preview.DiscountAmount
				appliedCouponID = &preview.CouponID
				appliedCouponCode = &preview.CouponCode
			}
		} else {
			return nil, utils.NewError(utils.ErrCodeInternal, "Coupon port is not configured")
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
