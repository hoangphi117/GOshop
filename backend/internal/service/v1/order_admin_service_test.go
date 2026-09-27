package v1service_test

import (
	"context"
	"errors"
	"testing"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/testutils"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderAdminService_BulkOperations(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	orderUUID1 := uuid.New()
	orderUUID2 := uuid.New()

	t.Run("BulkUpdateStatus: Success and Failure Aggregation", func(t *testing.T) {
		mockOrderService := &testutils.MockOrderService{
			UpdateOrderStatusFunc: func(ctx context.Context, input sqlc.UpdateOrderStatusParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {
				if input.OrderUuid == orderUUID1 {
					return sqlc.Order{OrderUuid: orderUUID1}, nil, nil
				}
				return sqlc.Order{}, nil, errors.New("order not found")
			},
		}

		adminSvc := v1service.NewOrderAdminService(mockOrderService)
		result, err := adminSvc.BulkUpdateStatus(ctx, v1dto.BulkUpdateOrderStatusInput{
			OrderUUIDs: []string{orderUUID1.String(), orderUUID2.String(), "invalid-uuid"},
			Status:     v1service.OrderStatusShipped,
		})

		require.NoError(t, err)
		assert.Equal(t, 3, result.Total)
		assert.Equal(t, 1, result.Succeeded)
		assert.Equal(t, 2, result.Failed)
		assert.Len(t, result.Errors, 2)
	})

	t.Run("BulkCancel: Success and Failure Aggregation", func(t *testing.T) {
		mockOrderService := &testutils.MockOrderService{
			CancelOrderFunc: func(ctx context.Context, uUUID, oUUID uuid.UUID, reason *string) (sqlc.Order, error) {
				if oUUID == orderUUID1 {
					return sqlc.Order{OrderUuid: orderUUID1}, nil
				}
				return sqlc.Order{}, errors.New("cannot cancel order in current state")
			},
		}

		adminSvc := v1service.NewOrderAdminService(mockOrderService)
		reason := "Customer requested"
		result, err := adminSvc.BulkCancel(ctx, userUUID, v1dto.BulkCancelOrdersInput{
			OrderUUIDs: []string{orderUUID1.String(), orderUUID2.String()},
			Reason:     &reason,
		})

		require.NoError(t, err)
		assert.Equal(t, 2, result.Total)
		assert.Equal(t, 1, result.Succeeded)
		assert.Equal(t, 1, result.Failed)
	})

	t.Run("GetAllOrders: DelegatesToOrderService", func(t *testing.T) {
		mockOrderService := &testutils.MockOrderService{
			GetAllOrdersFunc: func(ctx context.Context, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error) {
				return []v1dto.OrderSummaryDTO{{UUID: orderUUID1.String()}}, 1, nil
			},
		}

		adminSvc := v1service.NewOrderAdminService(mockOrderService)
		orders, total, err := adminSvc.GetAllOrders(ctx, v1dto.GetOrdersParam{})

		require.NoError(t, err)
		assert.Equal(t, int32(1), total)
		require.Len(t, orders, 1)
		assert.Equal(t, orderUUID1.String(), orders[0].UUID)
	})

	t.Run("UpdateOrderStatus: DelegatesToOrderService", func(t *testing.T) {
		mockOrderService := &testutils.MockOrderService{
			UpdateOrderStatusFunc: func(ctx context.Context, input sqlc.UpdateOrderStatusParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {
				return sqlc.Order{OrderUuid: orderUUID1, OrderStatus: v1service.OrderStatusDelivered}, nil, nil
			},
		}

		adminSvc := v1service.NewOrderAdminService(mockOrderService)
		order, _, err := adminSvc.UpdateOrderStatus(ctx, sqlc.UpdateOrderStatusParams{OrderUuid: orderUUID1, OrderStatus: v1service.OrderStatusDelivered})

		require.NoError(t, err)
		assert.Equal(t, orderUUID1, order.OrderUuid)
		assert.Equal(t, v1service.OrderStatusDelivered, order.OrderStatus)
	})
}
