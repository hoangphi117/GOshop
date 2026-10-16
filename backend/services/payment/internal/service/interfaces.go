package service

import (
	"context"

	"goshop/services/payment/internal/dto"

	"github.com/google/uuid"
)

type PaymentService interface {
	CreatePaymentIntent(ctx context.Context, input dto.CreatePaymentIntentInput) (dto.PaymentIntentDTO, error)
	GetPaymentTransactionsByOrderID(ctx context.Context, orderID int32) ([]dto.PaymentTransactionDTO, error)
	GetPaymentTransactionsByOrderUUID(ctx context.Context, orderUUID uuid.UUID) ([]dto.PaymentTransactionDTO, error)
	ConfirmCODPayment(ctx context.Context, orderUUID uuid.UUID) error
	RefundPayment(ctx context.Context, input dto.RefundPaymentInput) (dto.PaymentTransactionDTO, error)
	HandleWebhook(ctx context.Context, providerName string, rawPayload []byte, headers map[string]string) error
	ProcessWebhook(ctx context.Context, msg dto.PaymentWebhookMessage) error
	ReconcilePendingPayments(ctx context.Context) error
}

type OrderReaderClient interface {
	GetByUUID(ctx context.Context, orderUUID uuid.UUID) (*dto.PaymentOrderSummary, error)
	GetByOrderNumber(ctx context.Context, orderNumber string) (*dto.PaymentOrderSummary, error)
}
