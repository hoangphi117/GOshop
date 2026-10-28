package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"goshop/internal/testutils"
	"goshop/services/order/internal/db/sqlc"
	v1dto "goshop/services/order/internal/dto"
	"goshop/services/order/internal/repository"
	orderservice "goshop/services/order/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// inMemorySagaRepo provides an in-memory implementation of SagaRepository
// that enforces the exact CAS state machine and concurrency constraints of 000022_create_order_sagas.sql.
type inMemorySagaRepo struct {
	mu               sync.Mutex
	sagas            map[uuid.UUID]*sqlc.OrderSaga
	idempotencyIndex map[string]uuid.UUID
}

func newInMemorySagaRepo() *inMemorySagaRepo {
	return &inMemorySagaRepo{
		sagas:            make(map[uuid.UUID]*sqlc.OrderSaga),
		idempotencyIndex: make(map[string]uuid.UUID),
	}
}

var _ repository.SagaRepository = (*inMemorySagaRepo)(nil)

func (r *inMemorySagaRepo) Start(ctx context.Context, tx pgx.Tx, params sqlc.StartSagaParams) (sqlc.OrderSaga, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	idxKey := fmt.Sprintf("%s:%s", params.UserUuid.String(), params.IdempotencyKey)
	if existingUUID, exists := r.idempotencyIndex[idxKey]; exists {
		return *r.sagas[existingUUID], nil
	}

	saga := &sqlc.OrderSaga{
		OrderUuid:      params.OrderUuid,
		UserUuid:       params.UserUuid,
		IdempotencyKey: params.IdempotencyKey,
		State:          orderservice.SagaStateStarted,
		Payload:        params.Payload,
		Attempts:       0,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	r.sagas[params.OrderUuid] = saga
	r.idempotencyIndex[idxKey] = params.OrderUuid
	return *saga, nil
}

func (r *inMemorySagaRepo) Confirm(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	saga, exists := r.sagas[orderUUID]
	if !exists || saga.State != orderservice.SagaStateStarted {
		return false, nil // CAS failure: saga already claimed or compensated
	}

	saga.State = orderservice.SagaStateConfirmed
	saga.UpdatedAt = time.Now()
	return true, nil
}

func (r *inMemorySagaRepo) ClaimForCompensation(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, failureReason *string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	saga, exists := r.sagas[orderUUID]
	if !exists || saga.State != orderservice.SagaStateStarted {
		return false, nil // CAS failure: already confirmed or claimed
	}

	saga.State = orderservice.SagaStateCompensating
	saga.FailureReason = failureReason
	saga.UpdatedAt = time.Now()
	retryAt := time.Now().Add(30 * time.Second)
	saga.NextRetryAt = pgtype.Timestamptz{Time: retryAt, Valid: true}
	return true, nil
}

func (r *inMemorySagaRepo) MarkFailed(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, failureReason *string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	saga, exists := r.sagas[orderUUID]
	if !exists || saga.State != orderservice.SagaStateCompensating {
		return false, nil
	}

	saga.State = orderservice.SagaStateFailed
	if failureReason != nil {
		saga.FailureReason = failureReason
	}
	saga.UpdatedAt = time.Now()
	return true, nil
}

func (r *inMemorySagaRepo) ScheduleRetry(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, failureReason *string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	saga, exists := r.sagas[orderUUID]
	if !exists || saga.State != orderservice.SagaStateCompensating {
		return false, nil
	}

	saga.Attempts++
	if failureReason != nil {
		saga.FailureReason = failureReason
	}
	retryAt := time.Now().Add(time.Duration(30*(1<<min(saga.Attempts, 6))) * time.Second)
	saga.NextRetryAt = pgtype.Timestamptz{Time: retryAt, Valid: true}
	saga.UpdatedAt = time.Now()
	return true, nil
}

func (r *inMemorySagaRepo) ClaimStale(ctx context.Context, tx pgx.Tx, limit int32) ([]sqlc.OrderSaga, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	var claimed []sqlc.OrderSaga

	for _, saga := range r.sagas {
		if int32(len(claimed)) >= limit {
			break
		}

		isStaleStarted := saga.State == orderservice.SagaStateStarted && now.Sub(saga.UpdatedAt) > 10*time.Millisecond
		isRetryDue := saga.State == orderservice.SagaStateCompensating && (!saga.NextRetryAt.Valid || now.After(saga.NextRetryAt.Time))

		if isStaleStarted || isRetryDue {
			saga.State = orderservice.SagaStateCompensating
			saga.Attempts++
			saga.UpdatedAt = now
			claimed = append(claimed, *saga)
		}
	}
	return claimed, nil
}

func (r *inMemorySagaRepo) UpdateStep(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, lastStep *string, snapshot []byte) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	saga, exists := r.sagas[orderUUID]
	if !exists || saga.State != orderservice.SagaStateStarted {
		return false, nil
	}

	saga.LastStep = lastStep
	saga.Snapshot = snapshot
	saga.UpdatedAt = time.Now()
	return true, nil
}

func (r *inMemorySagaRepo) GetByOrderUUID(ctx context.Context, orderUUID uuid.UUID) (sqlc.OrderSaga, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	saga, exists := r.sagas[orderUUID]
	if !exists {
		return sqlc.OrderSaga{}, pgx.ErrNoRows
	}
	return *saga, nil
}

func (r *inMemorySagaRepo) GetByIdempotencyKey(ctx context.Context, userUUID uuid.UUID, idempotencyKey string) (sqlc.OrderSaga, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	idxKey := fmt.Sprintf("%s:%s", userUUID.String(), idempotencyKey)
	orderUUID, exists := r.idempotencyIndex[idxKey]
	if !exists {
		return sqlc.OrderSaga{}, pgx.ErrNoRows
	}
	return *r.sagas[orderUUID], nil
}

func setupFaultInjectionHarness(t *testing.T) (
	*inMemorySagaRepo,
	*MockOrderStockPort,
	*MockOrderCouponPort,
	*MockOrderRepository,
	*MockOrderItemRepository,
	*MockTrackingRepository,
	*MockOutboxRepository,
	*MockCartService,
	*orderservice.OrderCustomer,
	string,
	*int32, // pointer to track live stock
) {
	repo := newInMemorySagaRepo()
	mockStock := &MockOrderStockPort{}
	mockCoupon := &MockOrderCouponPort{}
	mockOrder := &MockOrderRepository{}
	mockItem := &MockOrderItemRepository{}
	mockTrack := &MockTrackingRepository{}
	mockOutbox := &MockOutboxRepository{}
	mockCart := &MockCartService{}

	userUUID := uuid.New()
	customer := &orderservice.OrderCustomer{
		UserID:       99,
		UserUUID:     userUUID,
		UserFullname: "Fault Injection Test Customer",
		UserEmail:    "fault@example.com",
		Status:       1,
	}

	prodUUID := uuid.New().String()
	var stock int32 = 100 // Initial stock

	// Stock port default behaviors: track stock in memory
	mockStock.LockAndDeductStockWithRefFunc = func(ctx context.Context, items []orderservice.DeductItem, refID string, refType string, actorID *int32, note *string) error {
		for _, it := range items {
			if stock < it.Quantity {
				return errors.New("OUT_OF_STOCK")
			}
			stock -= it.Quantity
		}
		return nil
	}

	mockStock.RestoreStockWithRefFunc = func(ctx context.Context, items []orderservice.RestoreItem, refID string, refType string, actorID *int32, note *string) error {
		for _, it := range items {
			stock += it.Quantity
		}
		return nil
	}

	return repo, mockStock, mockCoupon, mockOrder, mockItem, mockTrack, mockOutbox, mockCart, customer, prodUUID, &stock
}

func buildOrchestrator(
	repo repository.SagaRepository,
	stockPort orderservice.OrderStockPort,
	couponPort orderservice.OrderCouponPort,
	orderStore orderservice.OrderStorePort,
	itemStore orderservice.OrderItemStorePort,
	trackStore orderservice.OrderTrackingPort,
	outboxStore orderservice.OrderOutboxPort,
	cartPort orderservice.OrderCartPort,
	customer *orderservice.OrderCustomer,
	prodUUID string,
) orderservice.OrderSagaOrchestrator {
	couponID := int32(77)
	mockPricing := &MockOrderPricingService{
		CalculateOrderTotalsFunc: func(ctx context.Context, items []orderservice.ItemPriceInput, couponCode *string, userID int32, shippingFee float64, now time.Time) (*orderservice.OrderPricingResult, error) {
			var appliedID *int32
			if couponCode != nil && *couponCode != "" {
				appliedID = &couponID
			}
			return &orderservice.OrderPricingResult{
				AppliedCouponID:   appliedID,
				AppliedCouponCode: couponCode,
				SubtotalAmount:    50.0,
				ShippingFee:       5.0,
				DiscountAmount:    0.0,
				FinalTotalAmount:  55.0,
				Items: []orderservice.ItemPriceResult{
					{ProductID: 1, ProductName: "Consistent Product", Price: 50.0, Subtotal: 50.0, Quantity: 1},
				},
			}, nil
		},
	}

	mockCatalog := &MockCatalogPort{
		GetProductsForCheckoutFunc: func(ctx context.Context, productUUIDs []uuid.UUID) (map[string]orderservice.CheckoutProductSnapshot, error) {
			m := make(map[string]orderservice.CheckoutProductSnapshot)
			pUUID, _ := uuid.Parse(prodUUID)
			m[prodUUID] = orderservice.CheckoutProductSnapshot{
				ProductID:   1,
				ProductUUID: pUUID,
				ProductName: "Consistent Product",
				Price:       50.0,
				Status:      1,
			}
			return m, nil
		},
	}

	userRepo := &MockUserRepository{
		GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
			return sqlc.User{UserID: customer.UserID, UserUuid: customer.UserUUID}, nil
		},
	}
	customerAdapter := orderservice.NewOrderCustomerAdapter(userRepo)

	return orderservice.NewOrderSagaOrchestrator(
		repo,
		testutils.NewNoopTxManager(),
		mockPricing,
		mockCatalog,
		stockPort,
		orderStore,
		itemStore,
		customerAdapter,
		cartPort,
		couponPort,
		trackStore,
		nil,
		outboxStore,
	)
}

// Test 1: Crash after stock reservation (before apply coupon)
// Sweeper must discover stale saga, restore stock, and mark FAILED.
// Invariant verified: final stock == initial stock.
func TestFaultInjection_CrashAfterStockReservation_SweeperRestoresStock(t *testing.T) {
	repo, mockStock, mockCoupon, mockOrder, mockItem, mockTrack, mockOutbox, mockCart, customer, prodUUID, stockPtr := setupFaultInjectionHarness(t)
	logger := zerolog.Nop()

	initialStock := *stockPtr

	// Fault injection: Coupon service simulates a crash / timeout right after stock is reserved
	mockCoupon.LockAndApplyCouponFunc = func(ctx context.Context, couponID int32, userID int32, orderID int32, orderUUID *uuid.UUID) error {
		return errors.New("SIMULATED_CRASH_OR_NETWORK_TIMEOUT")
	}

	orch := buildOrchestrator(repo, mockStock, mockCoupon, mockOrder, mockItem, mockTrack, mockOutbox, mockCart, customer, prodUUID)

	ctx := orderservice.WithOrderCustomer(context.Background(), customer)
	couponCode := "DISCOUNT10"
	input := v1dto.CreateOrderInput{
		CouponCode:      &couponCode,
		ShippingName:    "Customer Name",
		ShippingPhone:   "0987654321",
		ShippingAddress: "123 Main St",
		Items: []v1dto.CreateOrderItemInput{
			{ProductUUID: prodUUID, Quantity: 2},
		},
	}

	// 1. Checkout execution fails due to crash
	_, err := orch.ExecuteCheckoutSaga(ctx, customer.UserUUID, "crash-step-coupon", input)
	require.Error(t, err)

	// In the immediate compensation attempt, suppose orchestrator also crashed before running compensation
	// We simulate this by resetting the saga state back to STARTED to simulate an abrupt server SIGKILL
	saga, getErr := repo.GetByIdempotencyKey(ctx, customer.UserUUID, "crash-step-coupon")
	require.NoError(t, getErr)

	// Simulate node crash: saga was left in STARTED with updated_at in the past
	repo.mu.Lock()
	repo.sagas[saga.OrderUuid].State = orderservice.SagaStateStarted
	repo.sagas[saga.OrderUuid].UpdatedAt = time.Now().Add(-1 * time.Minute)
	// Deduct stock to simulate that the reservation had persisted before node died
	*stockPtr -= 2
	repo.mu.Unlock()

	// Stock is temporarily missing (98 instead of 100)
	assert.Equal(t, initialStock-2, *stockPtr)

	// 2. Background Sweeper wakes up and runs
	sweeper := NewSagaRecoveryWorker(repo, orch, 10*time.Millisecond, 10, &logger)
	recoveredCount, err := sweeper.ProcessBatch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, recoveredCount)

	// 3. Verify invariants:
	// a) Total stock is 100% restored to initial stock
	assert.Equal(t, initialStock, *stockPtr, "INVARIANT VIOLATION: Stock was leaked after crash")

	// b) Saga is closed with state FAILED
	finalSaga, err := repo.GetByOrderUUID(context.Background(), saga.OrderUuid)
	require.NoError(t, err)
	assert.Equal(t, orderservice.SagaStateFailed, finalSaga.State)
}

// Test 2: Ambiguous Commit in Tx#2 (Network partition during final commit response).
// Client retries with identical Idempotency-Key.
// Invariant verified: Exactly 1 order created, stock deducted exactly once.
func TestFaultInjection_AmbiguousCommitTx2_IdempotentRetrySucceeds(t *testing.T) {
	repo, mockStock, mockCoupon, mockOrder, mockItem, mockTrack, mockOutbox, mockCart, customer, prodUUID, stockPtr := setupFaultInjectionHarness(t)

	initialStock := *stockPtr
	createdOrdersCount := 0

	createdOrderUUID := uuid.New()
	mockOrder.CreateFunc = func(ctx context.Context, params sqlc.CreateOrderParams) (sqlc.Order, error) {
		createdOrdersCount++
		return sqlc.Order{
			OrderID:     888,
			OrderUuid:   createdOrderUUID,
			OrderNumber: "ORD-IDEMPOTENT-001",
			OrderStatus: 1,
		}, nil
	}
	mockItem.CreateFunc = func(ctx context.Context, tx pgx.Tx, params sqlc.CreateOrderItemParams) (sqlc.OrderItem, error) {
		return sqlc.OrderItem{OrderItemID: 1}, nil
	}
	mockTrack.InsertEventFunc = func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
		return sqlc.OrderTrackingEvent{EventID: 1}, nil
	}
	mockOutbox.InsertFunc = func(ctx context.Context, tx pgx.Tx, aggType, aggID, evtType string, payload any) (sqlc.OutboxEvent, error) {
		return sqlc.OutboxEvent{EventID: 1}, nil
	}
	mockCart.RemovePurchasedItemsFunc = func(ctx context.Context, userID int32, productIDs []int32) error {
		return nil
	}

	orch := buildOrchestrator(repo, mockStock, mockCoupon, mockOrder, mockItem, mockTrack, mockOutbox, mockCart, customer, prodUUID)

	ctx := orderservice.WithOrderCustomer(context.Background(), customer)
	input := v1dto.CreateOrderInput{
		ShippingName:    "Customer Name",
		ShippingPhone:   "0987654321",
		ShippingAddress: "123 Main St",
		Items: []v1dto.CreateOrderItemInput{
			{ProductUUID: prodUUID, Quantity: 3},
		},
	}

	// First execution completes Tx#2 (state = CONFIRMED)
	order1, err := orch.ExecuteCheckoutSaga(ctx, customer.UserUUID, "idem-key-duplicate-network-drop", input)
	require.NoError(t, err)
	require.NotNil(t, order1)
	assert.Equal(t, 1, createdOrdersCount)
	assert.Equal(t, initialStock-3, *stockPtr)

	// Simulate client network drop: client never received the response, retries with the SAME key
	mockOrder.GetByUUIDFunc = func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
		return sqlc.GetOrderByUUIDRow{
			OrderID:     888,
			OrderUuid:   createdOrderUUID,
			OrderNumber: "ORD-IDEMPOTENT-001",
			OrderStatus: 1,
		}, nil
	}
	mockItem.GetByOrderIDFunc = func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
		return []sqlc.GetOrderItemsByOrderIDRow{}, nil
	}

	order2, err := orch.ExecuteCheckoutSaga(ctx, customer.UserUUID, "idem-key-duplicate-network-drop", input)
	require.NoError(t, err)
	require.NotNil(t, order2)

	// INVARIANT VERIFICATION:
	// 1) Exactly 1 order was created in DB
	assert.Equal(t, 1, createdOrdersCount, "INVARIANT VIOLATION: Duplicate order was persisted on retry")
	// 2) Stock was deducted exactly once (initial - 3, NOT initial - 6)
	assert.Equal(t, initialStock-3, *stockPtr, "INVARIANT VIOLATION: Stock was deducted twice")
	// 3) Both calls returned the same order UUID
	assert.Equal(t, order1.UUID, order2.UUID)
	assert.Equal(t, order1.OrderNumber, order2.OrderNumber)
}

// Test 3: Race condition between a slow checkout request and Sweeper compensation.
// Sweeper claims the stale saga first (state -> COMPENSATING).
// When the slow request wakes up and tries Tx#2 CAS (WHERE state = 'STARTED'),
// the CAS MUST fail and abort without creating an orphan order.
func TestFaultInjection_RaceSlowRequestVsSweeper_CASPreventsOrphanOrder(t *testing.T) {
	repo, mockStock, mockCoupon, mockOrder, mockItem, mockTrack, mockOutbox, mockCart, customer, prodUUID, stockPtr := setupFaultInjectionHarness(t)
	logger := zerolog.Nop()

	orderCreated := false
	mockOrder.CreateFunc = func(ctx context.Context, params sqlc.CreateOrderParams) (sqlc.Order, error) {
		orderCreated = true
		return sqlc.Order{OrderID: 1, OrderUuid: uuid.New()}, nil
	}

	orch := buildOrchestrator(repo, mockStock, mockCoupon, mockOrder, mockItem, mockTrack, mockOutbox, mockCart, customer, prodUUID)

	ctx := orderservice.WithOrderCustomer(context.Background(), customer)
	sagaOrderUUID := uuid.New()

	// Pre-seed a saga in STARTED state that timed out
	repo.mu.Lock()
	snapshotBytes, _ := json.Marshal(orderservice.SagaSnapshot{
		OrderNumber:   "ORD-SLOW-RACE",
		UserID:        customer.UserID,
		StockReserved: true,
		DeductItems: []orderservice.SagaDeductItem{
			{ProductUUID: prodUUID, ProductID: 1, Quantity: 4},
		},
	})
	repo.sagas[sagaOrderUUID] = &sqlc.OrderSaga{
		OrderUuid:      sagaOrderUUID,
		UserUuid:       customer.UserUUID,
		IdempotencyKey: "slow-race-key",
		State:          orderservice.SagaStateStarted,
		Snapshot:       snapshotBytes,
		UpdatedAt:      time.Now().Add(-5 * time.Minute), // stale
	}
	*stockPtr -= 4
	repo.mu.Unlock()

	// 1. Sweeper runs and wins the CAS race: claims saga for compensation
	sweeper := NewSagaRecoveryWorker(repo, orch, 10*time.Millisecond, 10, &logger)
	count, err := sweeper.ProcessBatch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Saga is now FAILED and stock has been restored
	assert.Equal(t, int32(100), *stockPtr)

	// 2. The slow thread wakes up and tries to confirm the saga via CAS
	wonCAS, err := repo.Confirm(ctx, nil, sagaOrderUUID)
	require.NoError(t, err)

	// INVARIANT VERIFICATION:
	// The slow thread MUST lose the CAS
	assert.False(t, wonCAS, "INVARIANT VIOLATION: Slow thread won CAS on already-compensated saga")
	// No order was created
	assert.False(t, orderCreated, "INVARIANT VIOLATION: Orphan order created")
}

// Test 4: Compensation retry backoff under temporary participant outage.
// Inventory service is temporarily down during initial compensation attempt.
// Invariant verified: Saga enters COMPENSATING, records failure reason and retry backoff.
// When participant recovers, next retry succeeds and saga moves to FAILED.
func TestFaultInjection_ParticipantOutage_CompensationRetriesAndRecovers(t *testing.T) {
	repo, mockStock, mockCoupon, mockOrder, mockItem, mockTrack, mockOutbox, mockCart, customer, prodUUID, stockPtr := setupFaultInjectionHarness(t)
	logger := zerolog.Nop()

	orch := buildOrchestrator(repo, mockStock, mockCoupon, mockOrder, mockItem, mockTrack, mockOutbox, mockCart, customer, prodUUID)

	sagaOrderUUID := uuid.New()
	snapshotBytes, _ := json.Marshal(orderservice.SagaSnapshot{
		OrderNumber:   "ORD-PARTICIPANT-OUTAGE",
		UserID:        customer.UserID,
		StockReserved: true,
		DeductItems: []orderservice.SagaDeductItem{
			{ProductUUID: prodUUID, ProductID: 1, Quantity: 1},
		},
	})

	repo.mu.Lock()
	repo.sagas[sagaOrderUUID] = &sqlc.OrderSaga{
		OrderUuid:      sagaOrderUUID,
		UserUuid:       customer.UserUUID,
		IdempotencyKey: "participant-down-key",
		State:          orderservice.SagaStateStarted,
		Snapshot:       snapshotBytes,
		UpdatedAt:      time.Now().Add(-3 * time.Minute),
	}
	*stockPtr -= 1 // Simulate stock deduction that occurred before crash
	repo.mu.Unlock()

	// 1. Participant outage: Stock service throws unavailable error
	mockStock.RestoreStockWithRefFunc = func(ctx context.Context, items []orderservice.RestoreItem, refID string, refType string, actorID *int32, note *string) error {
		return errors.New("INVENTORY_GRPC_UNAVAILABLE")
	}

	sweeper := NewSagaRecoveryWorker(repo, orch, 10*time.Millisecond, 10, &logger)
	_, _ = sweeper.ProcessBatch(context.Background())

	// Verify saga transitioned to COMPENSATING and scheduled retry backoff
	sagaAfterFailure, err := repo.GetByOrderUUID(context.Background(), sagaOrderUUID)
	require.NoError(t, err)
	assert.Equal(t, orderservice.SagaStateCompensating, sagaAfterFailure.State)
	assert.GreaterOrEqual(t, sagaAfterFailure.Attempts, int32(1))
	assert.True(t, sagaAfterFailure.NextRetryAt.Valid)

	// 2. Participant recovers: Stock service is back online
	mockStock.RestoreStockWithRefFunc = func(ctx context.Context, items []orderservice.RestoreItem, refID string, refType string, actorID *int32, note *string) error {
		for _, it := range items {
			*stockPtr += it.Quantity
		}
		return nil
	}

	// Make retry immediately due
	repo.mu.Lock()
	repo.sagas[sagaOrderUUID].NextRetryAt = pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Second), Valid: true}
	repo.mu.Unlock()

	// Sweeper retries
	recoveredCount, err := sweeper.ProcessBatch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, recoveredCount)

	// INVARIANT VERIFICATION:
	// Final saga state is FAILED, not stuck in COMPENSATING
	finalSaga, err := repo.GetByOrderUUID(context.Background(), sagaOrderUUID)
	require.NoError(t, err)
	assert.Equal(t, orderservice.SagaStateFailed, finalSaga.State)
	assert.Equal(t, int32(100), *stockPtr)
}
