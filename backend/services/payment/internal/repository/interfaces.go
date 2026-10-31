package repository

import (
	"context"
	"time"

	paymentsqlc "goshop/services/payment/internal/db/sqlc"

	"github.com/google/uuid"
)

type PaymentRepository interface {
	Create(ctx context.Context, params paymentsqlc.CreatePaymentTransactionParams) (paymentsqlc.PaymentTransaction, error)
	GetByIdempotencyKey(ctx context.Context, key string) (paymentsqlc.PaymentTransaction, error)
	GetByUUID(ctx context.Context, ptUUID uuid.UUID) (paymentsqlc.PaymentTransaction, error)
	GetByTxnRef(ctx context.Context, provider, txnRef string) (paymentsqlc.PaymentTransaction, error)
	GetByProviderTxnID(ctx context.Context, provider, providerTxnID string) (paymentsqlc.PaymentTransaction, error)
	UpdateStatusAtomically(ctx context.Context, params paymentsqlc.UpdatePaymentStatusAtomicallyParams) (paymentsqlc.PaymentTransaction, error)
	GetByOrderID(ctx context.Context, orderID int32) ([]paymentsqlc.PaymentTransaction, error)
	GetByOrderUUID(ctx context.Context, orderUUID uuid.UUID) ([]paymentsqlc.PaymentTransaction, error)
	CreateRefund(ctx context.Context, params paymentsqlc.CreateRefundTransactionParams) (paymentsqlc.PaymentTransaction, error)
	GetPendingOlderThan(ctx context.Context, olderThan time.Time, limit int32) ([]paymentsqlc.PaymentTransaction, error)
	UpdateProviderInfo(ctx context.Context, params paymentsqlc.UpdatePaymentProviderInfoParams) (paymentsqlc.PaymentTransaction, error)
}

type WebhookEventRepository interface {
	Insert(ctx context.Context, params paymentsqlc.InsertWebhookEventParams) (paymentsqlc.WebhookEvent, error)
	MarkProcessed(ctx context.Context, eventID int32) error
}

type OutboxRepository interface {
	Create(ctx context.Context, aggregateType, aggregateID, eventType string, payload []byte) (paymentsqlc.OutboxEvent, error)
	GetPending(ctx context.Context, limit int32) ([]paymentsqlc.OutboxEvent, error)
	MarkPublished(ctx context.Context, eventID int64) error
}
