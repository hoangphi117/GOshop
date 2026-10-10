package v1service

import (
	"context"
	"time"

	"goshop/internal/repository"
	"goshop/internal/utils"

	"github.com/google/uuid"
)

// PaymentOrderSummary encapsulates read-only order details needed by the Payment domain.
type PaymentOrderSummary struct {
	OrderID             int32
	OrderUUID           uuid.UUID
	OrderNumber         string
	UserID              int32
	UserUUID            uuid.UUID
	TotalAmount         float64
	PaymentStatus       int32
	OrderStatus         int32
	PaymentMethod       int32
	OrderRefundedAmount float64
	CreatedAt           time.Time
}

// PaymentOrderReaderPort decouples the Payment service from the full OrderRepository.
type PaymentOrderReaderPort interface {
	GetByUUID(ctx context.Context, orderUUID uuid.UUID) (*PaymentOrderSummary, error)
	GetByOrderNumber(ctx context.Context, orderNumber string) (*PaymentOrderSummary, error)
}

// OrderReaderAdapter adapts repository.OrderRepository into PaymentOrderReaderPort.
type OrderReaderAdapter struct {
	orderRepo repository.OrderRepository
}

// NewOrderReaderAdapter constructs a new OrderReaderAdapter.
func NewOrderReaderAdapter(orderRepo repository.OrderRepository) PaymentOrderReaderPort {
	return &OrderReaderAdapter{orderRepo: orderRepo}
}

// OrderRepository returns the underlying repository.OrderRepository if available.
func (a *OrderReaderAdapter) OrderRepository() repository.OrderRepository {
	if a == nil {
		return nil
	}
	return a.orderRepo
}

// GetByUUID retrieves an order by its UUID and maps it to a PaymentOrderSummary.
func (a *OrderReaderAdapter) GetByUUID(ctx context.Context, orderUUID uuid.UUID) (*PaymentOrderSummary, error) {
	if a == nil || a.orderRepo == nil {
		return nil, utils.NewError(utils.ErrCodeInternal, "order repository not initialized")
	}

	order, err := a.orderRepo.GetByUUID(ctx, orderUUID)
	if err != nil {
		return nil, err
	}

	return &PaymentOrderSummary{
		OrderID:             order.OrderID,
		OrderUUID:           order.OrderUuid,
		OrderNumber:         order.OrderNumber,
		UserID:              order.OrderUserID,
		UserUUID:            order.UserUuid,
		TotalAmount:         utils.NumericToFloat(order.OrderTotalAmount),
		PaymentStatus:       order.OrderPaymentStatus,
		OrderStatus:         order.OrderStatus,
		PaymentMethod:       order.OrderPaymentMethod,
		OrderRefundedAmount: utils.NumericToFloat(order.OrderRefundedAmount),
		CreatedAt:           order.OrderCreatedAt,
	}, nil
}

// GetByOrderNumber retrieves an order by its OrderNumber and maps it to a PaymentOrderSummary.
func (a *OrderReaderAdapter) GetByOrderNumber(ctx context.Context, orderNumber string) (*PaymentOrderSummary, error) {
	if a == nil || a.orderRepo == nil {
		return nil, utils.NewError(utils.ErrCodeInternal, "order repository not initialized")
	}

	order, err := a.orderRepo.GetByOrderNumber(ctx, orderNumber)
	if err != nil {
		return nil, err
	}

	return &PaymentOrderSummary{
		OrderID:             order.OrderID,
		OrderUUID:           order.OrderUuid,
		OrderNumber:         order.OrderNumber,
		UserID:              order.OrderUserID,
		UserUUID:            order.UserUuid,
		TotalAmount:         utils.NumericToFloat(order.OrderTotalAmount),
		PaymentStatus:       order.OrderPaymentStatus,
		OrderStatus:         order.OrderStatus,
		PaymentMethod:       order.OrderPaymentMethod,
		OrderRefundedAmount: utils.NumericToFloat(order.OrderRefundedAmount),
		CreatedAt:           order.OrderCreatedAt,
	}, nil
}
