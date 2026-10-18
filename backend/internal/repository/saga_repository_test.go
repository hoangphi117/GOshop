package repository_test

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"goshop/internal/db/sqlc"
	"goshop/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestPool(t *testing.T) *pgxpool.Pool {
	connStr := os.Getenv("TEST_DB_URL")
	if connStr == "" {
		connStr = "postgres://postgres:super@password@localhost:5432/shopping?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Skipf("Skipping test: cannot connect to postgres at %s: %v", connStr, err)
		return nil
	}
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("Skipping test: ping failed: %v", err)
		return nil
	}
	return pool
}

func cleanupSaga(t *testing.T, pool *pgxpool.Pool, orderUUID uuid.UUID) {
	_, _ = pool.Exec(context.Background(), "DELETE FROM order_sagas WHERE order_uuid = $1", orderUUID)
}

func TestSagaRepository_Start_Idempotency(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	repo := repository.NewSqlSagaRepository(sqlc.New(pool))

	orderUUID := uuid.New()
	userUUID := uuid.New()
	idemKey := "idem-" + uuid.New().String()
	defer cleanupSaga(t, pool, orderUUID)

	payload := json.RawMessage(`{"items":[{"product_id":1,"qty":2}]}`)
	step := "START"

	// 1. Initial start succeeds
	saga, err := repo.Start(ctx, nil, sqlc.StartSagaParams{
		OrderUuid:      orderUUID,
		UserUuid:       userUUID,
		IdempotencyKey: idemKey,
		LastStep:       &step,
		Payload:        payload,
	})
	require.NoError(t, err)
	assert.Equal(t, orderUUID, saga.OrderUuid)
	assert.Equal(t, "STARTED", saga.State)

	// 2. Second start with same userUUID and idemKey conflicts (ON CONFLICT DO NOTHING -> ErrNoRows)
	orderUUID2 := uuid.New()
	defer cleanupSaga(t, pool, orderUUID2)

	_, err2 := repo.Start(ctx, nil, sqlc.StartSagaParams{
		OrderUuid:      orderUUID2,
		UserUuid:       userUUID,
		IdempotencyKey: idemKey,
		LastStep:       &step,
		Payload:        payload,
	})
	assert.ErrorIs(t, err2, pgx.ErrNoRows)

	// 3. Query existing saga by idempotency key
	existing, err := repo.GetByIdempotencyKey(ctx, userUUID, idemKey)
	require.NoError(t, err)
	assert.Equal(t, orderUUID, existing.OrderUuid)
}

func TestSagaRepository_CAS_ConcurrentConfirm(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	repo := repository.NewSqlSagaRepository(sqlc.New(pool))

	orderUUID := uuid.New()
	userUUID := uuid.New()
	idemKey := "idem-" + uuid.New().String()
	defer cleanupSaga(t, pool, orderUUID)

	payload := json.RawMessage(`{"test":true}`)
	_, err := repo.Start(ctx, nil, sqlc.StartSagaParams{
		OrderUuid:      orderUUID,
		UserUuid:       userUUID,
		IdempotencyKey: idemKey,
		Payload:        payload,
	})
	require.NoError(t, err)

	// Concurrency test: 10 goroutines racing to confirm
	const workers = 10
	var winCount int64
	var loseCount int64
	var wg sync.WaitGroup

	startBarrier := make(chan struct{})

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startBarrier

			won, err := repo.Confirm(ctx, nil, orderUUID)
			if err != nil {
				return
			}
			if won {
				atomic.AddInt64(&winCount, 1)
			} else {
				atomic.AddInt64(&loseCount, 1)
			}
		}()
	}

	close(startBarrier)
	wg.Wait()

	// Exactly 1 winner in CAS race
	assert.Equal(t, int64(1), winCount, "Exactly one worker should win CAS confirm")
	assert.Equal(t, int64(workers-1), loseCount, "All other workers should see 0 rows affected")

	// Final state must be CONFIRMED
	updated, err := repo.GetByOrderUUID(ctx, orderUUID)
	require.NoError(t, err)
	assert.Equal(t, "CONFIRMED", updated.State)
}

func TestSagaRepository_CAS_ConcurrentCompensation(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	repo := repository.NewSqlSagaRepository(sqlc.New(pool))

	orderUUID := uuid.New()
	userUUID := uuid.New()
	idemKey := "idem-" + uuid.New().String()
	defer cleanupSaga(t, pool, orderUUID)

	payload := json.RawMessage(`{"test":true}`)
	_, err := repo.Start(ctx, nil, sqlc.StartSagaParams{
		OrderUuid:      orderUUID,
		UserUuid:       userUUID,
		IdempotencyKey: idemKey,
		Payload:        payload,
	})
	require.NoError(t, err)

	// Concurrency test: 8 workers racing to claim compensation
	const workers = 8
	var winCount int64
	var loseCount int64
	var wg sync.WaitGroup

	startBarrier := make(chan struct{})
	reason := "payment_declined"

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startBarrier

			won, err := repo.ClaimForCompensation(ctx, nil, orderUUID, &reason)
			if err != nil {
				return
			}
			if won {
				atomic.AddInt64(&winCount, 1)
			} else {
				atomic.AddInt64(&loseCount, 1)
			}
		}()
	}

	close(startBarrier)
	wg.Wait()

	assert.Equal(t, int64(1), winCount, "Exactly one worker should claim compensation")
	assert.Equal(t, int64(workers-1), loseCount)

	updated, err := repo.GetByOrderUUID(ctx, orderUUID)
	require.NoError(t, err)
	assert.Equal(t, "COMPENSATING", updated.State)
	assert.Equal(t, &reason, updated.FailureReason)
}

func TestSagaRepository_CAS_InvalidTransitions(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	repo := repository.NewSqlSagaRepository(sqlc.New(pool))

	orderUUID := uuid.New()
	userUUID := uuid.New()
	idemKey := "idem-" + uuid.New().String()
	defer cleanupSaga(t, pool, orderUUID)

	payload := json.RawMessage(`{"test":true}`)
	_, err := repo.Start(ctx, nil, sqlc.StartSagaParams{
		OrderUuid:      orderUUID,
		UserUuid:       userUUID,
		IdempotencyKey: idemKey,
		Payload:        payload,
	})
	require.NoError(t, err)

	// Cannot MarkFailed when state is STARTED
	reason := "some error"
	failedWon, err := repo.MarkFailed(ctx, nil, orderUUID, &reason)
	require.NoError(t, err)
	assert.False(t, failedWon, "MarkFailed should fail when state is STARTED")

	// Confirm saga
	confirmWon, err := repo.Confirm(ctx, nil, orderUUID)
	require.NoError(t, err)
	assert.True(t, confirmWon)

	// Cannot ClaimForCompensation after CONFIRMED
	compWon, err := repo.ClaimForCompensation(ctx, nil, orderUUID, &reason)
	require.NoError(t, err)
	assert.False(t, compWon, "ClaimForCompensation should fail when state is CONFIRMED")
}

func TestSagaRepository_ScheduleRetry_And_MarkFailed(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	repo := repository.NewSqlSagaRepository(sqlc.New(pool))

	orderUUID := uuid.New()
	userUUID := uuid.New()
	idemKey := "idem-" + uuid.New().String()
	defer cleanupSaga(t, pool, orderUUID)

	payload := json.RawMessage(`{"test":true}`)
	_, err := repo.Start(ctx, nil, sqlc.StartSagaParams{
		OrderUuid:      orderUUID,
		UserUuid:       userUUID,
		IdempotencyKey: idemKey,
		Payload:        payload,
	})
	require.NoError(t, err)

	// Claim compensation
	claimReason := "inventory_release_failed"
	claimed, err := repo.ClaimForCompensation(ctx, nil, orderUUID, &claimReason)
	require.NoError(t, err)
	require.True(t, claimed)

	// Schedule Retry #1
	retryReason := "retry_connection_timeout"
	retried, err := repo.ScheduleRetry(ctx, nil, orderUUID, &retryReason)
	require.NoError(t, err)
	assert.True(t, retried)

	saga, err := repo.GetByOrderUUID(ctx, orderUUID)
	require.NoError(t, err)
	assert.Equal(t, int32(1), saga.Attempts)
	assert.Equal(t, &retryReason, saga.FailureReason)
	assert.True(t, saga.NextRetryAt.Valid && saga.NextRetryAt.Time.After(time.Now()))

	// Finally Mark Failed
	finalReason := "max_retries_exhausted"
	failed, err := repo.MarkFailed(ctx, nil, orderUUID, &finalReason)
	require.NoError(t, err)
	assert.True(t, failed)

	finalSaga, err := repo.GetByOrderUUID(ctx, orderUUID)
	require.NoError(t, err)
	assert.Equal(t, "FAILED", finalSaga.State)
	assert.Equal(t, &finalReason, finalSaga.FailureReason)
}
