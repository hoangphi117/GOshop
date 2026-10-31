package dto

import (
	"encoding/json"
	"fmt"
	"time"

	paymentsqlc "goshop/services/payment/internal/db/sqlc"
	"goshop/services/payment/money"

	"github.com/google/uuid"
)

type CreatePaymentIntentInput struct {
	UserID         int32     `json:"user_id"`
	OrderUUID      uuid.UUID `json:"order_uuid" binding:"required,uuid"`
	Provider       string    `json:"provider" binding:"required,oneof=momo cod vnpay"`
	ClientIP       string    `json:"client_ip"`
	IdempotencyKey string    `json:"idempotency_key"`
}

// Amount fields carry integer minor units (xu, see internal/money). Storing a
// float here is what let binary floating point reach refund-limit checks.
type PaymentIntentDTO struct {
	PaymentUUID string `json:"payment_uuid"`
	OrderUUID   string `json:"order_uuid"`
	Provider    string `json:"provider"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	Status      string `json:"status"`
	PaymentURL  string `json:"payment_url"`
	CreatedAt   string `json:"created_at"`
}

type PaymentTransactionDTO struct {
	UUID          string  `json:"uuid"`
	OrderUUID     string  `json:"order_uuid"`
	Provider      string  `json:"provider"`
	ProviderTxnID *string `json:"provider_txn_id,omitempty"`
	Amount        int64   `json:"amount"`
	Currency      string  `json:"currency"`
	Status        string  `json:"status"`
	RefundOf      *string `json:"refund_of,omitempty"`
	RefundAmount  *int64  `json:"refund_amount,omitempty"`
	PaidAt        *string `json:"paid_at,omitempty"`
	CreatedAt     string  `json:"created_at"`
}

type RefundPaymentInput struct {
	IdempotencyKey string `json:"idempotency_key"`
	OrderUUID      string `json:"order_uuid" binding:"required,uuid"`
	Amount         int64  `json:"amount" binding:"required,gt=0"`
	Reason         string `json:"reason" binding:"required,max=500"`
}

type PaymentWebhookMessage struct {
	WebhookEventID int32             `json:"webhook_event_id"`
	OrderUUID      string            `json:"order_uuid"`
	Provider       string            `json:"provider"`
	EventType      string            `json:"event_type"`
	ProviderTxnID  string            `json:"provider_txn_id"`
	OrderNumber    string            `json:"order_number"`
	Amount         int64             `json:"amount"`
	Status         string            `json:"status"`
	Metadata       map[string]string `json:"metadata"`
}

type PaymentOrderSummary struct {
	OrderID             int32     `json:"order_id"`
	OrderUUID           uuid.UUID `json:"order_uuid"`
	OrderNumber         string    `json:"order_number"`
	UserID              int32     `json:"user_id"`
	UserUUID            uuid.UUID `json:"user_uuid"`
	TotalAmount         float64   `json:"total_amount"`
	PaymentStatus       int32     `json:"payment_status"`
	OrderStatus         int32     `json:"order_status"`
	PaymentMethod       int32     `json:"payment_method"`
	OrderRefundedAmount float64   `json:"order_refunded_amount"`
	CreatedAt           time.Time `json:"created_at"`
}

// MapToPaymentIntentDTO converts a ledger row to the API shape. It fails when
// the stored amount cannot be represented exactly instead of reporting zero.
func MapToPaymentIntentDTO(pt paymentsqlc.PaymentTransaction, orderUUID uuid.UUID) (PaymentIntentDTO, error) {
	amount, err := money.NumericToAmount(pt.PtAmount)
	if err != nil {
		return PaymentIntentDTO{}, fmt.Errorf("payment %s has an unreadable amount: %w", pt.PtUuid, err)
	}

	var paymentURL string
	if pt.PtProviderUrl != nil {
		paymentURL = *pt.PtProviderUrl
	}

	return PaymentIntentDTO{
		PaymentUUID: pt.PtUuid.String(),
		OrderUUID:   orderUUID.String(),
		Provider:    pt.PtProvider,
		Amount:      amount.MinorUnits(),
		Currency:    pt.PtCurrency,
		Status:      MapPaymentTxnStatusToString(pt.PtStatus),
		PaymentURL:  paymentURL,
		CreatedAt:   pt.PtCreatedAt.Format(time.RFC3339),
	}, nil
}

func MapToPaymentTransactionDTO(pt paymentsqlc.PaymentTransaction, orderUUID uuid.UUID) (PaymentTransactionDTO, error) {
	amount, err := money.NumericToAmount(pt.PtAmount)
	if err != nil {
		return PaymentTransactionDTO{}, fmt.Errorf("payment %s has an unreadable amount: %w", pt.PtUuid, err)
	}

	var refundOf *string
	if pt.PtRefundOf != nil {
		str := fmt.Sprintf("%d", *pt.PtRefundOf)
		refundOf = &str
	}

	var refundAmount *int64
	if pt.PtRefundAmount.Valid {
		refunded, refundErr := money.NumericToAmount(pt.PtRefundAmount)
		if refundErr != nil {
			return PaymentTransactionDTO{}, fmt.Errorf("payment %s has an unreadable refund amount: %w", pt.PtUuid, refundErr)
		}
		val := refunded.MinorUnits()
		refundAmount = &val
	}

	var paidAt *string
	if pt.PtPaidAt.Valid {
		formatted := pt.PtPaidAt.Time.Format(time.RFC3339)
		paidAt = &formatted
	}

	return PaymentTransactionDTO{
		UUID:          pt.PtUuid.String(),
		OrderUUID:     orderUUID.String(),
		Provider:      pt.PtProvider,
		ProviderTxnID: pt.PtProviderTxnID,
		Amount:        amount.MinorUnits(),
		Currency:      pt.PtCurrency,
		Status:        MapPaymentTxnStatusToString(pt.PtStatus),
		RefundOf:      refundOf,
		RefundAmount:  refundAmount,
		PaidAt:        paidAt,
		CreatedAt:     pt.PtCreatedAt.Format(time.RFC3339),
	}, nil
}

func MapPaymentTxnStatusToString(status int32) string {
	switch status {
	case 1:
		return "Pending"
	case 2:
		return "Paid"
	case 3:
		return "Failed"
	case 4:
		return "Expired"
	case 5:
		return "Refunded"
	default:
		return "Unknown"
	}
}

// PaymentEventPayload represents event payload for RabbitMQ topic
type PaymentEventPayload struct {
	PaymentUUID   string          `json:"payment_uuid"`
	OrderUUID     string          `json:"order_uuid"`
	OrderNumber   string          `json:"order_number"`
	UserUUID      string          `json:"user_uuid"`
	Provider      string          `json:"provider"`
	ProviderTxnID string          `json:"provider_txn_id"`
	Amount        float64         `json:"amount"`
	Currency      string          `json:"currency"`
	Status        string          `json:"status"`
	Reason        string          `json:"reason,omitempty"`
	Timestamp     time.Time       `json:"timestamp"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
}
