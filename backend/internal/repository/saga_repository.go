package repository

import (
	"context"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type sqlSagaRepository struct {
	db sqlc.Querier
}

func NewSqlSagaRepository(db sqlc.Querier) SagaRepository {
	return &sqlSagaRepository{db: db}
}

func (r *sqlSagaRepository) getQuerier(ctx context.Context, tx pgx.Tx) sqlc.Querier {
	if tx != nil {
		return sqlc.New(tx)
	}
	return db.GetQuerier(ctx, r.db)
}

func (r *sqlSagaRepository) Start(ctx context.Context, tx pgx.Tx, params sqlc.StartSagaParams) (sqlc.OrderSaga, error) {
	q := r.getQuerier(ctx, tx)
	return q.StartSaga(ctx, params)
}

func (r *sqlSagaRepository) Confirm(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (bool, error) {
	q := r.getQuerier(ctx, tx)
	rows, err := q.ConfirmSaga(ctx, orderUUID)
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *sqlSagaRepository) ClaimForCompensation(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, failureReason *string) (bool, error) {
	q := r.getQuerier(ctx, tx)
	rows, err := q.ClaimForCompensation(ctx, sqlc.ClaimForCompensationParams{
		OrderUuid:     orderUUID,
		FailureReason: failureReason,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *sqlSagaRepository) MarkFailed(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, failureReason *string) (bool, error) {
	q := r.getQuerier(ctx, tx)
	rows, err := q.MarkSagaFailed(ctx, sqlc.MarkSagaFailedParams{
		OrderUuid:     orderUUID,
		FailureReason: failureReason,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *sqlSagaRepository) ScheduleRetry(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, failureReason *string) (bool, error) {
	q := r.getQuerier(ctx, tx)
	rows, err := q.ScheduleSagaRetry(ctx, sqlc.ScheduleSagaRetryParams{
		OrderUuid:     orderUUID,
		FailureReason: failureReason,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *sqlSagaRepository) ClaimStale(ctx context.Context, tx pgx.Tx, limit int32) ([]sqlc.OrderSaga, error) {
	q := r.getQuerier(ctx, tx)
	return q.ClaimStaleSagas(ctx, limit)
}

func (r *sqlSagaRepository) UpdateStep(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, lastStep *string, snapshot []byte) (bool, error) {
	q := r.getQuerier(ctx, tx)
	rows, err := q.UpdateSagaStep(ctx, sqlc.UpdateSagaStepParams{
		OrderUuid: orderUUID,
		LastStep:  lastStep,
		Snapshot:  snapshot,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *sqlSagaRepository) GetByOrderUUID(ctx context.Context, orderUUID uuid.UUID) (sqlc.OrderSaga, error) {
	return r.db.GetSagaByOrderUUID(ctx, orderUUID)
}

func (r *sqlSagaRepository) GetByIdempotencyKey(ctx context.Context, userUUID uuid.UUID, idempotencyKey string) (sqlc.OrderSaga, error) {
	return r.db.GetSagaByIdempotencyKey(ctx, sqlc.GetSagaByIdempotencyKeyParams{
		UserUuid:       userUUID,
		IdempotencyKey: idempotencyKey,
	})
}
