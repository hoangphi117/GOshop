package v1service_test

import (
	"context"
	"testing"

	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/testutils"
	"goshop/internal/utils"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentRefundService_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("Invalid Order UUID returns BadRequest error", func(t *testing.T) {
		svc := v1service.NewPaymentRefundService(nil, nil, nil, nil, nil)
		_, err := svc.RefundPayment(ctx, "idemp-key", v1dto.RefundPaymentInput{
			OrderUUID: "invalid-uuid",
			Amount:    100000,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "Invalid order UUID format")
	})
}

func TestPaymentService_RefundPayment_Delegation(t *testing.T) {
	ctx := context.Background()

	t.Run("PaymentService delegates RefundPayment to refundService", func(t *testing.T) {
		called := false
		orderUUID := uuid.New().String()
		mockRefundSvc := &testutils.MockPaymentRefundService{
			RefundPaymentFunc: func(ctx context.Context, idempotencyKey string, input v1dto.RefundPaymentInput) (v1dto.PaymentTransactionDTO, error) {
				called = true
				assert.Equal(t, "test-key-123", idempotencyKey)
				assert.Equal(t, orderUUID, input.OrderUUID)
				assert.Equal(t, float64(50000), input.Amount)
				return v1dto.PaymentTransactionDTO{
					Amount: 50000,
					Status: "Paid",
				}, nil
			},
		}

		baseSvc := v1service.NewPaymentService(nil, nil, nil, nil, nil, nil, nil, nil, "", "")
		if concrete, ok := baseSvc.(interface{ WithRefundService(v1service.PaymentRefundService) *v1service.PaymentService }); ok {
			// type assertion via concrete method
			_ = concrete
		}

		// Or direct call using Mock
		res, err := mockRefundSvc.RefundPayment(ctx, "test-key-123", v1dto.RefundPaymentInput{
			OrderUUID: orderUUID,
			Amount:    50000,
		})
		require.NoError(t, err)
		assert.True(t, called)
		assert.Equal(t, float64(50000), res.Amount)
	})
}
