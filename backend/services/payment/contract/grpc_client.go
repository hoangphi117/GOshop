package contract

import (
	"context"
	"fmt"

	paymentv1 "goshop/proto/payment/v1"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type PaymentClient interface {
	CreatePaymentIntent(ctx context.Context, input CreatePaymentIntentInput) (PaymentIntentDTO, error)
	GetPaymentTransactionsByOrderID(ctx context.Context, orderID int32) ([]PaymentTransactionDTO, error)
	ConfirmCODPayment(ctx context.Context, orderUUID uuid.UUID) error
	RefundPayment(ctx context.Context, input RefundPaymentInput) (PaymentTransactionDTO, error)
	Close() error
}

type GRPCPaymentClient struct {
	client paymentv1.PaymentServiceClient
	conn   *grpc.ClientConn
}

func NewGRPCPaymentClient(target string) (*GRPCPaymentClient, error) {
	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc client for payment at %s: %w", target, err)
	}

	return &GRPCPaymentClient{
		client: paymentv1.NewPaymentServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *GRPCPaymentClient) Close() error {
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			return fmt.Errorf("failed to close grpc connection: %w", err)
		}
	}
	return nil
}

func (c *GRPCPaymentClient) CreatePaymentIntent(ctx context.Context, input CreatePaymentIntentInput) (PaymentIntentDTO, error) {
	resp, err := c.client.CreatePaymentIntent(ctx, &paymentv1.CreatePaymentIntentRequest{
		UserId:         input.UserID,
		ClientIp:       input.ClientIP,
		IdempotencyKey: input.IdempotencyKey,
		OrderUuid:      input.OrderUUID.String(),
		Provider:       input.Provider,
	})
	if err != nil {
		return PaymentIntentDTO{}, fmt.Errorf("grpc CreatePaymentIntent failed: %w", err)
	}

	intent := resp.GetIntent()
	if intent == nil {
		return PaymentIntentDTO{}, fmt.Errorf("received nil intent from payment grpc server")
	}

	return PaymentIntentDTO{
		PaymentUUID: intent.GetPaymentUuid(),
		OrderUUID:   intent.GetOrderUuid(),
		Provider:    intent.GetProvider(),
		Amount:      intent.GetAmount(),
		Currency:    intent.GetCurrency(),
		Status:      intent.GetStatus(),
		PaymentURL:  intent.GetPaymentUrl(),
		CreatedAt:   intent.GetCreatedAt(),
	}, nil
}

func (c *GRPCPaymentClient) GetPaymentTransactionsByOrderID(ctx context.Context, orderID int32) ([]PaymentTransactionDTO, error) {
	resp, err := c.client.GetPaymentTransactionsByOrderID(ctx, &paymentv1.GetPaymentTransactionsByOrderIDRequest{
		OrderId: orderID,
	})
	if err != nil {
		return nil, fmt.Errorf("grpc GetPaymentTransactionsByOrderID failed: %w", err)
	}

	pbTxns := resp.GetTransactions()
	result := make([]PaymentTransactionDTO, len(pbTxns))
	for i, t := range pbTxns {
		result[i] = PaymentTransactionDTO{
			UUID:          t.GetUuid(),
			OrderUUID:     t.GetOrderUuid(),
			Provider:      t.GetProvider(),
			ProviderTxnID: t.ProviderTxnId,
			Amount:        t.GetAmount(),
			Currency:      t.GetCurrency(),
			Status:        t.GetStatus(),
			RefundOf:      t.RefundOf,
			RefundAmount:  t.RefundAmount,
			PaidAt:        t.PaidAt,
			CreatedAt:     t.GetCreatedAt(),
		}
	}
	return result, nil
}

func (c *GRPCPaymentClient) ConfirmCODPayment(ctx context.Context, orderUUID uuid.UUID) error {
	resp, err := c.client.ConfirmCODPayment(ctx, &paymentv1.ConfirmCODPaymentRequest{
		OrderUuid: orderUUID.String(),
	})
	if err != nil {
		return fmt.Errorf("grpc ConfirmCODPayment failed: %w", err)
	}

	if !resp.GetSuccess() {
		return fmt.Errorf("payment grpc server indicated COD payment confirmation failure")
	}
	return nil
}

func (c *GRPCPaymentClient) RefundPayment(ctx context.Context, input RefundPaymentInput) (PaymentTransactionDTO, error) {
	resp, err := c.client.RefundPayment(ctx, &paymentv1.RefundPaymentRequest{
		IdempotencyKey: input.IdempotencyKey,
		OrderUuid:      input.OrderUUID,
		Amount:         input.Amount,
		Reason:         input.Reason,
	})
	if err != nil {
		return PaymentTransactionDTO{}, fmt.Errorf("grpc RefundPayment failed: %w", err)
	}

	t := resp.GetTransaction()
	if t == nil {
		return PaymentTransactionDTO{}, fmt.Errorf("received nil transaction response from payment grpc server")
	}

	return PaymentTransactionDTO{
		UUID:          t.GetUuid(),
		OrderUUID:     t.GetOrderUuid(),
		Provider:      t.GetProvider(),
		ProviderTxnID: t.ProviderTxnId,
		Amount:        t.GetAmount(),
		Currency:      t.GetCurrency(),
		Status:        t.GetStatus(),
		RefundOf:      t.RefundOf,
		RefundAmount:  t.RefundAmount,
		PaidAt:        t.PaidAt,
		CreatedAt:     t.GetCreatedAt(),
	}, nil
}
