package v1service

import (
	"context"
	v1dto "goshop/internal/dto/v1"

	"github.com/google/uuid"
)

type orderCouponAdapter struct {
	couponService CouponService
}

// NewOrderCouponAdapter bridges CouponService to OrderCouponPort.
func NewOrderCouponAdapter(couponService CouponService) OrderCouponPort {
	return &orderCouponAdapter{couponService: couponService}
}

func (a *orderCouponAdapter) PreviewCoupon(
	ctx context.Context,
	code string,
	amount float64,
	userUUID uuid.UUID,
) (*CouponDiscountResult, error) {
	preview, err := a.couponService.PreviewCoupon(ctx, userUUID, v1dto.PreviewCouponInput{
		Code:           code,
		SubtotalAmount: amount,
	})
	if err != nil {
		return nil, err
	}
	return &CouponDiscountResult{
		CouponID:         preview.CouponID,
		CouponCode:       preview.CouponCode,
		DiscountAmount:   preview.DiscountAmount,
		FinalTotalAmount: preview.FinalTotalAmount,
	}, nil
}

func (a *orderCouponAdapter) LockAndApplyCoupon(
	ctx context.Context,
	couponID int32,
	userID int32,
	orderID int32,
) error {
	return a.couponService.LockAndApplyCoupon(ctx, couponID, userID, orderID)
}
