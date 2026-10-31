package service

import (
	"context"

	ordercontract "goshop/services/order/contract"

	"github.com/google/uuid"
)

// OrderServiceClient adapts the order gRPC contract to the chat domain's
// narrow OrderReader port.
type OrderServiceClient struct {
	client ordercontract.OrderClient
}

func NewOrderServiceClient(client ordercontract.OrderClient) *OrderServiceClient {
	return &OrderServiceClient{client: client}
}

func (r *OrderServiceClient) GetOrderSummary(ctx context.Context, orderUUID uuid.UUID) (*OrderSummary, error) {
	summary, err := r.client.GetOrderSummary(ctx, orderUUID)
	if err != nil {
		return nil, err
	}
	if summary == nil {
		return nil, nil
	}
	return &OrderSummary{
		OrderUUID: summary.OrderUUID,
		UserID:    summary.UserID,
	}, nil
}
