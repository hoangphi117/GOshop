package service_test

import (
	"context"
	"testing"
	"time"

	"goshop/pkg/utils"
	paymentsqlc "goshop/services/payment/internal/db/sqlc"
	"goshop/services/payment/internal/dto"
	paymentservice "goshop/services/payment/internal/service"

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

func TestPaymentService_CreateCODPaymentIntent(t *testing.T) {
	mockRepo := new(MockPaymentRepo)
	mockOrderReader := new(MockOrderReader)

	orderUUID := uuid.New()
	ptUUID := uuid.New()

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
		PtAmount:   utils.FloatToNumeric(150000),
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
	assert.Equal(t, float64(150000), intent.Amount)
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
			PtAmount:   utils.FloatToNumeric(100000),
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
		Amount:    200000, // Exceeds 100000
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
			PtAmount:   utils.FloatToNumeric(100000),
			PtCurrency: "VND",
			PtProvider: "cod",
		},
		{
			PtID:           int32(existingRefundID),
			PtRefundOf:     &refundOf,
			PtRefundAmount: utils.FloatToNumeric(75000),
			PtAmount:       utils.FloatToNumeric(-75000),
			PtStatus:       5,
		},
	}, nil)
	mockRepo.On("GetByIdempotencyKey", mock.Anything, "refund:"+orderUUID.String()+":30000.00").
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
		Amount:    30000,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "total refund amount")
	mockRepo.AssertNotCalled(t, "CreateRefund", mock.Anything)
}

func TestPaymentService_RefundPayment_ReusesDeterministicIdempotencyKey(t *testing.T) {
	mockRepo := new(MockPaymentRepo)
	orderUUID := uuid.New()
	paidUUID := uuid.New()
	refundUUID := uuid.New()

	mockRepo.On("GetByOrderUUID", mock.Anything, orderUUID).Return([]paymentsqlc.PaymentTransaction{
		{
			PtID:       1,
			PtUuid:     paidUUID,
			PtOrderID:  10,
			PtStatus:   2,
			PtAmount:   utils.FloatToNumeric(100000),
			PtCurrency: "VND",
			PtProvider: "cod",
		},
	}, nil)
	mockRepo.On("GetByIdempotencyKey", mock.Anything, "refund:"+orderUUID.String()+":25000.00").
		Return(paymentsqlc.PaymentTransaction{}, pgx.ErrNoRows)
	mockRepo.On("CreateRefund", mock.Anything, mock.MatchedBy(func(params paymentsqlc.CreateRefundTransactionParams) bool {
		return params.IdempotencyKey == "refund:"+orderUUID.String()+":25000.00"
	})).Return(paymentsqlc.PaymentTransaction{
		PtID:        2,
		PtUuid:      refundUUID,
		PtOrderUuid: pgtype.UUID{Bytes: orderUUID, Valid: true},
		PtProvider:  "cod",
		PtAmount:    utils.FloatToNumeric(-25000),
		PtCurrency:  "VND",
		PtStatus:    4,
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

	result, err := svc.RefundPayment(context.Background(), dto.RefundPaymentInput{
		OrderUUID: orderUUID.String(),
		Amount:    25000,
		Reason:    "Customer complaint",
	})

	require.NoError(t, err)
	assert.Equal(t, refundUUID.String(), result.UUID)
	mockRepo.AssertExpectations(t)
}
