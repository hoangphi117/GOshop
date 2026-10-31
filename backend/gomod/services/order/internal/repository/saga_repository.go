package repository

import (
	"context"

	ordersqlc "goshop/services/order/internal/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type sqlSagaRepository struct {
	db ordersqlc.Querier
}

func NewSqlSagaRepository(db ordersqlc.Querier) SagaRepository {
	return &sqlSagaRepository{db: db}
}

func (r *sqlSagaRepository) getQuerier(tx pgx.Tx) ordersqlc.Querier {
	if tx != nil {
		return ordersqlc.New(tx)
	}
	return r.db
}

func (r *sqlSagaRepository) Start(ctx context.Context, tx pgx.Tx, params ordersqlc.StartSagaParams) (ordersqlc.OrderSaga, error) {
	return r.getQuerier(tx).StartSaga(ctx, params)
}

func (r *sqlSagaRepository) Confirm(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (bool, error) {
	rows, err := r.getQuerier(tx).ConfirmSaga(ctx, orderUUID)
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *sqlSagaRepository) ClaimForCompensation(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, failureReason *string) (bool, error) {
	rows, err := r.getQuerier(tx).ClaimForCompensation(ctx, ordersqlc.ClaimForCompensationParams{
		OrderUuid:     orderUUID,
		FailureReason: failureReason,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *sqlSagaRepository) MarkFailed(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, failureReason *string) (bool, error) {
	rows, err := r.getQuerier(tx).MarkSagaFailed(ctx, ordersqlc.MarkSagaFailedParams{
		OrderUuid:     orderUUID,
		FailureReason: failureReason,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *sqlSagaRepository) ScheduleRetry(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, failureReason *string) (bool, error) {
	rows, err := r.getQuerier(tx).ScheduleSagaRetry(ctx, ordersqlc.ScheduleSagaRetryParams{
		OrderUuid:     orderUUID,
		FailureReason: failureReason,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *sqlSagaRepository) ClaimStale(ctx context.Context, tx pgx.Tx, limit int32) ([]ordersqlc.OrderSaga, error) {
	return r.getQuerier(tx).ClaimStaleSagas(ctx, limit)
}

func (r *sqlSagaRepository) UpdateStep(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, lastStep *string, snapshot []byte) (bool, error) {
	rows, err := r.getQuerier(tx).UpdateSagaStep(ctx, ordersqlc.UpdateSagaStepParams{
		OrderUuid: orderUUID,
		LastStep:  lastStep,
		Snapshot:  snapshot,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *sqlSagaRepository) GetByOrderUUID(ctx context.Context, orderUUID uuid.UUID) (ordersqlc.OrderSaga, error) {
	return r.db.GetSagaByOrderUUID(ctx, orderUUID)
}

func (r *sqlSagaRepository) GetByIdempotencyKey(ctx context.Context, userUUID uuid.UUID, idempotencyKey string) (ordersqlc.OrderSaga, error) {
	return r.db.GetSagaByIdempotencyKey(ctx, ordersqlc.GetSagaByIdempotencyKeyParams{
		UserUuid:       userUUID,
		IdempotencyKey: idempotencyKey,
	})
}
