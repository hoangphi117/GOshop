package service

import (
	"context"
	"fmt"

	"goshop/services/order/contract"
	"goshop/services/payment/internal/dto"

	"github.com/google/uuid"
)

type OrderReaderGRPCClient struct {
	client contract.OrderClient
}

func NewOrderReaderGRPCClient(client contract.OrderClient) OrderReaderClient {
	return &OrderReaderGRPCClient{client: client}
}

func (c *OrderReaderGRPCClient) GetByUUID(ctx context.Context, orderUUID uuid.UUID) (*dto.PaymentOrderSummary, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("order gRPC client is unavailable")
	}

	order, err := c.client.GetOrderSummary(ctx, orderUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch order %s: %w", orderUUID, err)
	}
	if order == nil {
		return nil, fmt.Errorf("order %s not found", orderUUID)
	}

	return &dto.PaymentOrderSummary{
		OrderUUID:   order.OrderUUID,
		OrderNumber: order.OrderNumber,
		UserID:      order.UserID,
		UserUUID:    order.UserUUID,
		TotalAmount: order.TotalAmount,
		OrderStatus: order.Status,
	}, nil
}
