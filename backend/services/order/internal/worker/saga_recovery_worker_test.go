package worker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"goshop/services/order/internal/db/sqlc"
	v1dto "goshop/services/order/internal/dto"
	orderservice "goshop/services/order/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSagaOrchestrator struct {
	executeCheckoutSagaFunc func(ctx context.Context, userUUID uuid.UUID, idempotencyKey string, input v1dto.CreateOrderInput) (*v1dto.OrderDTO, error)
	compensateSagaFunc      func(ctx context.Context, orderUUID uuid.UUID, failureReason string) error
	getSagaStatusFunc       func(ctx context.Context, orderUUID uuid.UUID) (*sqlc.OrderSaga, error)
}

func (m *mockSagaOrchestrator) CreateOrder(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateOrderInput) (*v1dto.OrderDTO, error) {
	return m.ExecuteCheckoutSaga(ctx, userUUID, input.IdempotencyKey, input)
}

func (m *mockSagaOrchestrator) ExecuteCheckoutSaga(ctx context.Context, userUUID uuid.UUID, idempotencyKey string, input v1dto.CreateOrderInput) (*v1dto.OrderDTO, error) {
	if m.executeCheckoutSagaFunc != nil {
		return m.executeCheckoutSagaFunc(ctx, userUUID, idempotencyKey, input)
	}
	return nil, nil
}

func (m *mockSagaOrchestrator) CompensateSaga(ctx context.Context, orderUUID uuid.UUID, failureReason string) error {
	if m.compensateSagaFunc != nil {
		return m.compensateSagaFunc(ctx, orderUUID, failureReason)
	}
	return nil
}

func (m *mockSagaOrchestrator) GetSagaStatus(ctx context.Context, orderUUID uuid.UUID) (*sqlc.OrderSaga, error) {
	if m.getSagaStatusFunc != nil {
		return m.getSagaStatusFunc(ctx, orderUUID)
	}
	return nil, nil
}

func (m *mockSagaOrchestrator) WithShippingValidator(v orderservice.OrderShippingValidatorPort) orderservice.OrderSagaOrchestrator {
	return m
}

var _ orderservice.OrderSagaOrchestrator = (*mockSagaOrchestrator)(nil)

func TestSagaRecoveryWorker_NilDependencies(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	w1 := NewSagaRecoveryWorker(nil, nil, 0, 0, &logger)
	count, err := w1.ProcessBatch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Start returns immediately when deps are nil
	w1.Start(ctx)

	w2 := NewSagaRecoveryWorker(&MockSagaRepository{}, nil, 0, 0, &logger)
	count, err = w2.ProcessBatch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestSagaRecoveryWorker_ProcessBatch_NoStaleSagas(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	repo := &MockSagaRepository{
		ClaimStaleFunc: func(ctx context.Context, tx pgx.Tx, limit int32) ([]sqlc.OrderSaga, error) {
			assert.Equal(t, int32(20), limit)
			return []sqlc.OrderSaga{}, nil
		},
	}
	orch := &mockSagaOrchestrator{}

	w := NewSagaRecoveryWorker(repo, orch, time.Second, 20, &logger)
	count, err := w.ProcessBatch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestSagaRecoveryWorker_ProcessBatch_ClaimError(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	repo := &MockSagaRepository{
		ClaimStaleFunc: func(ctx context.Context, tx pgx.Tx, limit int32) ([]sqlc.OrderSaga, error) {
			return nil, errors.New("db claim failure")
		},
	}
	orch := &mockSagaOrchestrator{}

	w := NewSagaRecoveryWorker(repo, orch, time.Second, 20, &logger)
	count, err := w.ProcessBatch(ctx)
	require.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "db claim failure")
}

func TestSagaRecoveryWorker_ProcessBatch_CompensatesAll(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	order1 := uuid.New()
	order2 := uuid.New()
	customReason := "Cart lock timed out during checkout"

	repo := &MockSagaRepository{
		ClaimStaleFunc: func(ctx context.Context, tx pgx.Tx, limit int32) ([]sqlc.OrderSaga, error) {
			return []sqlc.OrderSaga{
				{
					OrderUuid:     order1,
					FailureReason: &customReason,
					Attempts:      1,
				},
				{
					OrderUuid:     order2,
					FailureReason: nil,
					Attempts:      2,
				},
			}, nil
		},
	}

	compensated := make(map[uuid.UUID]string)
	orch := &mockSagaOrchestrator{
		compensateSagaFunc: func(ctx context.Context, orderUUID uuid.UUID, failureReason string) error {
			compensated[orderUUID] = failureReason
			return nil
		},
	}

	w := NewSagaRecoveryWorker(repo, orch, time.Second, 20, &logger)
	count, err := w.ProcessBatch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	assert.Equal(t, customReason, compensated[order1])
	assert.Equal(t, "Stale saga recovered by background compensation worker", compensated[order2])
}

func TestSagaRecoveryWorker_ProcessBatch_CompensateErrorContinues(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	order1 := uuid.New()
	order2 := uuid.New()

	repo := &MockSagaRepository{
		ClaimStaleFunc: func(ctx context.Context, tx pgx.Tx, limit int32) ([]sqlc.OrderSaga, error) {
			return []sqlc.OrderSaga{
				{OrderUuid: order1, Attempts: 1},
				{OrderUuid: order2, Attempts: 1},
			}, nil
		},
	}

	var called []uuid.UUID
	orch := &mockSagaOrchestrator{
		compensateSagaFunc: func(ctx context.Context, orderUUID uuid.UUID, failureReason string) error {
			called = append(called, orderUUID)
			if orderUUID == order1 {
				return errors.New("temporary stock lock error")
			}
			return nil
		},
	}

	w := NewSagaRecoveryWorker(repo, orch, time.Second, 20, &logger)
	count, err := w.ProcessBatch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.ElementsMatch(t, []uuid.UUID{order1, order2}, called)
}

func TestSagaRecoveryWorker_StartAndCancel(t *testing.T) {
	logger := zerolog.Nop()

	var mu sync.Mutex
	callCount := 0

	repo := &MockSagaRepository{
		ClaimStaleFunc: func(ctx context.Context, tx pgx.Tx, limit int32) ([]sqlc.OrderSaga, error) {
			mu.Lock()
			callCount++
			mu.Unlock()
			return []sqlc.OrderSaga{}, nil
		},
	}
	orch := &mockSagaOrchestrator{}

	w := NewSagaRecoveryWorker(repo, orch, 10*time.Millisecond, 10, &logger)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		w.Start(ctx)
		close(done)
	}()

	time.Sleep(35 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Clean exit
	case <-time.After(500 * time.Millisecond):
		t.Fatal("worker did not shut down on ctx cancel")
	}

	mu.Lock()
	assert.GreaterOrEqual(t, callCount, 1)
	mu.Unlock()
}
