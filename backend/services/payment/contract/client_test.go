package contract

import (
	"context"
	"net"
	"testing"

	paymentv1 "goshop/proto/payment/v1"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type mockPaymentServer struct {
	paymentv1.UnimplementedPaymentServiceServer
}

func (s *mockPaymentServer) CreatePaymentIntent(ctx context.Context, req *paymentv1.CreatePaymentIntentRequest) (*paymentv1.CreatePaymentIntentResponse, error) {
	return &paymentv1.CreatePaymentIntentResponse{
		Intent: &paymentv1.PaymentIntentDTO{
			PaymentUuid: "pay-uuid-1",
			OrderUuid:   req.GetOrderUuid(),
			Provider:    req.GetProvider(),
			Amount:      15_000_000, // 150,000 dong in xu
			Currency:    "VND",
			Status:      "1",
			PaymentUrl:  "https://sandbox.vnpayment.vn/paymentv2/vpcpay.html",
			CreatedAt:   "2026-10-28T00:00:00Z",
		},
	}, nil
}

func (s *mockPaymentServer) GetPaymentTransactionsByOrderID(ctx context.Context, req *paymentv1.GetPaymentTransactionsByOrderIDRequest) (*paymentv1.GetPaymentTransactionsByOrderIDResponse, error) {
	providerTxnID := "15700001"
	refundOf := "ref-uuid-1"
	refundAmount := int64(5_000_000)
	paidAt := "2026-10-28T01:02:03Z"
	return &paymentv1.GetPaymentTransactionsByOrderIDResponse{
		Transactions: []*paymentv1.PaymentTransactionDTO{
			// Full refund mapping: every optional field is set.
			{
				Uuid:          "txn-uuid-1",
				OrderUuid:     testOrderUUID,
				Provider:      "vnpay",
				ProviderTxnId: &providerTxnID,
				Amount:        15_000_000,
				Currency:      "VND",
				Status:        "2",
				RefundOf:      &refundOf,
				RefundAmount:  &refundAmount,
				PaidAt:        &paidAt,
				CreatedAt:     "2026-10-28T00:00:00Z",
			},
			// None of the optional fields: pointers must stay nil.
			{
				Uuid:      "txn-uuid-2",
				OrderUuid: testOrderUUID,
				Provider:  "cod",
				Amount:    1_000_000,
				Currency:  "VND",
				Status:    "1",
				CreatedAt: "2026-10-28T00:00:01Z",
			},
		},
	}, nil
}

func (s *mockPaymentServer) ConfirmCODPayment(ctx context.Context, req *paymentv1.ConfirmCODPaymentRequest) (*paymentv1.ConfirmCODPaymentResponse, error) {
	return &paymentv1.ConfirmCODPaymentResponse{Success: req.GetOrderUuid() == testOrderUUID}, nil
}

func (s *mockPaymentServer) RefundPayment(ctx context.Context, req *paymentv1.RefundPaymentRequest) (*paymentv1.RefundPaymentResponse, error) {
	providerTxnID := "15700002"
	return &paymentv1.RefundPaymentResponse{
		Transaction: &paymentv1.PaymentTransactionDTO{
			Uuid:          "refund-uuid-1",
			OrderUuid:     req.GetOrderUuid(),
			Provider:      "vnpay",
			ProviderTxnId: &providerTxnID,
			Amount:        req.GetAmount(),
			Currency:      "VND",
			Status:        "4",
			RefundAmount:  &req.Amount,
			CreatedAt:     "2026-10-28T02:00:00Z",
		},
	}, nil
}

const testOrderUUID = "5f0c9a3e-8b1d-4c2a-9e6f-7a3b5c1d2e4f"

func newTestPaymentClient(t *testing.T) *GRPCPaymentClient {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	paymentv1.RegisterPaymentServiceServer(grpcServer, &mockPaymentServer{})
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(grpcServer.Stop)

	client, err := NewGRPCPaymentClient(listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestGRPCPaymentClient_CreatePaymentIntent(t *testing.T) {
	client := newTestPaymentClient(t)
	orderUUID := uuid.MustParse(testOrderUUID)

	intent, err := client.CreatePaymentIntent(context.Background(), CreatePaymentIntentInput{
		UserID:         42,
		OrderUUID:      orderUUID,
		Provider:       "vnpay",
		ClientIP:       "127.0.0.1",
		IdempotencyKey: "idem-key-1",
	})
	require.NoError(t, err)

	assert.Equal(t, "pay-uuid-1", intent.PaymentUUID)
	assert.Equal(t, testOrderUUID, intent.OrderUUID, "order uuid must survive the wire roundtrip")
	assert.Equal(t, "vnpay", intent.Provider)
	assert.Equal(t, int64(15_000_000), intent.Amount, "amount must stay integer minor units")
	assert.Equal(t, "VND", intent.Currency)
	assert.Equal(t, "1", intent.Status)
	assert.Equal(t, "https://sandbox.vnpayment.vn/paymentv2/vpcpay.html", intent.PaymentURL)
	assert.Equal(t, "2026-10-28T00:00:00Z", intent.CreatedAt)
}

func TestGRPCPaymentClient_GetPaymentTransactionsByOrderID(t *testing.T) {
	client := newTestPaymentClient(t)

	txns, err := client.GetPaymentTransactionsByOrderID(context.Background(), 99)
	require.NoError(t, err)
	require.Len(t, txns, 2)

	// Optional proto fields map to pointers: set -> non-nil, unset -> nil.
	assert.Equal(t, "txn-uuid-1", txns[0].UUID)
	require.NotNil(t, txns[0].ProviderTxnID)
	assert.Equal(t, "15700001", *txns[0].ProviderTxnID)
	require.NotNil(t, txns[0].RefundOf)
	assert.Equal(t, "ref-uuid-1", *txns[0].RefundOf)
	require.NotNil(t, txns[0].RefundAmount)
	assert.Equal(t, int64(5_000_000), *txns[0].RefundAmount)
	require.NotNil(t, txns[0].PaidAt)
	assert.Equal(t, "2026-10-28T01:02:03Z", *txns[0].PaidAt)

	assert.Nil(t, txns[1].ProviderTxnID)
	assert.Nil(t, txns[1].RefundOf)
	assert.Nil(t, txns[1].RefundAmount)
	assert.Nil(t, txns[1].PaidAt)
}

func TestGRPCPaymentClient_ConfirmCODPayment(t *testing.T) {
	client := newTestPaymentClient(t)
	ctx := context.Background()

	require.NoError(t, client.ConfirmCODPayment(ctx, uuid.MustParse(testOrderUUID)))

	// The mock answers success=false for unknown orders; the client must
	// surface that as an error, not swallow it.
	assert.Error(t, client.ConfirmCODPayment(ctx, uuid.MustParse("11111111-2222-4333-8444-555555555555")))
}

func TestGRPCPaymentClient_RefundPayment(t *testing.T) {
	client := newTestPaymentClient(t)

	txn, err := client.RefundPayment(context.Background(), RefundPaymentInput{
		IdempotencyKey: "refund-key-1",
		OrderUUID:      testOrderUUID,
		Amount:         5_000_000,
		Reason:         "customer return",
	})
	require.NoError(t, err)

	assert.Equal(t, "refund-uuid-1", txn.UUID)
	assert.Equal(t, testOrderUUID, txn.OrderUUID)
	assert.Equal(t, "vnpay", txn.Provider)
	require.NotNil(t, txn.ProviderTxnID)
	assert.Equal(t, "15700002", *txn.ProviderTxnID)
	assert.Equal(t, int64(5_000_000), txn.Amount, "refund amount must arrive as sent")
	require.NotNil(t, txn.RefundAmount)
	assert.Equal(t, int64(5_000_000), *txn.RefundAmount)
	assert.Equal(t, "4", txn.Status)
}
