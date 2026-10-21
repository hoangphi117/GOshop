package client

import (
	"context"
	"fmt"
	v1service "goshop/internal/service/v1"
	ordercontract "goshop/services/order/contract"

	"github.com/google/uuid"
)

// GRPCOrderReader implements v1service.OrderReader backed by Order gRPC microservice.
type GRPCOrderReader struct {
	client ordercontract.OrderClient
}

// NewGRPCOrderReader creates a new OrderReader calling the Order microservice via gRPC.
func NewGRPCOrderReader(client ordercontract.OrderClient) v1service.OrderReader {
	return &GRPCOrderReader{client: client}
}

func (r *GRPCOrderReader) GetOrderSummary(ctx context.Context, orderUUID uuid.UUID) (*v1service.OrderSummary, error) {
	if r.client == nil {
		return nil, fmt.Errorf("order grpc client is not initialized")
	}
	s, err := r.client.GetOrderSummary(ctx, orderUUID)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, nil
	}
	return &v1service.OrderSummary{
		OrderID:       s.OrderID,
		OrderUUID:     s.OrderUUID,
		OrderNumber:   s.OrderNumber,
		UserID:        s.UserID,
		UserUUID:      s.UserUUID,
		CustomerName:  s.CustomerName,
		CustomerEmail: s.CustomerEmail,
		Status:        s.Status,
		TotalAmount:   s.TotalAmount,
		DeliveredAt:   s.DeliveredAt,
		UpdatedAt:     s.UpdatedAt,
	}, nil
}

func (r *GRPCOrderReader) VerifyPurchase(ctx context.Context, orderUUID uuid.UUID, userID int32, productID int32) (*v1service.PurchaseVerification, error) {
	if r.client == nil {
		return nil, fmt.Errorf("order grpc client is not initialized")
	}
	v, err := r.client.VerifyPurchase(ctx, orderUUID, userID, productID, nil, nil)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	return &v1service.PurchaseVerification{
		OrderID:       v.OrderID,
		OrderUUID:     v.OrderUUID,
		UserID:        v.UserID,
		IsPurchased:   v.IsPurchased,
		CanReview:     v.CanReview,
		DeliveredAt:   v.DeliveredAt,
		CustomerName:  v.CustomerName,
		CustomerEmail: v.CustomerEmail,
		ShippingName:  v.ShippingName,
	}, nil
}
