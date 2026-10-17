package v1service

import (
	"context"
	couponcontract "goshop/services/coupon/contract"

	"github.com/google/uuid"
)

type orderCouponGRPCAdapter struct {
	client couponcontract.CouponClient
}

// NewOrderCouponGRPCAdapter bridges gRPC CouponClient to OrderCouponPort.
func NewOrderCouponGRPCAdapter(client couponcontract.CouponClient) OrderCouponPort {
	return &orderCouponGRPCAdapter{client: client}
}

func (a *orderCouponGRPCAdapter) PreviewCoupon(
	ctx context.Context,
	code string,
	amount float64,
	userUUID uuid.UUID,
) (*CouponDiscountResult, error) {
	var uStr string
	if userUUID != uuid.Nil {
		uStr = userUUID.String()
	}
	preview, err := a.client.PreviewCoupon(ctx, code, amount, uStr)
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

func (a *orderCouponGRPCAdapter) LockAndApplyCoupon(
	ctx context.Context,
	couponID int32,
	userID int32,
	orderID int32,
) error {
	_, err := a.client.ReserveCoupon(ctx, couponcontract.ReserveCouponInput{
		CouponID: couponID,
		UserID:   userID,
		OrderID:  orderID,
	})
	return err
}
