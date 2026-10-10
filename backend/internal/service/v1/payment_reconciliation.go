package v1service

import (
	"context"
	"errors"
	"time"

	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	"goshop/internal/utils"
	"goshop/pkg/logger"
	"goshop/pkg/payment"
	"goshop/pkg/rabbitmq"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ReconcilePendingPayments periodically queries gateways for pending transactions and marks stale transactions as Expired.
func (s *paymentService) ReconcilePendingPayments(ctx context.Context) error {
	startTime := time.Now()

	lockKey := "lock:payment:reconciliation"
	if s.cache != nil {
		acquired, err := s.cache.SetNX(lockKey, "locked", 4*time.Minute)
		if err != nil {
			logger.Log.Error().Err(err).Msg("Reconciliation: Failed to acquire distributed lock in Redis")
			return err
		}
		if !acquired {
			logger.Log.Info().Msg("Reconciliation: Another instance is currently running reconciliation, skipping this cycle")
			return nil
		}
		defer func() {
			if delErr := s.cache.Del(lockKey); delErr != nil {
				logger.Log.Warn().Err(delErr).Str("key", lockKey).Msg("Reconciliation: Failed to release Redis lock")
			}
		}()
	}

	olderThan := time.Now().Add(-15 * time.Minute)

	pendingTxns, err := s.paymentRepo.GetPendingTransactionsOlderThan(ctx, nil, sqlc.GetPendingTransactionsOlderThanParams{
		OlderThan: olderThan,
		Lim:       50,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Reconciliation: Failed to fetch pending payment transactions")
		return utils.WrapError(utils.ErrCodeInternal, "failed to fetch pending transactions for reconciliation", err)
	}

	if len(pendingTxns) == 0 {
		return nil
	}

	logger.Log.Info().
		Int("count", len(pendingTxns)).
		Msg("Reconciliation: Processing expired/pending payment transactions")

	successCount := 0
	expiredCount := 0

	for _, txn := range pendingTxns {
		provider, exists := s.providers[txn.PtProvider]
		if !exists {
			logger.Log.Warn().
				Str("provider", txn.PtProvider).
				Str("pt_uuid", txn.PtUuid.String()).
				Msg("Reconciliation: Provider not supported for stale pending transaction, marking as Expired")

			tx, err := db.BeginTx(ctx)
			if err != nil {
				logger.Log.Error().Err(err).Msg("Reconciliation: Failed to begin db transaction for unsupported provider")
				continue
			}

			_, err = s.paymentRepo.UpdateStatusAtomically(ctx, tx, sqlc.UpdatePaymentStatusAtomicallyParams{
				PtUuid:              txn.PtUuid,
				NewStatus:           PaymentStatusExpired,
				AllowedFromStatuses: []int32{PaymentStatusPending},
			})
			if err != nil {
				_ = tx.Rollback(ctx)
				if !errors.Is(err, pgx.ErrNoRows) {
					logger.Log.Error().Err(err).Str("pt_uuid", txn.PtUuid.String()).Msg("Reconciliation: Failed to mark unsupported provider transaction as Expired")
				}
				continue
			}

			if s.outboxRepo != nil {
				expEvent := rabbitmq.NewPaymentExpiredEvent(
					uuid.New().String(),
					txn.PtUuid.String(),
					txn.OrderUuid.String(),
					txn.OrderNumber,
					txn.UserUuid.String(),
					txn.PtProvider,
				)
				if s.outboxRepo != nil {
					if _, err := s.outboxRepo.Insert(
						ctx, tx, "payment", txn.PtUuid.String(),
						rabbitmq.QueuePaymentExpired, expEvent,
					); err != nil {
						_ = tx.Rollback(ctx)
						logger.Log.Error().Err(err).Msg("Failed to insert payment outbox event")
						continue
					}
				}
			}

			if err := tx.Commit(ctx); err != nil {
				logger.Log.Error().Err(err).Msg("Reconciliation: Failed to commit Expired update for unsupported provider")
				continue
			}

			expiredCount++
			s.publishPaymentExpired(ctx, txn)
			continue
		}

		queryOrderID := txn.OrderNumber
		if txn.PtTxnRef != nil && *txn.PtTxnRef != "" {
			queryOrderID = *txn.PtTxnRef
		} else if txn.PtProviderTxnID != nil && *txn.PtProviderTxnID != "" {
			queryOrderID = *txn.PtProviderTxnID
		}

		status, err := provider.QueryTransactionStatus(ctx, queryOrderID)
		if err != nil {
			logger.Log.Warn().
				Err(err).
				Str("provider", txn.PtProvider).
				Str("query_order_id", queryOrderID).
				Msg("Reconciliation: Failed to query transaction status from gateway")
			continue
		}

		tx, err := db.BeginTx(ctx)
		if err != nil {
			logger.Log.Error().Err(err).Msg("Reconciliation: Failed to begin db transaction")
			continue
		}

		switch status {
		case payment.TxnStatusSuccess:
			_, err = s.paymentRepo.UpdateStatusAtomically(ctx, tx, sqlc.UpdatePaymentStatusAtomicallyParams{
				PtUuid:              txn.PtUuid,
				NewStatus:           PaymentStatusPaid,
				AllowedFromStatuses: []int32{PaymentStatusPending},
			})
			if err != nil {
				_ = tx.Rollback(ctx)
				if errors.Is(err, pgx.ErrNoRows) {
					logger.Log.Info().
						Str("pt_uuid", txn.PtUuid.String()).
						Msg("Reconciliation: Transaction was already updated concurrently (idempotent skip)")
				} else {
					logger.Log.Error().
						Err(err).
						Str("pt_uuid", txn.PtUuid.String()).
						Msg("Reconciliation: DB error updating payment status to Paid")
				}
				continue
			}

			if s.outboxRepo != nil {
				providerTxnID := queryOrderID
				if txn.PtProviderTxnID != nil && *txn.PtProviderTxnID != "" {
					providerTxnID = *txn.PtProviderTxnID
				}
				event := rabbitmq.NewPaymentSucceededEvent(
					uuid.New().String(),
					txn.PtUuid.String(),
					txn.OrderUuid.String(),
					txn.OrderNumber,
					txn.UserUuid.String(),
					txn.PtProvider,
					providerTxnID,
					utils.NumericToFloat(txn.PtAmount),
				)
				if s.outboxRepo != nil {
					if _, err := s.outboxRepo.Insert(
						ctx, tx, "payment", txn.PtUuid.String(),
						rabbitmq.QueuePaymentSucceeded, event,
					); err != nil {
						_ = tx.Rollback(ctx)
						logger.Log.Error().Err(err).Msg("Failed to insert payment outbox event")
						continue
					}
				}
			}

			if err := tx.Commit(ctx); err != nil {
				logger.Log.Error().Err(err).Msg("Reconciliation: Failed to commit Paid update")
				continue
			}

			successCount++
			providerTxnID := queryOrderID
			if txn.PtProviderTxnID != nil && *txn.PtProviderTxnID != "" {
				providerTxnID = *txn.PtProviderTxnID
			}
			if s.outboxRepo == nil {
				s.publishPaymentSucceeded(ctx, txn.PtUuid.String(), txn.OrderUuid.String(), txn.OrderNumber, txn.UserUuid.String(), txn.PtProvider, providerTxnID, utils.NumericToFloat(txn.PtAmount))
			}
			logger.Log.Info().
				Str("order_number", txn.OrderNumber).
				Str("pt_uuid", txn.PtUuid.String()).
				Msg("Reconciliation: Successfully recovered missed paid transaction")

		case payment.TxnStatusFailed, payment.TxnStatusExpired, payment.TxnStatusPending:
			_, err = s.paymentRepo.UpdateStatusAtomically(ctx, tx, sqlc.UpdatePaymentStatusAtomicallyParams{
				PtUuid:              txn.PtUuid,
				NewStatus:           PaymentStatusExpired,
				AllowedFromStatuses: []int32{PaymentStatusPending},
			})
			if err != nil {
				_ = tx.Rollback(ctx)
				if errors.Is(err, pgx.ErrNoRows) {
					logger.Log.Info().
						Str("pt_uuid", txn.PtUuid.String()).
						Msg("Reconciliation: Transaction was already updated concurrently before expiring (idempotent skip)")
				} else {
					logger.Log.Error().
						Err(err).
						Str("pt_uuid", txn.PtUuid.String()).
						Msg("Reconciliation: DB error expiring pending transaction")
				}
				continue
			}

			if s.outboxRepo != nil {
				expEvent := rabbitmq.NewPaymentExpiredEvent(
					uuid.New().String(),
					txn.PtUuid.String(),
					txn.OrderUuid.String(),
					txn.OrderNumber,
					txn.UserUuid.String(),
					txn.PtProvider,
				)
				if s.outboxRepo != nil {
					if _, err := s.outboxRepo.Insert(
						ctx, tx, "payment", txn.PtUuid.String(),
						rabbitmq.QueuePaymentExpired, expEvent,
					); err != nil {
						_ = tx.Rollback(ctx)
						logger.Log.Error().Err(err).Msg("Failed to insert payment outbox event")
						continue
					}
				}
			}

			if err := tx.Commit(ctx); err != nil {
				logger.Log.Error().Err(err).Msg("Reconciliation: Failed to commit Expired update")
				continue
			}

			expiredCount++
			if s.outboxRepo == nil {
				s.publishPaymentExpired(ctx, txn)
			}
			logger.Log.Info().
				Str("pt_uuid", txn.PtUuid.String()).
				Str("order_number", txn.OrderNumber).
				Msg("Reconciliation: Marked abandoned payment transaction as Expired and emitted PaymentExpired event")
		}
	}

	if successCount > 0 && s.cache != nil {
		if clearErr := s.cache.Clear("orders:*"); clearErr != nil {
			logger.Log.Warn().Err(clearErr).Msg("Reconciliation: Failed to clear orders cache")
		}
	}

	logger.Log.Info().
		Int("recovered_paid", successCount).
		Int("marked_expired", expiredCount).
		Int64("duration_ms", time.Since(startTime).Milliseconds()).
		Msg("Reconciliation: Job finished successfully")

	return nil
}

func (s *paymentService) publishPaymentExpired(ctx context.Context, txn sqlc.GetPendingTransactionsOlderThanRow) {
	if s.rabbitmq == nil {
		return
	}
	event := rabbitmq.NewPaymentExpiredEvent(
		uuid.New().String(),
		txn.PtUuid.String(),
		txn.OrderUuid.String(),
		txn.OrderNumber,
		txn.UserUuid.String(),
		txn.PtProvider,
	)
	if err := s.rabbitmq.Publish(ctx, rabbitmq.QueuePaymentExpired, event); err != nil {
		logger.Log.Error().
			Err(err).
			Str("pt_uuid", txn.PtUuid.String()).
			Str("order_number", txn.OrderNumber).
			Msg("Failed to publish payment.expired event")
	}
}

func (s *paymentService) publishPaymentSucceeded(ctx context.Context, ptUUID, orderUUID, orderNumber, userUUID, provider, providerTxnID string, amount float64) {
	if s.rabbitmq == nil {
		return
	}
	event := rabbitmq.NewPaymentSucceededEvent(
		uuid.New().String(),
		ptUUID,
		orderUUID,
		orderNumber,
		userUUID,
		provider,
		providerTxnID,
		amount,
	)
	if err := s.rabbitmq.Publish(ctx, rabbitmq.QueuePaymentSucceeded, event); err != nil {
		logger.Log.Error().
			Err(err).
			Str("order_uuid", orderUUID).
			Str("order_number", orderNumber).
			Msg("Failed to publish payment.succeeded event")
	}
}

func (s *paymentService) publishPaymentFailed(ctx context.Context, ptUUID, orderUUID, orderNumber, userUUID, provider, reason string) {
	if s.rabbitmq == nil {
		return
	}
	event := rabbitmq.NewPaymentFailedEvent(
		uuid.New().String(),
		ptUUID,
		orderUUID,
		orderNumber,
		userUUID,
		provider,
		reason,
	)
	if err := s.rabbitmq.Publish(ctx, rabbitmq.QueuePaymentFailed, event); err != nil {
		logger.Log.Error().
			Err(err).
			Str("order_uuid", orderUUID).
			Str("order_number", orderNumber).
			Msg("Failed to publish payment.failed event")
	}
}
