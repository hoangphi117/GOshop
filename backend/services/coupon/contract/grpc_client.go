package contract

import (
	"context"
	"fmt"

	"goshop/pkg/grpcclient"
	couponv1 "goshop/proto/coupon/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type PreviewCouponResult struct {
	CouponID         int32
	CouponUUID       string
	CouponCode       string
	CouponName       string
	CouponType       int32
	DiscountValue    float64
	SubtotalAmount   float64
	DiscountAmount   float64
	FinalTotalAmount float64
	MinOrderAmount   float64
}

type ReserveCouponInput struct {
	CouponID   int32
	CouponUUID string
	UserID     int32
	UserUUID   string
	OrderID    int32
	OrderUUID  string
}

type ReleaseCouponInput struct {
	CouponID   int32
	CouponUUID string
	OrderID    int32
	OrderUUID  string
}

type CouponClient interface {
	PreviewCoupon(ctx context.Context, code string, orderSubtotal float64, userUUID string) (*PreviewCouponResult, error)
	ReserveCoupon(ctx context.Context, input ReserveCouponInput) (string, error)
	ReleaseCoupon(ctx context.Context, input ReleaseCouponInput) error
	Close() error
}

type GRPCCouponClient struct {
	client couponv1.CouponServiceClient
	conn   *grpc.ClientConn
}

func NewGRPCCouponClient(target string) (*GRPCCouponClient, error) {
	conn, err := grpcclient.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc client for coupon at %s: %w", target, err)
	}

	return &GRPCCouponClient{
		client: couponv1.NewCouponServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *GRPCCouponClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *GRPCCouponClient) PreviewCoupon(ctx context.Context, code string, orderSubtotal float64, userUUID string) (*PreviewCouponResult, error) {
	resp, err := c.client.PreviewCoupon(ctx, &couponv1.PreviewCouponRequest{
		CouponCode:    code,
		OrderSubtotal: orderSubtotal,
		UserUuid:      userUUID,
	})
	if err != nil {
		return nil, fmt.Errorf("grpc PreviewCoupon failed: %w", err)
	}

	return &PreviewCouponResult{
		CouponID:         resp.CouponId,
		CouponUUID:       resp.CouponUuid,
		CouponCode:       resp.CouponCode,
		CouponName:       resp.CouponName,
		CouponType:       resp.CouponType,
		DiscountValue:    resp.DiscountValue,
		SubtotalAmount:   resp.SubtotalAmount,
		DiscountAmount:   resp.DiscountAmount,
		FinalTotalAmount: resp.FinalTotalAmount,
		MinOrderAmount:   resp.MinOrderAmount,
	}, nil
}

func (c *GRPCCouponClient) ReserveCoupon(ctx context.Context, input ReserveCouponInput) (string, error) {
	resp, err := c.client.ReserveCoupon(ctx, &couponv1.ReserveCouponRequest{
		CouponId:   input.CouponID,
		CouponUuid: input.CouponUUID,
		UserId:     input.UserID,
		UserUuid:   input.UserUUID,
		OrderId:    input.OrderID,
		OrderUuid:  input.OrderUUID,
	})
	if err != nil {
		return "", fmt.Errorf("grpc ReserveCoupon failed: %w", err)
	}
	return resp.UsageUuid, nil
}

func (c *GRPCCouponClient) ReleaseCoupon(ctx context.Context, input ReleaseCouponInput) error {
	_, err := c.client.ReleaseCoupon(ctx, &couponv1.ReleaseCouponRequest{
		CouponId:   input.CouponID,
		CouponUuid: input.CouponUUID,
		OrderId:    input.OrderID,
		OrderUuid:  input.OrderUUID,
	})
	if err != nil {
		return fmt.Errorf("grpc ReleaseCoupon failed: %w", err)
	}
	return nil
}
