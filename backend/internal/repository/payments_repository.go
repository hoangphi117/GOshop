package repository

import (
	"context"
	"goshop/internal/db/sqlc"

	"github.com/jackc/pgx/v5"
)

type SqlPaymentRepository struct {
	db sqlc.Querier
}

func NewSqlPaymentRepository(db sqlc.Querier) PaymentRepository {
	return &SqlPaymentRepository{
		db: db,
	}
}

func (r *SqlPaymentRepository) getQuerier(tx pgx.Tx) sqlc.Querier {
	if tx != nil {
		return sqlc.New(tx)
	}

	return r.db
}

func (r *SqlPaymentRepository) Create(ctx context.Context, tx pgx.Tx, input sqlc.CreatePaymentTransactionParams) (sqlc.PaymentTransaction, error) {
	return r.getQuerier(tx).CreatePaymentTransaction(ctx, input)
}

func (r *SqlPaymentRepository) GetByIdempotencyKey(ctx context.Context, tx pgx.Tx, idempotencyKey string) (sqlc.PaymentTransaction, error) {
	return r.getQuerier(tx).GetPaymentByIdempotencyKey(ctx, idempotencyKey)
}

func (r *SqlPaymentRepository) GetByTxnRef(ctx context.Context, tx pgx.Tx, provider, txnRef string) (sqlc.PaymentTransaction, error) {
	return r.getQuerier(tx).GetPaymentByTxnRef(ctx, sqlc.GetPaymentByTxnRefParams{
		PtProvider: provider,
		PtTxnRef:   &txnRef,
	})
}

func (r *SqlPaymentRepository) GetByProviderTxnID(ctx context.Context, tx pgx.Tx, provider, providerTxnID string) (sqlc.PaymentTransaction, error) {
	return r.getQuerier(tx).GetPaymentByProviderTxnID(ctx, sqlc.GetPaymentByProviderTxnIDParams{
		PtProvider:      provider,
		PtProviderTxnID: &providerTxnID,
	})
}

func (r *SqlPaymentRepository) UpdateStatusAtomically(ctx context.Context, tx pgx.Tx, params sqlc.UpdatePaymentStatusAtomicallyParams) (sqlc.PaymentTransaction, error) {
	return r.getQuerier(tx).UpdatePaymentStatusAtomically(ctx, params)
}

func (r *SqlPaymentRepository) UpdateProviderInfo(ctx context.Context, tx pgx.Tx, params sqlc.UpdatePaymentProviderInfoParams) (sqlc.PaymentTransaction, error) {
	return r.getQuerier(tx).UpdatePaymentProviderInfo(ctx, params)
}

func (r *SqlPaymentRepository) GetByOrderID(ctx context.Context, tx pgx.Tx, orderID int32) ([]sqlc.PaymentTransaction, error) {
	return r.getQuerier(tx).GetPaymentTransactionsByOrderID(ctx, orderID)
}

func (r *SqlPaymentRepository) CreateRefund(ctx context.Context, tx pgx.Tx, params sqlc.CreateRefundTransactionParams) (sqlc.PaymentTransaction, error) {
	return r.getQuerier(tx).CreateRefundTransaction(ctx, params)
}

func (r *SqlPaymentRepository) GetPendingTransactionsOlderThan(ctx context.Context, tx pgx.Tx, params sqlc.GetPendingTransactionsOlderThanParams) ([]sqlc.GetPendingTransactionsOlderThanRow, error) {
	return r.getQuerier(tx).GetPendingTransactionsOlderThan(ctx, params)
}
