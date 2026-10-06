package payment

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sony/gobreaker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPaymentProvider struct {
	createPaymentURLFunc       func(ctx context.Context, req CreatePaymentRequest) (CreatePaymentResponse, error)
	verifyWebhookFunc          func(payload []byte, headers map[string]string) error
	parseWebhookPayloadFunc    func(payload []byte) (WebhookResult, error)
	queryTransactionStatusFunc func(ctx context.Context, providerTxnID string) (TransactionStatus, error)
	refundFunc                 func(ctx context.Context, req RefundRequest) (RefundResponse, error)
}

func (m *mockPaymentProvider) CreatePaymentURL(ctx context.Context, req CreatePaymentRequest) (CreatePaymentResponse, error) {
	if m.createPaymentURLFunc != nil {
		return m.createPaymentURLFunc(ctx, req)
	}
	return CreatePaymentResponse{PaymentURL: "https://mock.payment.com/pay"}, nil
}

func (m *mockPaymentProvider) VerifyWebhook(payload []byte, headers map[string]string) error {
	if m.verifyWebhookFunc != nil {
		return m.verifyWebhookFunc(payload, headers)
	}
	return nil
}

func (m *mockPaymentProvider) ParseWebhookPayload(payload []byte) (WebhookResult, error) {
	if m.parseWebhookPayloadFunc != nil {
		return m.parseWebhookPayloadFunc(payload)
	}
	return WebhookResult{Status: TxnStatusSuccess}, nil
}

func (m *mockPaymentProvider) QueryTransactionStatus(ctx context.Context, providerTxnID string) (TransactionStatus, error) {
	if m.queryTransactionStatusFunc != nil {
		return m.queryTransactionStatusFunc(ctx, providerTxnID)
	}
	return TxnStatusSuccess, nil
}

func (m *mockPaymentProvider) Refund(ctx context.Context, req RefundRequest) (RefundResponse, error) {
	if m.refundFunc != nil {
		return m.refundFunc(ctx, req)
	}
	return RefundResponse{RefundTxnID: "refund_123", Status: TxnStatusSuccess}, nil
}

func TestCircuitBreakerPaymentProvider_NormalPassThrough(t *testing.T) {
	var callCount int32
	mock := &mockPaymentProvider{
		createPaymentURLFunc: func(ctx context.Context, req CreatePaymentRequest) (CreatePaymentResponse, error) {
			atomic.AddInt32(&callCount, 1)
			return CreatePaymentResponse{PaymentURL: "https://pay.momo.vn/123"}, nil
		},
	}

	cfg := CBConfig{
		MaxRequests: 1,
		Interval:    0,
		Timeout:     100 * time.Millisecond,
		MaxFailures: 3,
	}

	cbProvider := NewCircuitBreakerPaymentProvider(mock, "momo", cfg)

	// Execute successful calls
	for i := 0; i < 5; i++ {
		resp, err := cbProvider.CreatePaymentURL(context.Background(), CreatePaymentRequest{})
		require.NoError(t, err)
		assert.Equal(t, "https://pay.momo.vn/123", resp.PaymentURL)
	}

	assert.Equal(t, int32(5), atomic.LoadInt32(&callCount))

	reporter, ok := cbProvider.(CircuitBreakerReporter)
	require.True(t, ok)
	assert.Equal(t, "closed", reporter.CBState())
	assert.Equal(t, "payment:momo", reporter.CBName())
}

func TestCircuitBreakerPaymentProvider_TripsToOpenState(t *testing.T) {
	var callCount int32
	mockErr := errors.New("gateway network timeout")

	mock := &mockPaymentProvider{
		createPaymentURLFunc: func(ctx context.Context, req CreatePaymentRequest) (CreatePaymentResponse, error) {
			atomic.AddInt32(&callCount, 1)
			return CreatePaymentResponse{}, mockErr
		},
	}

	cfg := CBConfig{
		MaxRequests: 1,
		Interval:    0,
		Timeout:     200 * time.Millisecond,
		MaxFailures: 3,
	}

	cbProvider := NewCircuitBreakerPaymentProvider(mock, "momo", cfg)

	// Trigger 3 failures -> trips to OPEN
	for i := 0; i < 3; i++ {
		_, err := cbProvider.CreatePaymentURL(context.Background(), CreatePaymentRequest{})
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockErr))
	}
	assert.Equal(t, int32(3), atomic.LoadInt32(&callCount))

	reporter, ok := cbProvider.(CircuitBreakerReporter)
	require.True(t, ok)
	assert.Equal(t, "open", reporter.CBState())

	// 4th call should FAST-FAIL immediately with ErrOpenState without calling inner mock
	_, err := cbProvider.CreatePaymentURL(context.Background(), CreatePaymentRequest{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, gobreaker.ErrOpenState))
	assert.Equal(t, int32(3), atomic.LoadInt32(&callCount), "Inner provider should NOT be called when breaker is OPEN")
}

func TestCircuitBreakerPaymentProvider_HalfOpenRecovery(t *testing.T) {
	var shouldFail int32 = 1
	var callCount int32

	mock := &mockPaymentProvider{
		createPaymentURLFunc: func(ctx context.Context, req CreatePaymentRequest) (CreatePaymentResponse, error) {
			atomic.AddInt32(&callCount, 1)
			if atomic.LoadInt32(&shouldFail) == 1 {
				return CreatePaymentResponse{}, errors.New("gateway unavailable")
			}
			return CreatePaymentResponse{PaymentURL: "https://pay.momo.vn/recovered"}, nil
		},
	}

	cfg := CBConfig{
		MaxRequests: 1,
		Interval:    0,
		Timeout:     50 * time.Millisecond,
		MaxFailures: 2,
	}

	cbProvider := NewCircuitBreakerPaymentProvider(mock, "momo", cfg)

	// Trip to OPEN
	for i := 0; i < 2; i++ {
		_, _ = cbProvider.CreatePaymentURL(context.Background(), CreatePaymentRequest{})
	}

	reporter := cbProvider.(CircuitBreakerReporter)
	assert.Equal(t, "open", reporter.CBState())

	// Wait for Timeout duration to elapse -> switches to HALF-OPEN
	time.Sleep(70 * time.Millisecond)

	// Heal the mock service
	atomic.StoreInt32(&shouldFail, 0)

	// Probe call succeeds
	resp, err := cbProvider.CreatePaymentURL(context.Background(), CreatePaymentRequest{})
	require.NoError(t, err)
	assert.Equal(t, "https://pay.momo.vn/recovered", resp.PaymentURL)

	// Circuit Breaker should be back to CLOSED
	assert.Equal(t, "closed", reporter.CBState())
}

func TestCircuitBreakerPaymentProvider_OtherMethodsProtected(t *testing.T) {
	var refundCalls int32
	var queryCalls int32

	mock := &mockPaymentProvider{
		refundFunc: func(ctx context.Context, req RefundRequest) (RefundResponse, error) {
			atomic.AddInt32(&refundCalls, 1)
			return RefundResponse{}, errors.New("refund service down")
		},
		queryTransactionStatusFunc: func(ctx context.Context, providerTxnID string) (TransactionStatus, error) {
			atomic.AddInt32(&queryCalls, 1)
			return "", errors.New("query status down")
		},
	}

	cfg := CBConfig{
		MaxRequests: 1,
		Interval:    0,
		Timeout:     100 * time.Millisecond,
		MaxFailures: 2,
	}

	cbProvider := NewCircuitBreakerPaymentProvider(mock, "momo", cfg)

	// 2 refund failures trip the breaker
	_, err := cbProvider.Refund(context.Background(), RefundRequest{})
	require.Error(t, err)
	_, err = cbProvider.Refund(context.Background(), RefundRequest{})
	require.Error(t, err)

	reporter := cbProvider.(CircuitBreakerReporter)
	assert.Equal(t, "open", reporter.CBState())

	// QueryTransactionStatus should fast fail because breaker is OPEN
	_, err = cbProvider.QueryTransactionStatus(context.Background(), "txn_123")
	require.Error(t, err)
	assert.True(t, errors.Is(err, gobreaker.ErrOpenState))
	assert.Equal(t, int32(0), atomic.LoadInt32(&queryCalls))

	// Webhook verification should still work locally without tripping/blocking
	err = cbProvider.VerifyWebhook([]byte(`{}`), map[string]string{})
	require.NoError(t, err)

	res, err := cbProvider.ParseWebhookPayload([]byte(`{}`))
	require.NoError(t, err)
	assert.Equal(t, TxnStatusSuccess, res.Status)
}
