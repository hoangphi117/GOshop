package client

import (
	"context"
	"fmt"

	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/services/payment/contract"

	"github.com/google/uuid"
)

type (
	PaymentClient     = contract.PaymentClient
	GRPCPaymentClient = contract.GRPCPaymentClient
)

var (
	NewGRPCPaymentClient = contract.NewGRPCPaymentClient
)

var (
	_ v1service.PaymentService       = (*PaymentServiceAdapter)(nil)
	_ v1service.PaymentRefundService = (*PaymentServiceAdapter)(nil)
)

// PaymentServiceAdapter bridges the standalone Payment gRPC microservice into Monolith services
type PaymentServiceAdapter struct {
	client contract.PaymentClient
}

func NewPaymentServiceAdapter(client contract.PaymentClient) *PaymentServiceAdapter {
	return &PaymentServiceAdapter{client: client}
}

func (a *PaymentServiceAdapter) RefundPayment(ctx context.Context, idempotencyKey string, input v1dto.RefundPaymentInput) (v1dto.PaymentTransactionDTO, error) {
	if a.client == nil {
		return v1dto.PaymentTransactionDTO{}, fmt.Errorf("payment gRPC client is not initialized")
	}

	// Both the v1 DTO and the payment contract carry integer minor units (xu),
	// so the amount crosses unchanged.
	res, err := a.client.RefundPayment(ctx, contract.RefundPaymentInput{
		IdempotencyKey: idempotencyKey,
		OrderUUID:      input.OrderUUID,
		Amount:         input.Amount,
		Reason:         input.Reason,
	})
	if err != nil {
		return v1dto.PaymentTransactionDTO{}, fmt.Errorf("payment client refund failed: %w", err)
	}

	return v1dto.PaymentTransactionDTO{
		UUID:          res.UUID,
		OrderUUID:     res.OrderUUID,
		Provider:      res.Provider,
		ProviderTxnID: res.ProviderTxnID,
		Amount:        res.Amount,
		Currency:      res.Currency,
		Status:        res.Status,
		RefundOf:      res.RefundOf,
		RefundAmount:  res.RefundAmount,
		PaidAt:        res.PaidAt,
		CreatedAt:     res.CreatedAt,
	}, nil
}

func (a *PaymentServiceAdapter) CreatePaymentIntent(ctx context.Context, userID int32, clientIP, idempotencyKey string, input v1dto.CreatePaymentIntentInput) (v1dto.PaymentIntentDTO, error) {
	if a.client == nil {
		return v1dto.PaymentIntentDTO{}, fmt.Errorf("payment gRPC client is not initialized")
	}

	intent, err := a.client.CreatePaymentIntent(ctx, contract.CreatePaymentIntentInput{
		UserID:         userID,
		OrderUUID:      input.OrderUUID,
		Provider:       input.Provider,
		ClientIP:       clientIP,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return v1dto.PaymentIntentDTO{}, fmt.Errorf("failed to create payment intent via gRPC: %w", err)
	}

	return v1dto.PaymentIntentDTO{
		PaymentUUID: intent.PaymentUUID,
		OrderUUID:   intent.OrderUUID,
		Provider:    intent.Provider,
		Amount:      intent.Amount,
		Currency:    intent.Currency,
		Status:      intent.Status,
		PaymentURL:  intent.PaymentURL,
		CreatedAt:   intent.CreatedAt,
	}, nil
}

func (a *PaymentServiceAdapter) ConfirmCODPayment(ctx context.Context, orderUUID uuid.UUID) error {
	if a.client == nil {
		return fmt.Errorf("payment gRPC client is not initialized")
	}

	if err := a.client.ConfirmCODPayment(ctx, orderUUID); err != nil {
		return fmt.Errorf("failed to confirm COD payment via gRPC: %w", err)
	}
	return nil
}
