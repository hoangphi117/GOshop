package contract_test

import (
	"context"
	"net"
	"testing"
	"time"

	orderv1 "goshop/proto/order/v1"
	"goshop/services/order/contract"
	ordergrpc "goshop/services/order/internal/grpc"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type mockOrderProvider struct {
	summaryFn func(ctx context.Context, orderUUID uuid.UUID) (*contract.OrderSummary, error)
	verifyFn  func(ctx context.Context, orderUUID uuid.UUID, userID, productID int32, userUUID, productUUID *uuid.UUID) (*contract.PurchaseVerification, error)
	paymentFn func(ctx context.Context, orderUUID uuid.UUID) (*contract.OrderPaymentInfo, error)
}

func (m *mockOrderProvider) GetOrderSummary(ctx context.Context, orderUUID uuid.UUID) (*contract.OrderSummary, error) {
	if m.summaryFn != nil {
		return m.summaryFn(ctx, orderUUID)
	}
	return nil, nil
}

func (m *mockOrderProvider) VerifyPurchase(ctx context.Context, orderUUID uuid.UUID, userID, productID int32, userUUID, productUUID *uuid.UUID) (*contract.PurchaseVerification, error) {
	if m.verifyFn != nil {
		return m.verifyFn(ctx, orderUUID, userID, productID, userUUID, productUUID)
	}
	return nil, nil
}

func (m *mockOrderProvider) GetOrderForPayment(ctx context.Context, orderUUID uuid.UUID) (*contract.OrderPaymentInfo, error) {
	if m.paymentFn != nil {
		return m.paymentFn(ctx, orderUUID)
	}
	return nil, nil
}

func setupTestGRPC(t *testing.T, provider ordergrpc.OrderProvider) (contract.OrderClient, func()) {
	buffer := 1024 * 1024
	lis := bufconn.Listen(buffer)

	srv := grpc.NewServer()
	orderServer := ordergrpc.NewOrderGRPCServer(provider)
	orderv1.RegisterOrderServiceServer(srv, orderServer)

	go func() {
		if err := srv.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			t.Logf("Server error: %v", err)
		}
	}()

	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	client := contract.NewOrderClientFromConn(conn)

	cleanup := func() {
		_ = client.Close()
		srv.GracefulStop()
		_ = lis.Close()
	}

	return client, cleanup
}

func TestOrderClient_GetOrderSummary(t *testing.T) {
	orderUUID := uuid.New()
	userUUID := uuid.New()
	now := time.Now().Truncate(time.Second)

	mockProvider := &mockOrderProvider{
		summaryFn: func(ctx context.Context, u uuid.UUID) (*contract.OrderSummary, error) {
			assert.Equal(t, orderUUID, u)
			return &contract.OrderSummary{
				OrderID:       101,
				OrderUUID:     orderUUID,
				OrderNumber:   "ORD-12345",
				UserID:        12,
				UserUUID:      userUUID,
				CustomerName:  "John Doe",
				CustomerEmail: "john@example.com",
				Status:        4,
				TotalAmount:   250.50,
				DeliveredAt:   &now,
				UpdatedAt:     now,
				Items: []contract.OrderItemSummary{
					{
						ProductUUID: "prod-uuid-1",
						ProductName: "T-Shirt",
						Quantity:    2,
						Price:       50.0,
					},
				},
			}, nil
		},
	}

	client, cleanup := setupTestGRPC(t, mockProvider)
	defer cleanup()

	summary, err := client.GetOrderSummary(context.Background(), orderUUID)
	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, int32(101), summary.OrderID)
	assert.Equal(t, orderUUID, summary.OrderUUID)
	assert.Equal(t, "ORD-12345", summary.OrderNumber)
	assert.Equal(t, 250.50, summary.TotalAmount)
	assert.Len(t, summary.Items, 1)
	assert.Equal(t, "T-Shirt", summary.Items[0].ProductName)
}

func TestOrderClient_VerifyPurchase(t *testing.T) {
	orderUUID := uuid.New()
	now := time.Now().Truncate(time.Second)

	mockProvider := &mockOrderProvider{
		verifyFn: func(ctx context.Context, oUUID uuid.UUID, userID, productID int32, userUUID, productUUID *uuid.UUID) (*contract.PurchaseVerification, error) {
			assert.Equal(t, orderUUID, oUUID)
			assert.Equal(t, int32(12), userID)
			assert.Equal(t, int32(99), productID)
			return &contract.PurchaseVerification{
				OrderID:       101,
				OrderUUID:     orderUUID,
				UserID:        userID,
				IsPurchased:   true,
				CanReview:     true,
				DeliveredAt:   &now,
				CustomerName:  "John Doe",
				CustomerEmail: "john@example.com",
				ShippingName:  "John Shipping",
			}, nil
		},
	}

	client, cleanup := setupTestGRPC(t, mockProvider)
	defer cleanup()

	verification, err := client.VerifyPurchase(context.Background(), orderUUID, 12, 99, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, verification)
	assert.True(t, verification.IsPurchased)
	assert.True(t, verification.CanReview)
	assert.Equal(t, "John Shipping", verification.ShippingName)
}

func TestOrderClient_GetOrderForPayment(t *testing.T) {
	orderUUID := uuid.New()
	userUUID := uuid.New()

	mockProvider := &mockOrderProvider{
		paymentFn: func(ctx context.Context, u uuid.UUID) (*contract.OrderPaymentInfo, error) {
			assert.Equal(t, orderUUID, u)
			return &contract.OrderPaymentInfo{
				OrderUUID:   orderUUID,
				UserID:      12,
				UserUUID:    userUUID,
				TotalAmount: 199.99,
				Status:      1,
			}, nil
		},
	}

	client, cleanup := setupTestGRPC(t, mockProvider)
	defer cleanup()

	info, err := client.GetOrderForPayment(context.Background(), orderUUID)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, orderUUID, info.OrderUUID)
	assert.Equal(t, 199.99, info.TotalAmount)
	assert.Equal(t, int32(1), info.Status)
}
