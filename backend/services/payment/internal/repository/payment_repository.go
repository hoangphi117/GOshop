package repository

import (
	"context"
	"fmt"
	"time"

	paymentsqlc "goshop/services/payment/internal/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type sqlPaymentRepository struct {
	querier paymentsqlc.Querier
}

func NewSqlPaymentRepository(querier paymentsqlc.Querier) PaymentRepository {
	return &sqlPaymentRepository{querier: querier}
}

func (r *sqlPaymentRepository) Create(ctx context.Context, params paymentsqlc.CreatePaymentTransactionParams) (paymentsqlc.PaymentTransaction, error) {
	txn, err := r.querier.CreatePaymentTransaction(ctx, params)
	if err != nil {
		return paymentsqlc.PaymentTransaction{}, fmt.Errorf("failed to create payment transaction: %w", err)
	}
	return txn, nil
}

func (r *sqlPaymentRepository) GetByIdempotencyKey(ctx context.Context, key string) (paymentsqlc.PaymentTransaction, error) {
	txn, err := r.querier.GetPaymentByIdempotencyKey(ctx, key)
	if err != nil {
		return paymentsqlc.PaymentTransaction{}, fmt.Errorf("failed to get payment by idempotency key: %w", err)
	}
	return txn, nil
}

func (r *sqlPaymentRepository) GetByUUID(ctx context.Context, ptUUID uuid.UUID) (paymentsqlc.PaymentTransaction, error) {
	txn, err := r.querier.GetPaymentByUUID(ctx, ptUUID)
	if err != nil {
		return paymentsqlc.PaymentTransaction{}, fmt.Errorf("failed to get payment by uuid %s: %w", ptUUID, err)
	}
	return txn, nil
}

func (r *sqlPaymentRepository) GetByTxnRef(ctx context.Context, provider, txnRef string) (paymentsqlc.PaymentTransaction, error) {
	txn, err := r.querier.GetPaymentByTxnRef(ctx, paymentsqlc.GetPaymentByTxnRefParams{
		PtProvider: provider,
		PtTxnRef:   &txnRef,
	})
	if err != nil {
		return paymentsqlc.PaymentTransaction{}, fmt.Errorf("failed to get payment by txn ref %s: %w", txnRef, err)
	}
	return txn, nil
}

func (r *sqlPaymentRepository) GetByProviderTxnID(ctx context.Context, provider, providerTxnID string) (paymentsqlc.PaymentTransaction, error) {
	txn, err := r.querier.GetPaymentByProviderTxnID(ctx, paymentsqlc.GetPaymentByProviderTxnIDParams{
		PtProvider:        provider,
		PtProviderTxnID: &providerTxnID,
	})
	if err != nil {
		return paymentsqlc.PaymentTransaction{}, fmt.Errorf("failed to get payment by provider txn id %s: %w", providerTxnID, err)
	}
	return txn, nil
}

func (r *sqlPaymentRepository) UpdateStatusAtomically(ctx context.Context, params paymentsqlc.UpdatePaymentStatusAtomicallyParams) (paymentsqlc.PaymentTransaction, error) {
	txn, err := r.querier.UpdatePaymentStatusAtomically(ctx, params)
	if err != nil {
		return paymentsqlc.PaymentTransaction{}, fmt.Errorf("failed to update payment status atomically: %w", err)
	}
	return txn, nil
}

func (r *sqlPaymentRepository) GetByOrderID(ctx context.Context, orderID int32) ([]paymentsqlc.PaymentTransaction, error) {
	txns, err := r.querier.GetPaymentTransactionsByOrderID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get payments by order id %d: %w", orderID, err)
	}
	return txns, nil
}

func (r *sqlPaymentRepository) GetByOrderUUID(ctx context.Context, orderUUID uuid.UUID) ([]paymentsqlc.PaymentTransaction, error) {
	pgUUID := pgtype.UUID{Bytes: orderUUID, Valid: true}
	txns, err := r.querier.GetPaymentTransactionsByOrderUUID(ctx, pgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get payments by order uuid %s: %w", orderUUID, err)
	}
	return txns, nil
}

func (r *sqlPaymentRepository) CreateRefund(ctx context.Context, params paymentsqlc.CreateRefundTransactionParams) (paymentsqlc.PaymentTransaction, error) {
	txn, err := r.querier.CreateRefundTransaction(ctx, params)
	if err != nil {
		return paymentsqlc.PaymentTransaction{}, fmt.Errorf("failed to create refund transaction: %w", err)
	}
	return txn, nil
}

func (r *sqlPaymentRepository) GetPendingOlderThan(ctx context.Context, olderThan time.Time, limit int32) ([]paymentsqlc.PaymentTransaction, error) {
	txns, err := r.querier.GetPendingTransactionsOlderThan(ctx, paymentsqlc.GetPendingTransactionsOlderThanParams{
		OlderThan: olderThan,
		Lim:       limit,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get pending transactions: %w", err)
	}
	return txns, nil
}

func (r *sqlPaymentRepository) UpdateProviderInfo(ctx context.Context, params paymentsqlc.UpdatePaymentProviderInfoParams) (paymentsqlc.PaymentTransaction, error) {
	txn, err := r.querier.UpdatePaymentProviderInfo(ctx, params)
	if err != nil {
		return paymentsqlc.PaymentTransaction{}, fmt.Errorf("failed to update provider info: %w", err)
	}
	return txn, nil
}

type sqlWebhookEventRepository struct {
	querier paymentsqlc.Querier
}

func NewSqlWebhookEventRepository(querier paymentsqlc.Querier) WebhookEventRepository {
	return &sqlWebhookEventRepository{querier: querier}
}

func (r *sqlWebhookEventRepository) Insert(ctx context.Context, params paymentsqlc.InsertWebhookEventParams) (paymentsqlc.WebhookEvent, error) {
	event, err := r.querier.InsertWebhookEvent(ctx, params)
	if err != nil {
		return paymentsqlc.WebhookEvent{}, fmt.Errorf("failed to insert webhook event: %w", err)
	}
	return event, nil
}

func (r *sqlWebhookEventRepository) MarkProcessed(ctx context.Context, eventID int32) error {
	if err := r.querier.MarkWebhookEventProcessed(ctx, eventID); err != nil {
		return fmt.Errorf("failed to mark webhook event %d as processed: %w", eventID, err)
	}
	return nil
}

type sqlOutboxRepository struct {
	querier paymentsqlc.Querier
}

func NewSqlOutboxRepository(querier paymentsqlc.Querier) OutboxRepository {
	return &sqlOutboxRepository{querier: querier}
}

func (r *sqlOutboxRepository) Create(ctx context.Context, aggregateType, aggregateID, eventType string, payload []byte) (paymentsqlc.OutboxEvent, error) {
	evt, err := r.querier.CreateOutboxEvent(ctx, paymentsqlc.CreateOutboxEventParams{
		OeAggregateType: aggregateType,
		OeAggregateID:   aggregateID,
		OeEventType:     eventType,
		OePayload:       payload,
	})
	if err != nil {
		return paymentsqlc.OutboxEvent{}, fmt.Errorf("failed to create outbox event: %w", err)
	}
	return evt, nil
}

func (r *sqlOutboxRepository) GetPending(ctx context.Context, limit int32) ([]paymentsqlc.OutboxEvent, error) {
	evts, err := r.querier.GetPendingOutboxEvents(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending outbox events: %w", err)
	}
	return evts, nil
}

func (r *sqlOutboxRepository) MarkPublished(ctx context.Context, eventID int64) error {
	if err := r.querier.MarkOutboxEventPublished(ctx, eventID); err != nil {
		return fmt.Errorf("failed to mark outbox event %d as published: %w", eventID, err)
	}
	return nil
}
