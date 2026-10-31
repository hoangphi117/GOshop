package contract

import (
	"context"
	"net"
	"testing"

	cartv1 "goshop/proto/cart/v1"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type mockCartServer struct {
	cartv1.UnimplementedCartServiceServer
}

func (s *mockCartServer) GetCart(ctx context.Context, req *cartv1.GetCartRequest) (*cartv1.GetCartResponse, error) {
	return &cartv1.GetCartResponse{
		Cart: &cartv1.CartInfo{
			CartUuid:    "cart-uuid-1",
			UserUuid:    req.UserUuid,
			UserId:      req.UserId,
			TotalItems:  1,
			TotalAmount: 100.0,
			Items: []*cartv1.CartItemInfo{
				{
					ItemUuid:       "item-uuid-1",
					ProductUuid:    "prod-uuid-1",
					ProductName:    "Item 1",
					Price:          100.0,
					Quantity:       1,
					Subtotal:       100.0,
					StockAvailable: 10,
					IsAvailable:    true,
				},
			},
		},
	}, nil
}

func (s *mockCartServer) ClearCart(ctx context.Context, req *cartv1.ClearCartRequest) (*cartv1.ClearCartResponse, error) {
	return &cartv1.ClearCartResponse{
		Success: true,
	}, nil
}

func (s *mockCartServer) RemovePurchasedItems(ctx context.Context, req *cartv1.RemovePurchasedItemsRequest) (*cartv1.RemovePurchasedItemsResponse, error) {
	return &cartv1.RemovePurchasedItemsResponse{
		Success: true,
	}, nil
}

func TestGRPCCartClient(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)

	grpcServer := grpc.NewServer()
	cartv1.RegisterCartServiceServer(grpcServer, &mockCartServer{})

	go func() {
		_ = grpcServer.Serve(listener)
	}()
	defer grpcServer.Stop()

	conn, err := grpc.NewClient(
		listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	assert.NoError(t, err)

	client := NewGRPCCartClientWithConn(conn)
	defer func() { _ = client.Close() }()

	ctx := context.Background()

	cart, err := client.GetCart(ctx, "user-uuid-1", 1)
	assert.NoError(t, err)
	assert.NotNil(t, cart)
	assert.Equal(t, "cart-uuid-1", cart.CartUUID)
	assert.Equal(t, int32(1), cart.TotalItems)
	assert.Equal(t, 100.0, cart.TotalAmount)

	cleared, err := client.ClearCart(ctx, "user-uuid-1", 1)
	assert.NoError(t, err)
	assert.True(t, cleared)

	removed, err := client.RemovePurchasedItems(ctx, "user-uuid-1", 1, []string{"prod-uuid-1"}, nil)
	assert.NoError(t, err)
	assert.True(t, removed)
}
