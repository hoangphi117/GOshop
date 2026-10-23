package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"goshop/pkg/utils"
	paymentsqlc "goshop/services/payment/internal/db/sqlc"
	"goshop/services/payment/internal/dto"
	"goshop/services/payment/internal/provider"
	paymentservice "goshop/services/payment/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// fakeProvider implements provider.PaymentProvider with canned query answers.
type fakeProvider struct {
	status     provider.TransactionStatus
	err        error
	refundResp provider.RefundResponse
	refundErr  error
}

func (p *fakeProvider) CreatePaymentURL(context.Context, provider.CreatePaymentRequest) (provider.CreatePaymentResponse, error) {
	return provider.CreatePaymentResponse{}, nil
}
func (p *fakeProvider) VerifyWebhook([]byte, map[string]string) error { return nil }
func (p *fakeProvider) ParseWebhookPayload([]byte) (provider.WebhookResult, error) {
	return provider.WebhookResult{}, nil
}
func (p *fakeProvider) QueryTransactionStatus(context.Context, string, string) (provider.TransactionStatus, error) {
	return p.status, p.err
}
func (p *fakeProvider) Refund(context.Context, provider.RefundRequest) (provider.RefundResponse, error) {
	return p.refundResp, p.refundErr
}

func newTestService(repo *MockPaymentRepo, providers map[string]provider.PaymentProvider) paymentservice.PaymentService {
	return paymentservice.NewPaymentService(
		repo, nil, nil, nil, providers, nil, nil,
		"http://localhost:3001/payment/result", "http://localhost:8083",
	)
}

func pendingTxn(createdAt time.Time) paymentsqlc.PaymentTransaction {
	txnRef := "261023-34-RPM37X"
	return paymentsqlc.PaymentTransaction{
		PtID:        4,
		PtUuid:      uuid.New(),
		PtOrderID:   34,
		PtProvider:  "vnpay",
		PtTxnRef:    &txnRef,
		PtAmount:    utils.FloatToNumeric(380865),
		PtCurrency:  "VND",
		PtStatus:    paymentservice.PaymentStatusPending,
		PtCreatedAt: createdAt,
	}
}

// The gateway is the source of truth: a pending transaction the gateway reports
// as paid must be finalized instead of expired, even when no IPN ever arrived.
func TestReconcile_ConfirmsPaidTransactionWhenGatewayReportsSuccess(t *testing.T) {
	repo := new(MockPaymentRepo)
	txn := pendingTxn(time.Now().UTC().Add(-20 * time.Minute))

	repo.On("GetPendingOlderThan", mock.Anything, mock.Anything, mock.Anything).
		Return([]paymentsqlc.PaymentTransaction{txn}, nil)
	repo.On("UpdateStatusAtomically", mock.Anything, mock.MatchedBy(func(p paymentsqlc.UpdatePaymentStatusAtomicallyParams) bool {
		return p.PtUuid == txn.PtUuid && p.NewStatus == paymentservice.PaymentStatusPaid
	})).Return(txn, nil)

	svc := newTestService(repo, map[string]provider.PaymentProvider{
		"vnpay": &fakeProvider{status: provider.TxnStatusSuccess},
	})

	err := svc.ReconcilePendingPayments(context.Background())

	assert.NoError(t, err)
	assert.True(t, repo.AssertCalled(t, "UpdateStatusAtomically", mock.Anything, mock.Anything))
}

// Gateway still says pending: the transaction is expired as before.
func TestReconcile_ExpiresTransactionGatewayReportsUnpaid(t *testing.T) {
	repo := new(MockPaymentRepo)
	txn := pendingTxn(time.Now().UTC().Add(-20 * time.Minute))

	repo.On("GetPendingOlderThan", mock.Anything, mock.Anything, mock.Anything).
		Return([]paymentsqlc.PaymentTransaction{txn}, nil)
	repo.On("UpdateStatusAtomically", mock.Anything, mock.MatchedBy(func(p paymentsqlc.UpdatePaymentStatusAtomicallyParams) bool {
		return p.NewStatus == paymentservice.PaymentStatusExpired
	})).Return(txn, nil)

	svc := newTestService(repo, map[string]provider.PaymentProvider{
		"vnpay": &fakeProvider{status: provider.TxnStatusPending},
	})

	assert.NoError(t, svc.ReconcilePendingPayments(context.Background()))
	repo.AssertExpectations(t)
}

// An inconclusive gateway answer must not expire a payment we cannot verify.
func TestReconcile_KeepsTransactionPendingWhenQueryFails(t *testing.T) {
	repo := new(MockPaymentRepo)
	txn := pendingTxn(time.Now().UTC().Add(-20 * time.Minute))

	repo.On("GetPendingOlderThan", mock.Anything, mock.Anything, mock.Anything).
		Return([]paymentsqlc.PaymentTransaction{txn}, nil)

	svc := newTestService(repo, map[string]provider.PaymentProvider{
		"vnpay": &fakeProvider{err: errors.New("querydr timeout")},
	})

	assert.NoError(t, svc.ReconcilePendingPayments(context.Background()))
	repo.AssertNotCalled(t, "UpdateStatusAtomically", mock.Anything, mock.Anything)
}

// A callback that claims success for less than the order total must not settle
// the transaction and must surface as an amount mismatch.
func TestProcessWebhook_RejectsUncoveredAmount(t *testing.T) {
	repo := new(MockPaymentRepo)
	txn := pendingTxn(time.Now().UTC())

	repo.On("GetByTxnRef", mock.Anything, "vnpay", *txn.PtTxnRef).Return(txn, nil)

	svc := newTestService(repo, map[string]provider.PaymentProvider{
		"vnpay": &fakeProvider{},
	})

	err := svc.ProcessWebhook(context.Background(), dto.PaymentWebhookMessage{
		Provider:    "vnpay",
		OrderNumber: *txn.PtTxnRef,
		Amount:      1000,
		Status:      string(provider.TxnStatusSuccess),
	})

	assert.Error(t, err)
	assert.True(t, errors.Is(err, paymentservice.ErrWebhookAmountMismatch))
	repo.AssertNotCalled(t, "UpdateStatusAtomically", mock.Anything, mock.Anything)
}

// Unknown callbacks must report a not-found sentinel so the handler can answer
// the provider with the correct code.
func TestProcessWebhook_UnknownTransactionReturnsNotFound(t *testing.T) {
	repo := new(MockPaymentRepo)
	repo.On("GetByTxnRef", mock.Anything, "vnpay", "missing").Return(paymentsqlc.PaymentTransaction{}, pgx.ErrNoRows)

	svc := newTestService(repo, map[string]provider.PaymentProvider{
		"vnpay": &fakeProvider{},
	})

	err := svc.ProcessWebhook(context.Background(), dto.PaymentWebhookMessage{
		Provider:    "vnpay",
		OrderNumber: "missing",
		Status:      string(provider.TxnStatusSuccess),
	})

	assert.True(t, errors.Is(err, paymentservice.ErrPaymentTxnNotFound))
}

// A refund the gateway rejected must not write a refund row: money did not move.
func TestRefundPayment_GatewayRejectedWritesNothing(t *testing.T) {
	repo := new(MockPaymentRepo)
	txn := pendingTxn(time.Now().UTC().Add(-1 * time.Hour))
	txn.PtStatus = paymentservice.PaymentStatusPaid
	providerTxnID := "14567890"
	txn.PtProviderTxnID = &providerTxnID

	repo.On("GetByOrderUUID", mock.Anything, mock.Anything).Return([]paymentsqlc.PaymentTransaction{txn}, nil)
	repo.On("GetByIdempotencyKey", mock.Anything, mock.Anything).Return(paymentsqlc.PaymentTransaction{}, pgx.ErrNoRows)

	svc := newTestService(repo, map[string]provider.PaymentProvider{
		"vnpay": &fakeProvider{refundResp: provider.RefundResponse{Status: provider.TxnStatusFailed, Message: "insufficient balance"}},
	})

	_, err := svc.RefundPayment(context.Background(), dto.RefundPaymentInput{
		OrderUUID: uuid.New().String(),
		Amount:    100000,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient balance")
	repo.AssertNotCalled(t, "CreateRefund", mock.Anything, mock.Anything)
}

// Gateway confirms the refund: the refund row carries the gateway refund txn id.
func TestRefundPayment_GatewayConfirmedPersistsRefund(t *testing.T) {
	repo := new(MockPaymentRepo)
	txn := pendingTxn(time.Now().UTC().Add(-1 * time.Hour))
	txn.PtStatus = paymentservice.PaymentStatusPaid
	providerTxnID := "14567890"
	txn.PtProviderTxnID = &providerTxnID

	repo.On("GetByOrderUUID", mock.Anything, mock.Anything).Return([]paymentsqlc.PaymentTransaction{txn}, nil)
	repo.On("GetByIdempotencyKey", mock.Anything, mock.Anything).Return(paymentsqlc.PaymentTransaction{}, pgx.ErrNoRows)
	repo.On("CreateRefund", mock.Anything, mock.MatchedBy(func(p paymentsqlc.CreateRefundTransactionParams) bool {
		return p.ProviderTxnID != nil && *p.ProviderTxnID == "REFUND-TXN-1"
	})).Return(txn, nil)

	svc := newTestService(repo, map[string]provider.PaymentProvider{
		"vnpay": &fakeProvider{refundResp: provider.RefundResponse{Status: provider.TxnStatusSuccess, RefundTxnID: "REFUND-TXN-1"}},
	})

	_, err := svc.RefundPayment(context.Background(), dto.RefundPaymentInput{
		OrderUUID: uuid.New().String(),
		Amount:    100000,
	})

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}
