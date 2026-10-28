package grpc

import (
	"context"

	couponv1 "goshop/proto/coupon/v1"
	"goshop/services/coupon/internal/dto"
	"goshop/services/coupon/internal/service"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CouponGRPCServer struct {
	couponv1.UnimplementedCouponServiceServer
	couponService service.CouponService
}

func NewCouponGRPCServer(couponService service.CouponService) *CouponGRPCServer {
	return &CouponGRPCServer{
		couponService: couponService,
	}
}

func (s *CouponGRPCServer) PreviewCoupon(ctx context.Context, req *couponv1.PreviewCouponRequest) (*couponv1.PreviewCouponResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	result, err := s.couponService.PreviewCoupon(ctx, dto.PreviewCouponInput{
		Code:           req.CouponCode,
		SubtotalAmount: req.OrderSubtotal,
		UserUUID:       req.UserUuid,
	})
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to preview coupon: %v", err)
	}

	return &couponv1.PreviewCouponResponse{		CouponId:         result.CouponID,
		CouponUuid:       result.CouponUUID,
		CouponCode:       result.CouponCode,
		CouponName:       result.CouponName,
		CouponType:       result.CouponType,
		DiscountValue:    result.DiscountValue,
		SubtotalAmount:   result.SubtotalAmount,
		DiscountAmount:   result.DiscountAmount,
		FinalTotalAmount: result.FinalTotalAmount,
		MinOrderAmount:   result.MinOrderAmount,
	}, nil
}

func (s *CouponGRPCServer) ReserveCoupon(ctx context.Context, req *couponv1.ReserveCouponRequest) (*couponv1.ReserveCouponResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	usageUUID, err := s.couponService.ReserveCoupon(ctx, dto.ReserveCouponInput{
		CouponID:   req.CouponId,
		CouponUUID: req.CouponUuid,
		UserID:     req.UserId,
		UserUUID:   req.UserUuid,
		OrderID:    req.OrderId,
		OrderUUID:  req.OrderUuid,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to reserve coupon: %v", err)
	}

	return &couponv1.ReserveCouponResponse{
		Success:   true,
		UsageUuid: usageUUID,
	}, nil
}

func (s *CouponGRPCServer) ReleaseCoupon(ctx context.Context, req *couponv1.ReleaseCouponRequest) (*couponv1.ReleaseCouponResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	err := s.couponService.ReleaseCoupon(ctx, dto.ReleaseCouponInput{
		CouponID:   req.CouponId,
		CouponUUID: req.CouponUuid,
		OrderID:    req.OrderId,
		OrderUUID:  req.OrderUuid,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to release coupon: %v", err)
	}

	return &couponv1.ReleaseCouponResponse{
		Success: true,
	}, nil
}