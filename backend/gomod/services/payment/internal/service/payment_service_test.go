package service_test

import (
	"context"
	"testing"
	"time"

	paymentsqlc "goshop/services/payment/internal/db/sqlc"
	"goshop/services/payment/internal/dto"
	"goshop/services/payment/internal/provider"
	"goshop/services/payment/internal/repository"
	paymentservice "goshop/services/payment/internal/service"
	"goshop/services/payment/money"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockPaymentRepo
type MockPaymentRepo struct {
	mock.Mock
}

func (m *MockPaymentRepo) Create(ctx context.Context, params paymentsqlc.CreatePaymentTransactionParams) (paymentsqlc.PaymentTransaction, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(paymentsqlc.PaymentTransaction), args.Error(1)
}

func (m *MockPaymentRepo) GetByIdempotencyKey(ctx context.Context, key string) (paymentsqlc.PaymentTransaction, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(paymentsqlc.PaymentTransaction), args.Error(1)
}

func (m *MockPaymentRepo) GetByUUID(ctx context.Context, ptUUID uuid.UUID) (paymentsqlc.PaymentTransaction, error) {
	args := m.Called(ctx, ptUUID)
	return args.Get(0).(paymentsqlc.PaymentTransaction), args.Error(1)
}

func (m *MockPaymentRepo) GetByTxnRef(ctx context.Context, provider, txnRef string) (paymentsqlc.PaymentTransaction, error) {
	args := m.Called(ctx, provider, txnRef)
	return args.Get(0).(paymentsqlc.PaymentTransaction), args.Error(1)
}

func (m *MockPaymentRepo) GetByProviderTxnID(ctx context.Context, provider, providerTxnID string) (paymentsqlc.PaymentTransaction, error) {
	args := m.Called(ctx, provider, providerTxnID)
	return args.Get(0).(paymentsqlc.PaymentTransaction), args.Error(1)
}

func (m *MockPaymentRepo) UpdateStatusAtomically(ctx context.Context, params paymentsqlc.UpdatePaymentStatusAtomicallyParams) (paymentsqlc.PaymentTransaction, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(paymentsqlc.PaymentTransaction), args.Error(1)
}

func (m *MockPaymentRepo) GetByOrderID(ctx context.Context, orderID int32) ([]paymentsqlc.PaymentTransaction, error) {
	args := m.Called(ctx, orderID)
	return args.Get(0).([]paymentsqlc.PaymentTransaction), args.Error(1)
}

func (m *MockPaymentRepo) GetByOrderUUID(ctx context.Context, orderUUID uuid.UUID) ([]paymentsqlc.PaymentTransaction, error) {
	args := m.Called(ctx, orderUUID)
	return args.Get(0).([]paymentsqlc.PaymentTransaction), args.Error(1)
}

func (m *MockPaymentRepo) CreateRefund(ctx context.Context, params paymentsqlc.CreateRefundTransactionParams) (paymentsqlc.PaymentTransaction, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(paymentsqlc.PaymentTransaction), args.Error(1)
}

func (m *MockPaymentRepo) GetPendingOlderThan(ctx context.Context, olderThan time.Time, limit int32) ([]paymentsqlc.PaymentTransaction, error) {
	args := m.Called(ctx, olderThan, limit)
	return args.Get(0).([]paymentsqlc.PaymentTransaction), args.Error(1)
}

func (m *MockPaymentRepo) UpdateProviderInfo(ctx context.Context, params paymentsqlc.UpdatePaymentProviderInfoParams) (paymentsqlc.PaymentTransaction, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(paymentsqlc.PaymentTransaction), args.Error(1)
}

// MockOrderReader
type MockOrderReader struct {
	mock.Mock
}

func (m *MockOrderReader) GetByUUID(ctx context.Context, orderUUID uuid.UUID) (*dto.PaymentOrderSummary, error) {
	args := m.Called(ctx, orderUUID)
	if res := args.Get(0); res != nil {
		return res.(*dto.PaymentOrderSummary), args.Error(1)
	}
	return nil, args.Error(1)
}

// dong builds the ledger NUMERIC(15,2) value for a whole-dong amount, the way
// the payment service writes it.
func dong(t *testing.T, wholeDong int64) pgtype.Numeric {
	t.Helper()
	amount, err := money.FromWholeDong(wholeDong)
	require.NoError(t, err)
	return amount.Numeric()
}

func TestPaymentService_CreateCODPaymentIntent(t *testing.T) {
	mockRepo := new(MockPaymentRepo)
	mockOrderReader := new(MockOrderReader)

	orderUUID := uuid.New()
	ptUUID := uuid.New()

	// The order service still reports its total as a gRPC double in dong.
	mockOrderReader.On("GetByUUID", mock.Anything, orderUUID).Return(&dto.PaymentOrderSummary{
		OrderID:     10,
		OrderUUID:   orderUUID,
		OrderNumber: "ORD-12345",
		TotalAmount: 150000,
	}, nil)

	mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(p paymentsqlc.CreatePaymentTransactionParams) bool {
		return p.OrderID == 10 && p.Provider == "cod"
	})).Return(paymentsqlc.PaymentTransaction{
		PtID:       1,
		PtUuid:     ptUUID,
		PtOrderID:  10,
		PtProvider: "cod",
		PtAmount:   dong(t, 150000),
		PtCurrency: "VND",
		PtStatus:   1,
	}, nil)

	svc := paymentservice.NewPaymentService(
		mockRepo,
		mockOrderReader,
		nil,
		nil,
		nil,
		nil,
		nil,
		"http://localhost:3001/payment/result",
		"http://localhost:8083",
	)

	intent, err := svc.CreatePaymentIntent(context.Background(), dto.CreatePaymentIntentInput{
		OrderUUID: orderUUID,
		Provider:  "cod",
	})

	assert.NoError(t, err)
	assert.Equal(t, ptUUID.String(), intent.PaymentUUID)
	assert.Equal(t, "cod", intent.Provider)
	// 150000 dong = 15_000_000 xu; the API speaks minor units.
	assert.Equal(t, int64(15_000_000), intent.Amount)
}

// A total a float cannot represent exactly must be refused, not rounded into
// the payment request.
func TestPaymentService_CreatePaymentIntent_RejectsUnexactTotal(t *testing.T) {
	mockRepo := new(MockPaymentRepo)
	mockOrderReader := new(MockOrderReader)
	orderUUID := uuid.New()

	mockOrderReader.On("GetByUUID", mock.Anything, orderUUID).Return(&dto.PaymentOrderSummary{
		OrderID:     10,
		OrderUUID:   orderUUID,
		OrderNumber: "ORD-1",
		TotalAmount: 100000.005,
	}, nil)

	svc := paymentservice.NewPaymentService(
		mockRepo, mockOrderReader, nil, nil, nil, nil, nil,
		"http://localhost:3001/payment/result", "http://localhost:8083",
	)

	_, err := svc.CreatePaymentIntent(context.Background(), dto.CreatePaymentIntentInput{
		OrderUUID: orderUUID,
		Provider:  "cod",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be charged exactly")
	mockRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestPaymentService_ConfirmCODPayment_NoPendingTransaction(t *testing.T) {
	mockRepo := new(MockPaymentRepo)
	orderUUID := uuid.New()

	mockRepo.On("GetByOrderUUID", mock.Anything, orderUUID).Return([]paymentsqlc.PaymentTransaction{}, nil)

	svc := paymentservice.NewPaymentService(
		mockRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		"http://localhost:3001/payment/result",
		"http://localhost:8083",
	)

	err := svc.ConfirmCODPayment(context.Background(), orderUUID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no pending COD transaction found")
}

func TestPaymentService_RefundPayment_ExceedsPaidAmount(t *testing.T) {
	mockRepo := new(MockPaymentRepo)
	orderUUID := uuid.New()

	mockRepo.On("GetByOrderUUID", mock.Anything, orderUUID).Return([]paymentsqlc.PaymentTransaction{
		{
			PtID:       1,
			PtUuid:     uuid.New(),
			PtOrderID:  10,
			PtStatus:   2, // Paid
			PtAmount:   dong(t, 100000),
			PtCurrency: "VND",
			PtProvider: "cod",
		},
	}, nil)

	svc := paymentservice.NewPaymentService(
		mockRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		"http://localhost:3001/payment/result",
		"http://localhost:8083",
	)

	_, err := svc.RefundPayment(context.Background(), dto.RefundPaymentInput{
		OrderUUID: orderUUID.String(),
		Amount:    20_000_000, // 200000 dong, exceeds the 100000 dong payment
		Reason:    "Customer complaint",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds original transaction amount")
}

func TestPaymentService_RefundPayment_RejectsCumulativeAmount(t *testing.T) {
	mockRepo := new(MockPaymentRepo)
	orderUUID := uuid.New()
	paidUUID := uuid.New()
	existingRefundID := 2
	refundOf := int32(1)

	mockRepo.On("GetByOrderUUID", mock.Anything, orderUUID).Return([]paymentsqlc.PaymentTransaction{
		{
			PtID:       1,
			PtUuid:     paidUUID,
			PtOrderID:  10,
			PtStatus:   2,
			PtAmount:   dong(t, 100000),
			PtCurrency: "VND",
			PtProvider: "cod",
		},
		{
			PtID:           int32(existingRefundID),
			PtRefundOf:     &refundOf,
			PtRefundAmount: dong(t, 75000),
			PtAmount:       dong(t, -75000),
			PtStatus:       5,
		},
	}, nil)
	// The idempotency key is built from integer minor units.
	mockRepo.On("GetByIdempotencyKey", mock.Anything, "refund:"+orderUUID.String()+":3000000").
		Return(paymentsqlc.PaymentTransaction{}, pgx.ErrNoRows)

	svc := paymentservice.NewPaymentService(
		mockRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		"http://localhost:3001/payment/result",
		"http://localhost:8083",
	)

	_, err := svc.RefundPayment(context.Background(), dto.RefundPaymentInput{
		OrderUUID: orderUUID.String(),
		Amount:    3_000_000, // 30000 dong; 75000 already refunded
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "total refund amount")
	mockRepo.AssertNotCalled(t, "CreateRefund", mock.Anything, mock.Anything)
}

// The cumulative ceiling is exact: an amount that lands precisely on the paid
// total is allowed, one xu more is not. The old float sum could not tell these
// apart.
func TestPaymentService_RefundPayment_CumulativeCeilingIsExact(t *testing.T) {
	orderPaid := int64(10_000_000) // 100000 dong
	alreadyRefunded := int64(3_333_300)

	for _, tc := range []struct {
		name    string
		refund  int64
		wantErr bool
	}{
		{"lands exactly on the paid total", orderPaid - alreadyRefunded, false},
		{"one xu over the paid total", orderPaid - alreadyRefunded + 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := new(MockPaymentRepo)
			orderUUID := uuid.New()
			paidUUID := uuid.New()
			refundOf := int32(1)
			txnRef := "261023-34-EXACT"
			providerTxnID := "14567890"

			mockRepo.On("GetByOrderUUID", mock.Anything, orderUUID).Return([]paymentsqlc.PaymentTransaction{
				{
					PtID:            1,
					PtUuid:          paidUUID,
					PtOrderID:       10,
					PtStatus:        2,
					PtAmount:        money.FromMinorUnits(orderPaid).Numeric(),
					PtCurrency:      "VND",
					PtProvider:      "vnpay",
					PtTxnRef:        &txnRef,
					PtProviderTxnID: &providerTxnID,
				},
				{
					PtID:           2,
					PtRefundOf:     &refundOf,
					PtRefundAmount: money.FromMinorUnits(alreadyRefunded).Numeric(),
					PtAmount:       money.FromMinorUnits(-alreadyRefunded).Numeric(),
					PtStatus:       5,
				},
			}, nil)
			mockRepo.On("GetByIdempotencyKey", mock.Anything, mock.Anything).
				Return(paymentsqlc.PaymentTransaction{}, pgx.ErrNoRows)
			mockRepo.On("CreateRefund", mock.Anything, mock.Anything).Return(paymentsqlc.PaymentTransaction{
				PtID:        3,
				PtUuid:      uuid.New(),
				PtOrderUuid: pgtype.UUID{Bytes: orderUUID, Valid: true},
				PtAmount:    money.FromMinorUnits(-tc.refund).Numeric(),
				PtCurrency:  "VND",
				PtStatus:    5,
			}, nil)

			svc := paymentservice.NewPaymentService(
				mockRepo, nil, nil, nil,
				map[string]provider.PaymentProvider{
					"vnpay": &fakeProvider{refundResp: provider.RefundResponse{Status: provider.TxnStatusSuccess, RefundTxnID: "R-1"}},
				},
				nil, nil,
				"http://localhost:3001/payment/result", "",
			)

			_, err := svc.RefundPayment(context.Background(), dto.RefundPaymentInput{
				OrderUUID: orderUUID.String(),
				Amount:    tc.refund,
			})

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "total refund amount")
				mockRepo.AssertNotCalled(t, "CreateRefund", mock.Anything, mock.Anything)
				return
			}
			require.NoError(t, err)
			mockRepo.AssertCalled(t, "CreateRefund", mock.Anything, mock.Anything)
		})
	}
}

// The refund row stores the amount without float rounding, and the gateway is
// asked for exactly the requested minor units.
func TestPaymentService_RefundPayment_PersistsExactMinorUnits(t *testing.T) {
	mockRepo := new(MockPaymentRepo)
	orderUUID := uuid.New()
	paidUUID := uuid.New()
	txnRef := "261023-34-EXACT-ROW"
	providerTxnID := "14567890"

	mockRepo.On("GetByOrderUUID", mock.Anything, orderUUID).Return([]paymentsqlc.PaymentTransaction{
		{
			PtID:            1,
			PtUuid:          paidUUID,
			PtOrderID:       10,
			PtStatus:        2,
			PtAmount:        money.FromMinorUnits(10_000_000).Numeric(),
			PtCurrency:      "VND",
			PtProvider:      "vnpay",
			PtTxnRef:        &txnRef,
			PtProviderTxnID: &providerTxnID,
		},
	}, nil)
	mockRepo.On("GetByIdempotencyKey", mock.Anything, mock.Anything).
		Return(paymentsqlc.PaymentTransaction{}, pgx.ErrNoRows)

	var captured paymentsqlc.CreateRefundTransactionParams
	mockRepo.On("CreateRefund", mock.Anything, mock.MatchedBy(func(p paymentsqlc.CreateRefundTransactionParams) bool {
		captured = p
		return true
	})).Return(paymentsqlc.PaymentTransaction{
		PtID:        2,
		PtUuid:      uuid.New(),
		PtOrderUuid: pgtype.UUID{Bytes: orderUUID, Valid: true},
		PtAmount:    money.FromMinorUnits(-1_234_567).Numeric(),
		PtCurrency:  "VND",
		PtStatus:    5,
	}, nil)

	gateway := &fakeProvider{refundResp: provider.RefundResponse{Status: provider.TxnStatusSuccess, RefundTxnID: "R-2"}}
	svc := paymentservice.NewPaymentService(
		mockRepo, nil, nil, nil,
		map[string]provider.PaymentProvider{"vnpay": gateway},
		nil, nil, "http://localhost:3001/payment/result", "",
	)

	_, err := svc.RefundPayment(context.Background(), dto.RefundPaymentInput{
		OrderUUID: orderUUID.String(),
		Amount:    1_234_567, // 12345.67 dong
	})

	require.NoError(t, err)
	// 12345.67 dong = 1_234_567 xu stored as NUMERIC(15,2).
	assert.Equal(t, "1234567", captured.RefundAmount.Int.String())
	assert.Equal(t, int32(-2), captured.RefundAmount.Exp)
	assert.Equal(t, "-1234567", captured.Amount.Int.String())
}

func TestPaymentService_RefundPayment_ReusesDeterministicIdempotencyKey(t *testing.T) {
	mockRepo := new(MockPaymentRepo)
	orderUUID := uuid.New()
	paidUUID := uuid.New()
	refundUUID := uuid.New()

	mockRepo.On("GetByOrderUUID", mock.Anything, orderUUID).Return([]paymentsqlc.PaymentTransaction{
		func() paymentsqlc.PaymentTransaction {
			txnRef := "261023-34-REFUND"
			txnID := "14567890"
			return paymentsqlc.PaymentTransaction{
				PtID:            1,
				PtUuid:          paidUUID,
				PtOrderID:       10,
				PtStatus:        2,
				PtAmount:        money.FromMinorUnits(10_000_000).Numeric(),
				PtCurrency:      "VND",
				PtProvider:      "vnpay",
				PtTxnRef:        &txnRef,
				PtProviderTxnID: &txnID,
			}
		}(),
	}, nil)
	mockRepo.On("GetByIdempotencyKey", mock.Anything, "refund:"+orderUUID.String()+":2500000").
		Return(paymentsqlc.PaymentTransaction{}, pgx.ErrNoRows)
	mockRepo.On("CreateRefund", mock.Anything, mock.MatchedBy(func(params paymentsqlc.CreateRefundTransactionParams) bool {
		return params.IdempotencyKey == "refund:"+orderUUID.String()+":2500000"
	})).Return(paymentsqlc.PaymentTransaction{
		PtID:        2,
		PtUuid:      refundUUID,
		PtOrderUuid: pgtype.UUID{Bytes: orderUUID, Valid: true},
		PtProvider:  "vnpay",
		PtAmount:    money.FromMinorUnits(-2_500_000).Numeric(),
		PtCurrency:  "VND",
		PtStatus:    5,
	}, nil)

	vnpayTxnID := "14567890"
	gatewayProvider := &fakeProvider{refundResp: provider.RefundResponse{Status: provider.TxnStatusSuccess, RefundTxnID: vnpayTxnID}}
	svc := paymentservice.NewPaymentService(
		mockRepo,
		nil,
		nil,
		nil,
		map[string]provider.PaymentProvider{"vnpay": gatewayProvider},
		nil,
		nil,
		"http://localhost:3001/payment/result",
		"",
	)

	result, err := svc.RefundPayment(context.Background(), dto.RefundPaymentInput{
		OrderUUID: orderUUID.String(),
		Amount:    2_500_000, // 25000 dong
		Reason:    "Customer complaint",
	})

	require.NoError(t, err)
	assert.Equal(t, refundUUID.String(), result.UUID)
	mockRepo.AssertExpectations(t)
}

// MockWebhookRepo records whether the audit row was ever written. A rejected
// callback must not create one: the settlement transaction rolls back.
type MockWebhookRepo struct {
	mock.Mock
}

func (m *MockWebhookRepo) Insert(ctx context.Context, params paymentsqlc.InsertWebhookEventParams) (paymentsqlc.WebhookEvent, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(paymentsqlc.WebhookEvent), args.Error(1)
}

func (m *MockWebhookRepo) MarkProcessed(ctx context.Context, eventID int32) error {
	args := m.Called(ctx, eventID)
	return args.Error(0)
}

var _ repository.WebhookEventRepository = (*MockWebhookRepo)(nil)

// An underpaying callback settles nothing: no status change, no audit row, and
// an error the handler maps to the provider's "invalid amount" code.
func TestPaymentService_SettlePayment_UnderpaymentChangesNothing(t *testing.T) {
	mockRepo := new(MockPaymentRepo)
	mockWebhook := new(MockWebhookRepo)

	txn := paymentsqlc.PaymentTransaction{
		PtID:       1,
		PtUuid:     uuid.New(),
		PtProvider: "vnpay",
		PtTxnRef:   strPtr("261028-34-TEST01"),
		PtAmount:   dong(t, 150000),
		PtCurrency: "VND",
		PtStatus:   paymentservice.PaymentStatusPending,
	}

	svc := paymentservice.NewPaymentService(
		mockRepo, nil, mockWebhook, nil, nil, nil, nil,
		"http://localhost:3001/payment/result", "http://localhost:8083",
	)

	mockRepo.On("GetByTxnRef", mock.Anything, "vnpay", "261028-34-TEST01").Return(txn, nil)

	err := svc.ProcessWebhook(context.Background(), dto.PaymentWebhookMessage{
		Provider:    "vnpay",
		OrderNumber: "261028-34-TEST01",
		Amount:      1000, // 10 dong, far below the 150000 dong charged
		Status:      string(provider.TxnStatusSuccess),
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, paymentservice.ErrWebhookAmountMismatch)
	// Rejection happens before any write: no status transition, no audit row.
	mockRepo.AssertNotCalled(t, "UpdateStatusAtomically", mock.Anything, mock.Anything)
	mockWebhook.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
}

func strPtr(s string) *string { return &s }
