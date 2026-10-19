package v1service

import (
	"context"
	"goshop/internal/utils"
	couponcontract "goshop/services/coupon/contract"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.InvalidArgument, codes.NotFound:
				return nil, utils.NewError(utils.ErrCodeBadRequest, st.Message())
			}
		}
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
	orderUUID *uuid.UUID,
) error {
	var orderUUIDStr string
	if orderUUID != nil && *orderUUID != uuid.Nil {
		orderUUIDStr = orderUUID.String()
	}
	_, err := a.client.ReserveCoupon(ctx, couponcontract.ReserveCouponInput{
		CouponID:  couponID,
		UserID:    userID,
		OrderID:   orderID,
		OrderUUID: orderUUIDStr,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.InvalidArgument, codes.NotFound, codes.FailedPrecondition:
				return utils.NewError(utils.ErrCodeBadRequest, st.Message())
			}
		}
		return err
	}
	return nil
}

func (a *orderCouponGRPCAdapter) ReleaseCoupon(
	ctx context.Context,
	couponID int32,
	orderID int32,
	orderUUID *uuid.UUID,
) error {
	var orderUUIDStr string
	if orderUUID != nil && *orderUUID != uuid.Nil {
		orderUUIDStr = orderUUID.String()
	}
	return a.client.ReleaseCoupon(ctx, couponcontract.ReleaseCouponInput{
		CouponID:  couponID,
		OrderID:   orderID,
		OrderUUID: orderUUIDStr,
	})
}
