package grpc

import (
	"context"

	"goshop/services/payment/internal/dto"
	"goshop/services/payment/internal/service"
	paymentv1 "goshop/shared/proto/payment/v1"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PaymentGRPCServer struct {
	paymentv1.UnimplementedPaymentServiceServer
	paymentService service.PaymentService
}

func NewPaymentGRPCServer(paymentService service.PaymentService) *PaymentGRPCServer {
	return &PaymentGRPCServer{
		paymentService: paymentService,
	}
}

func (s *PaymentGRPCServer) CreatePaymentIntent(ctx context.Context, req *paymentv1.CreatePaymentIntentRequest) (*paymentv1.CreatePaymentIntentResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	orderUUID, err := uuid.Parse(req.OrderUuid)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid order_uuid %q: %v", req.OrderUuid, err)
	}

	intent, err := s.paymentService.CreatePaymentIntent(ctx, dto.CreatePaymentIntentInput{
		UserID:         req.UserId,
		OrderUUID:      orderUUID,
		Provider:       req.Provider,
		ClientIP:       req.ClientIp,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create payment intent: %v", err)
	}

	return &paymentv1.CreatePaymentIntentResponse{
		Intent: &paymentv1.PaymentIntentDTO{
			PaymentUuid: intent.PaymentUUID,
			OrderUuid:   intent.OrderUUID,
			Provider:    intent.Provider,
			Amount:      intent.Amount,
			Currency:    intent.Currency,
			Status:      intent.Status,
			PaymentUrl:  intent.PaymentURL,
			CreatedAt:   intent.CreatedAt,
		},
	}, nil
}

func (s *PaymentGRPCServer) GetPaymentTransactionsByOrderID(ctx context.Context, req *paymentv1.GetPaymentTransactionsByOrderIDRequest) (*paymentv1.GetPaymentTransactionsByOrderIDResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	txns, err := s.paymentService.GetPaymentTransactionsByOrderID(ctx, req.OrderId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get transactions: %v", err)
	}

	pbTxns := make([]*paymentv1.PaymentTransactionDTO, len(txns))
	for i, t := range txns {
		pbTxns[i] = &paymentv1.PaymentTransactionDTO{
			Uuid:          t.UUID,
			OrderUuid:     t.OrderUUID,
			Provider:      t.Provider,
			ProviderTxnId: t.ProviderTxnID,
			Amount:        t.Amount,
			Currency:      t.Currency,
			Status:        t.Status,
			RefundOf:      t.RefundOf,
			RefundAmount:  t.RefundAmount,
			PaidAt:        t.PaidAt,
			CreatedAt:     t.CreatedAt,
		}
	}

	return &paymentv1.GetPaymentTransactionsByOrderIDResponse{
		Transactions: pbTxns,
	}, nil
}

func (s *PaymentGRPCServer) ConfirmCODPayment(ctx context.Context, req *paymentv1.ConfirmCODPaymentRequest) (*paymentv1.ConfirmCODPaymentResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	orderUUID, err := uuid.Parse(req.OrderUuid)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid order_uuid %q: %v", req.OrderUuid, err)
	}

	if err := s.paymentService.ConfirmCODPayment(ctx, orderUUID); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to confirm COD payment: %v", err)
	}

	return &paymentv1.ConfirmCODPaymentResponse{
		Success: true,
	}, nil
}

func (s *PaymentGRPCServer) RefundPayment(ctx context.Context, req *paymentv1.RefundPaymentRequest) (*paymentv1.RefundPaymentResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	txn, err := s.paymentService.RefundPayment(ctx, dto.RefundPaymentInput{
		IdempotencyKey: req.IdempotencyKey,
		OrderUUID:      req.OrderUuid,
		Amount:         req.Amount,
		Reason:         req.Reason,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to refund payment: %v", err)
	}

	return &paymentv1.RefundPaymentResponse{
		Transaction: &paymentv1.PaymentTransactionDTO{
			Uuid:          txn.UUID,
			OrderUuid:     txn.OrderUUID,
			Provider:      txn.Provider,
			ProviderTxnId: txn.ProviderTxnID,
			Amount:        txn.Amount,
			Currency:      txn.Currency,
			Status:        txn.Status,
			RefundOf:      txn.RefundOf,
			RefundAmount:  txn.RefundAmount,
			PaidAt:        txn.PaidAt,
			CreatedAt:     txn.CreatedAt,
		},
	}, nil
}
