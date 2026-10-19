package grpc

import (
	"context"
	"time"

	orderv1 "goshop/proto/order/v1"
	"goshop/services/order/contract"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type OrderProvider interface {
	GetOrderSummary(ctx context.Context, orderUUID uuid.UUID) (*contract.OrderSummary, error)
	VerifyPurchase(ctx context.Context, orderUUID uuid.UUID, userID, productID int32, userUUID, productUUID *uuid.UUID) (*contract.PurchaseVerification, error)
	GetOrderForPayment(ctx context.Context, orderUUID uuid.UUID) (*contract.OrderPaymentInfo, error)
}

type OrderGRPCServer struct {
	orderv1.UnimplementedOrderServiceServer
	provider OrderProvider
}

func NewOrderGRPCServer(provider OrderProvider) *OrderGRPCServer {
	return &OrderGRPCServer{
		provider: provider,
	}
}

func (s *OrderGRPCServer) GetOrderSummary(ctx context.Context, req *orderv1.GetOrderSummaryRequest) (*orderv1.GetOrderSummaryResponse, error) {
	if req == nil || req.OrderUuid == "" {
		return nil, status.Error(codes.InvalidArgument, "order_uuid is required")
	}

	orderUUID, err := uuid.Parse(req.OrderUuid)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order_uuid format")
	}

	summary, err := s.provider.GetOrderSummary(ctx, orderUUID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get order summary: %v", err)
	}
	if summary == nil {
		return nil, status.Error(codes.NotFound, "order not found")
	}

	items := make([]*orderv1.OrderItemSummary, 0, len(summary.Items))
	for _, it := range summary.Items {
		items = append(items, &orderv1.OrderItemSummary{
			ProductUuid: it.ProductUUID,
			ProductName: it.ProductName,
			Quantity:    it.Quantity,
			Price:       it.Price,
		})
	}

	var deliveredAt string
	if summary.DeliveredAt != nil {
		deliveredAt = summary.DeliveredAt.Format(time.RFC3339)
	}

	return &orderv1.GetOrderSummaryResponse{
		Order: &orderv1.OrderSummaryDTO{
			OrderId:       summary.OrderID,
			OrderUuid:     summary.OrderUUID.String(),
			OrderNumber:   summary.OrderNumber,
			UserId:        summary.UserID,
			UserUuid:      summary.UserUUID.String(),
			CustomerName:  summary.CustomerName,
			CustomerEmail: summary.CustomerEmail,
			Status:        summary.Status,
			TotalAmount:   summary.TotalAmount,
			DeliveredAt:   deliveredAt,
			UpdatedAt:     summary.UpdatedAt.Format(time.RFC3339),
			Items:         items,
		},
	}, nil
}

func (s *OrderGRPCServer) VerifyPurchase(ctx context.Context, req *orderv1.VerifyPurchaseRequest) (*orderv1.VerifyPurchaseResponse, error) {
	if req == nil || req.OrderUuid == "" {
		return nil, status.Error(codes.InvalidArgument, "order_uuid is required")
	}

	orderUUID, err := uuid.Parse(req.OrderUuid)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order_uuid format")
	}

	var userUUIDPtr *uuid.UUID
	if req.UserUuid != "" {
		if u, err := uuid.Parse(req.UserUuid); err == nil {
			userUUIDPtr = &u
		}
	}
	var productUUIDPtr *uuid.UUID
	if req.ProductUuid != "" {
		if p, err := uuid.Parse(req.ProductUuid); err == nil {
			productUUIDPtr = &p
		}
	}

	verification, err := s.provider.VerifyPurchase(ctx, orderUUID, req.UserId, req.ProductId, userUUIDPtr, productUUIDPtr)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to verify purchase: %v", err)
	}
	if verification == nil {
		return nil, status.Error(codes.NotFound, "order not found")
	}

	var deliveredAt string
	if verification.DeliveredAt != nil {
		deliveredAt = verification.DeliveredAt.Format(time.RFC3339)
	}

	return &orderv1.VerifyPurchaseResponse{
		OrderId:       verification.OrderID,
		OrderUuid:     verification.OrderUUID.String(),
		UserId:        verification.UserID,
		IsPurchased:   verification.IsPurchased,
		CanReview:     verification.CanReview,
		DeliveredAt:   deliveredAt,
		CustomerName:  verification.CustomerName,
		CustomerEmail: verification.CustomerEmail,
		ShippingName:  verification.ShippingName,
	}, nil
}

func (s *OrderGRPCServer) GetOrderForPayment(ctx context.Context, req *orderv1.GetOrderForPaymentRequest) (*orderv1.GetOrderForPaymentResponse, error) {
	if req == nil || req.OrderUuid == "" {
		return nil, status.Error(codes.InvalidArgument, "order_uuid is required")
	}

	orderUUID, err := uuid.Parse(req.OrderUuid)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order_uuid format")
	}

	info, err := s.provider.GetOrderForPayment(ctx, orderUUID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get order for payment: %v", err)
	}
	if info == nil {
		return nil, status.Error(codes.NotFound, "order not found")
	}

	return &orderv1.GetOrderForPaymentResponse{
		OrderUuid:   info.OrderUUID.String(),
		UserId:      info.UserID,
		UserUuid:    info.UserUUID.String(),
		TotalAmount: info.TotalAmount,
		Status:      info.Status,
	}, nil
}
