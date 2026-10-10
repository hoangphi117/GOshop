package v1service_test

import (
	"context"
	"testing"
	"time"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/testutils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderLifecycleService(t *testing.T) {
	ctx := context.Background()
	ownerUUID := uuid.New()
	otherUserUUID := uuid.New()
	orderUUID := uuid.New()

	baseCustomer := v1service.NewOrderCustomer(10, ownerUUID, "Alice", "alice@example.com", 3, 1)
	otherCustomer := v1service.NewOrderCustomer(20, otherUserUUID, "Bob", "bob@example.com", 3, 1)

	baseOrderRow := sqlc.GetOrderByUUIDRow{
		OrderID:            100,
		OrderUuid:          orderUUID,
		OrderNumber:        "ORD-TEST-100",
		OrderUserID:        baseCustomer.UserID,
		OrderStatus:        v1service.OrderStatusPending,
		OrderPaymentStatus: v1service.OrderPaymentStatusUnpaid,
		UserUuid:           ownerUUID,
		OrderCreatedAt:     time.Now(),
	}

	t.Run("UpdateOrderStatus: Invalid status transition -> BadRequest", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return baseOrderRow, nil
			},
		}

		svc := v1service.NewOrderLifecycleService(
			testutils.NewNoopTxManager(),
			mockOrderRepo,
			&testutils.MockOrderItemRepository{},
			&testutils.MockTrackingRepository{},
			nil,
			&testutils.MockInventoryService{},
			nil,
		)

		// Transition directly from Pending (1) to Delivered (4) is invalid
		res, err := svc.UpdateOrderStatus(ctx, orderUUID, v1dto.UpdateOrderStatusInput{
			Status: v1service.OrderStatusDelivered,
		})
		require.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "Cannot change order status")
	})

	t.Run("UpdateOrderStatus: Valid transition Pending -> Confirmed", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return baseOrderRow, nil
			},
			UpdateStatusFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpdateOrderStatusParams) (sqlc.Order, error) {
				return sqlc.Order{
					OrderID:            baseOrderRow.OrderID,
					OrderUuid:          orderUUID,
					OrderNumber:        baseOrderRow.OrderNumber,
					OrderUserID:        baseOrderRow.OrderUserID,
					OrderStatus:        v1service.OrderStatusConfirmed,
					OrderPaymentStatus: baseOrderRow.OrderPaymentStatus,
					OrderCreatedAt:     time.Now(),
				}, nil
			},
		}

		trackingInserted := false
		mockTracking := &testutils.MockTrackingRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error) {
				trackingInserted = true
				return sqlc.OrderTrackingEvent{}, nil
			},
		}

		svc := v1service.NewOrderLifecycleService(
			testutils.NewNoopTxManager(),
			mockOrderRepo,
			&testutils.MockOrderItemRepository{},
			mockTracking,
			nil,
			&testutils.MockInventoryService{},
			nil,
		)

		res, err := svc.UpdateOrderStatus(ctx, orderUUID, v1dto.UpdateOrderStatusInput{
			Status: v1service.OrderStatusConfirmed,
		})
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, "Confirmed", res.Status)
		assert.True(t, trackingInserted)
	})

	t.Run("CancelOrder: Unauthorized user -> Forbidden", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: otherCustomer.UserID, UserUuid: otherCustomer.UserUUID, UserLevel: 3}, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return baseOrderRow, nil // owned by userID 10
			},
		}

		customerAdapter := v1service.NewOrderCustomerAdapter(mockUserRepo)
		svc := v1service.NewOrderLifecycleService(
			testutils.NewNoopTxManager(),
			mockOrderRepo,
			&testutils.MockOrderItemRepository{},
			&testutils.MockTrackingRepository{},
			customerAdapter,
			&testutils.MockInventoryService{},
			nil,
		)

		res, err := svc.CancelOrder(ctx, otherUserUUID, orderUUID, nil)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "You are not authorized to cancel this order")
	})

	t.Run("CancelOrder: Already delivered order -> BadRequest", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: baseCustomer.UserID, UserUuid: baseCustomer.UserUUID, UserLevel: 3}, nil
			},
		}
		deliveredOrderRow := baseOrderRow
		deliveredOrderRow.OrderStatus = v1service.OrderStatusDelivered

		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return deliveredOrderRow, nil
			},
		}

		customerAdapter := v1service.NewOrderCustomerAdapter(mockUserRepo)
		svc := v1service.NewOrderLifecycleService(
			testutils.NewNoopTxManager(),
			mockOrderRepo,
			&testutils.MockOrderItemRepository{},
			&testutils.MockTrackingRepository{},
			customerAdapter,
			&testutils.MockInventoryService{},
			nil,
		)

		res, err := svc.CancelOrder(ctx, ownerUUID, orderUUID, nil)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "Only pending or confirmed orders can be cancelled")
	})

	t.Run("CancelOrder: Success with stock restoration and outbox event", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: baseCustomer.UserID, UserUuid: baseCustomer.UserUUID, UserLevel: 3}, nil
			},
		}

		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return baseOrderRow, nil
			},
			CancelOrderFunc: func(ctx context.Context, tx pgx.Tx, u uuid.UUID) (sqlc.Order, error) {
				return sqlc.Order{
					OrderID:            baseOrderRow.OrderID,
					OrderUuid:          orderUUID,
					OrderNumber:        baseOrderRow.OrderNumber,
					OrderUserID:        baseCustomer.UserID,
					OrderStatus:        v1service.OrderStatusCancelled,
					OrderPaymentStatus: baseOrderRow.OrderPaymentStatus,
					OrderCreatedAt:     time.Now(),
				}, nil
			},
		}

		mockItemRepo := &testutils.MockOrderItemRepository{
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				return []sqlc.GetOrderItemsByOrderIDRow{
					{
						OrderItemProductID: 15,
						OrderItemQuantity:  3,
					},
				}, nil
			},
		}

		stockRestored := false
		mockInv := &testutils.MockInventoryService{
			RestoreStockWithRefFunc: func(ctx context.Context, items []v1service.RestoreItem, refID, refType string, actorID *int32, note *string) error {
				stockRestored = true
				assert.Equal(t, int32(15), items[0].ProductID)
				assert.Equal(t, int32(3), items[0].Quantity)
				return nil
			},
		}

		outboxEventPublished := false
		mockOutbox := &testutils.MockOutboxRepository{
			InsertFunc: func(ctx context.Context, tx pgx.Tx, aggType, aggID, evtType string, payload any) (sqlc.OutboxEvent, error) {
				outboxEventPublished = true
				assert.Equal(t, "order", aggType)
				assert.Equal(t, "order.cancelled", evtType)
				return sqlc.OutboxEvent{}, nil
			},
		}

		customerAdapter := v1service.NewOrderCustomerAdapter(mockUserRepo)
		svc := v1service.NewOrderLifecycleService(
			testutils.NewNoopTxManager(),
			mockOrderRepo,
			mockItemRepo,
			&testutils.MockTrackingRepository{},
			customerAdapter,
			mockInv,
			nil,
			mockOutbox,
		)

		reason := "Changed my mind"
		res, err := svc.CancelOrder(ctx, ownerUUID, orderUUID, &reason)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, "Cancelled", res.Status)
		assert.True(t, stockRestored, "Stock must be restored on cancellation")
		assert.True(t, outboxEventPublished, "Outbox event order.cancelled must be recorded")
	})
}
