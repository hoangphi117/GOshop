package client

import (
	"context"
	"errors"
	"testing"
	"time"

	ordercontract "goshop/services/order/contract"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockOrderClient struct {
	summaryFunc func(ctx context.Context, orderUUID uuid.UUID) (*ordercontract.OrderSummary, error)
	paymentFunc func(ctx context.Context, orderUUID uuid.UUID) (*ordercontract.OrderPaymentInfo, error)
}

func (m *mockOrderClient) GetOrderSummary(ctx context.Context, orderUUID uuid.UUID) (*ordercontract.OrderSummary, error) {
	if m.summaryFunc != nil {
		return m.summaryFunc(ctx, orderUUID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockOrderClient) VerifyPurchase(ctx context.Context, orderUUID uuid.UUID, userID, productID int32, userUUID, productUUID *uuid.UUID) (*ordercontract.PurchaseVerification, error) {
	return nil, errors.New("not implemented")
}

func (m *mockOrderClient) GetOrderForPayment(ctx context.Context, orderUUID uuid.UUID) (*ordercontract.OrderPaymentInfo, error) {
	if m.paymentFunc != nil {
		return m.paymentFunc(ctx, orderUUID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockOrderClient) Close() error {
	return nil
}

func TestGRPCOrderReader_GetOrderSummary(t *testing.T) {
	targetUUID := uuid.New()
	userUUID := uuid.New()
	now := time.Now()

	mockClient := &mockOrderClient{
		summaryFunc: func(ctx context.Context, orderUUID uuid.UUID) (*ordercontract.OrderSummary, error) {
			if orderUUID == targetUUID {
				return &ordercontract.OrderSummary{
					OrderID:       101,
					OrderUUID:     targetUUID,
					OrderNumber:   "ORD-TEST-101",
					UserID:        55,
					UserUUID:      userUUID,
					CustomerName:  "Test Customer",
					CustomerEmail: "test@example.com",
					Status:        2,
					TotalAmount:   500000.0,
					DeliveredAt:   &now,
					UpdatedAt:     now,
				}, nil
			}
			return nil, errors.New("order not found")
		},
	}

	reader := NewGRPCOrderReader(mockClient)

	// Success case
	summary, err := reader.GetOrderSummary(context.Background(), targetUUID)
	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, int32(101), summary.OrderID)
	assert.Equal(t, targetUUID, summary.OrderUUID)
	assert.Equal(t, "ORD-TEST-101", summary.OrderNumber)
	assert.Equal(t, int32(55), summary.UserID)
	assert.Equal(t, 500000.0, summary.TotalAmount)

	// Error case
	_, err = reader.GetOrderSummary(context.Background(), uuid.New())
	assert.Error(t, err)

	// Nil client case
	nilReader := NewGRPCOrderReader(nil)
	_, err = nilReader.GetOrderSummary(context.Background(), targetUUID)
	assert.Error(t, err)
}
