package v1dto

import (
	"github.com/google/uuid"
)

type CreatePaymentIntentInput struct {
	OrderUUID uuid.UUID `json:"order_uuid" binding:"required,uuid"`
	Provider  string    `json:"provider" binding:"required,oneof=momo"`
}

type PaymentIntentDTO struct {
	PaymentUUID string `json:"payment_uuid"`
	OrderUUID   string `json:"order_uuid"`
	Provider    string `json:"provider"`
	// Amount is in integer minor units (xu: 1 dong = 100 xu), matching the
	// payment service contract. A float here reintroduces rounding into money.
	Amount     int64  `json:"amount"`
	Currency   string `json:"currency"`
	Status     string `json:"status"`
	PaymentURL string `json:"payment_url"`
	CreatedAt  string `json:"created_at"`
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

type GetTransactionHistoryByOrderUUID struct {
	OrderUUID string `uri:"order_uuid" binding:"uuid"`
}

type ConfirmCODPaymentInput struct {
	OrderUUID string `uri:"order_uuid" binding:"uuid"`
}

type RefundPaymentInput struct {
	OrderUUID string `json:"order_uuid" binding:"required,uuid"`
	// Amount is in integer minor units (xu), see PaymentIntentDTO.Amount.
	Amount int64  `json:"amount" binding:"required,gt=0,max=99999999999999999"`
	Reason string `json:"reason" binding:"required,max=500"`
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
