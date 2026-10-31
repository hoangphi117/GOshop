package contract

import (
	"context"
	"net"
	"testing"

	couponv1 "goshop/shared/proto/coupon/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type mockCouponServer struct {
	couponv1.UnimplementedCouponServiceServer
}

func (s *mockCouponServer) PreviewCoupon(ctx context.Context, req *couponv1.PreviewCouponRequest) (*couponv1.PreviewCouponResponse, error) {
	discount := req.GetOrderSubtotal() * 0.1
	return &couponv1.PreviewCouponResponse{
		CouponId:         7,
		CouponUuid:       "coupon-uuid-7",
		CouponCode:       req.GetCouponCode(),
		CouponName:       "Ten percent off",
		CouponType:       1,
		DiscountValue:    10,
		SubtotalAmount:   req.GetOrderSubtotal(),
		DiscountAmount:   discount,
		FinalTotalAmount: req.GetOrderSubtotal() - discount,
		MinOrderAmount:   100,
	}, nil
}

func (s *mockCouponServer) ReserveCoupon(ctx context.Context, req *couponv1.ReserveCouponRequest) (*couponv1.ReserveCouponResponse, error) {
	return &couponv1.ReserveCouponResponse{Success: true, UsageUuid: "usage-uuid-1"}, nil
}

func (s *mockCouponServer) ReleaseCoupon(ctx context.Context, req *couponv1.ReleaseCouponRequest) (*couponv1.ReleaseCouponResponse, error) {
	return &couponv1.ReleaseCouponResponse{Success: true}, nil
}

func newTestCouponClient(t *testing.T) *GRPCCouponClient {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	couponv1.RegisterCouponServiceServer(grpcServer, &mockCouponServer{})
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(grpcServer.Stop)

	client, err := NewGRPCCouponClient(listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestGRPCCouponClient_PreviewCoupon(t *testing.T) {
	client := newTestCouponClient(t)

	preview, err := client.PreviewCoupon(context.Background(), "SAVE10", 200, "user-uuid-1")
	require.NoError(t, err)
	require.NotNil(t, preview)

	assert.Equal(t, int32(7), preview.CouponID)
	assert.Equal(t, "coupon-uuid-7", preview.CouponUUID)
	assert.Equal(t, "SAVE10", preview.CouponCode)
	assert.Equal(t, "Ten percent off", preview.CouponName)
	assert.Equal(t, int32(1), preview.CouponType)
	assert.InDelta(t, 10, preview.DiscountValue, 1e-9)
	assert.InDelta(t, 200, preview.SubtotalAmount, 1e-9)
	assert.InDelta(t, 20, preview.DiscountAmount, 1e-9)
	assert.InDelta(t, 180, preview.FinalTotalAmount, 1e-9)
	assert.InDelta(t, 100, preview.MinOrderAmount, 1e-9)
}

func TestGRPCCouponClient_ReserveCoupon(t *testing.T) {
	client := newTestCouponClient(t)

	usageUUID, err := client.ReserveCoupon(context.Background(), ReserveCouponInput{
		CouponID:   7,
		CouponUUID: "coupon-uuid-7",
		UserID:     42,
		UserUUID:   "user-uuid-1",
		OrderID:    99,
		OrderUUID:  "order-uuid-1",
	})
	require.NoError(t, err)
	assert.Equal(t, "usage-uuid-1", usageUUID)
}

func TestGRPCCouponClient_ReleaseCoupon(t *testing.T) {
	client := newTestCouponClient(t)

	require.NoError(t, client.ReleaseCoupon(context.Background(), ReleaseCouponInput{
		CouponID:   7,
		CouponUUID: "coupon-uuid-7",
		OrderID:    99,
		OrderUUID:  "order-uuid-1",
	}))
}
