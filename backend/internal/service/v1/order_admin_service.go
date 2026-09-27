package v1service

import (
	"context"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"

	"github.com/google/uuid"
)

type orderAdminService struct {
	orderService OrderService
}

// NewOrderAdminService creates an admin-specific order service for batch and management tasks.
func NewOrderAdminService(orderService OrderService) OrderAdminService {
	return &orderAdminService{
		orderService: orderService,
	}
}

func (s *orderAdminService) GetAllOrders(ctx context.Context, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error) {
	return s.orderService.GetAllOrders(ctx, input)
}

func (s *orderAdminService) UpdateOrderStatus(ctx context.Context, input sqlc.UpdateOrderStatusParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {
	return s.orderService.UpdateOrderStatus(ctx, input)
}

func (s *orderAdminService) BulkUpdateStatus(ctx context.Context, input v1dto.BulkUpdateOrderStatusInput) (*v1dto.BulkOperationResultDTO, error) {
	result := &v1dto.BulkOperationResultDTO{
		Total:  len(input.OrderUUIDs),
		Errors: make([]v1dto.BulkItemError, 0),
	}

	for _, rawUUID := range input.OrderUUIDs {
		parsedUUID, err := uuid.Parse(rawUUID)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, v1dto.BulkItemError{
				OrderUUID: rawUUID,
				Error:     "invalid UUID format",
			})
			continue
		}

		_, _, err = s.orderService.UpdateOrderStatus(ctx, sqlc.UpdateOrderStatusParams{
			OrderUuid:   parsedUUID,
			OrderStatus: input.Status,
		})
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, v1dto.BulkItemError{
				OrderUUID: rawUUID,
				Error:     err.Error(),
			})
			continue
		}

		result.Succeeded++
	}

	return result, nil
}

func (s *orderAdminService) BulkCancel(ctx context.Context, userUUID uuid.UUID, input v1dto.BulkCancelOrdersInput) (*v1dto.BulkOperationResultDTO, error) {
	result := &v1dto.BulkOperationResultDTO{
		Total:  len(input.OrderUUIDs),
		Errors: make([]v1dto.BulkItemError, 0),
	}

	for _, rawUUID := range input.OrderUUIDs {
		parsedUUID, err := uuid.Parse(rawUUID)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, v1dto.BulkItemError{
				OrderUUID: rawUUID,
				Error:     "invalid UUID format",
			})
			continue
		}

		_, err = s.orderService.CancelOrder(ctx, userUUID, parsedUUID, input.Reason)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, v1dto.BulkItemError{
				OrderUUID: rawUUID,
				Error:     err.Error(),
			})
			continue
		}

		result.Succeeded++
	}

	return result, nil
}
