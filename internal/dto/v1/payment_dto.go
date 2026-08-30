package v1dto

import (
	"goshop/internal/db/sqlc"
	"goshop/internal/utils"
	"time"

	"github.com/google/uuid"
)

type CreatePaymentIntentInput struct {
	OrderUUID uuid.UUID `json:"order_uuid" binding:"required,uuid"`
	Provider  string    `json:"provider" binding:"required,oneof=vnpay momo"`
}

type PaymentIntentDTO struct {
	PaymentUUID string  `json:"payment_uuid"`
	OrderUUID   string  `json:"order_uuid"`
	Provider    string  `json:"provider"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Status      string  `json:"status"`
	PaymentURL  string  `json:"payment_url"`
	CreatedAt   string  `json:"created_at"`
}

type PaymentTransactionDTO struct {
	UUID          string   `json:"uuid"`
	OrderUUID     string   `json:"order_uuid"`
	Provider      string   `json:"provider"`
	ProviderTxnID *string  `json:"provider_txn_id,omitempty"`
	Amount        float64  `json:"amount"`
	Currency      string   `json:"currency"`
	Status        string   `json:"status"`
	RefundOf      *string  `json:"refund_of,omitempty"`
	RefundAmount  *float64 `json:"refund_amount,omitempty"`
	PaidAt        *string  `json:"paid_at,omitempty"`
	CreatedAt     string   `json:"created_at"`
}

type PaymentWebhookMessage struct {
	WebhookEventID int32             `json:"webhook_event_id"`
	Provider       string            `json:"provider"`
	EventType      string            `json:"event_type"`
	ProviderTxnID  string            `json:"provider_txn_id"`
	OrderNumber    string            `json:"order_number"`
	Amount         float64           `json:"amount"`
	Status         string            `json:"status"`
	Metadata       map[string]string `json:"metadata"`
}

func MapToPaymentIntentDTO(pt sqlc.PaymentTransaction, orderUUID uuid.UUID) PaymentIntentDTO {
	var paymentURL string
	if pt.PtProviderUrl != nil {
		paymentURL = *pt.PtProviderUrl
	}

	return PaymentIntentDTO{
		PaymentUUID: pt.PtUuid.String(),
		OrderUUID:   orderUUID.String(),
		Provider:    pt.PtProvider,
		Amount:      utils.NumericToFloat(pt.PtAmount),
		Currency:    pt.PtCurrency,
		Status:      MapPaymentTxnStatusToString(pt.PtStatus),
		PaymentURL:  paymentURL,
		CreatedAt:   pt.PtCreatedAt.Format(time.RFC3339),
	}
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
