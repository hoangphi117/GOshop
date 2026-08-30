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
