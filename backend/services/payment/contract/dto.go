package contract

import (
	"github.com/google/uuid"
)

type CreatePaymentIntentInput struct {
	UserID         int32     `json:"user_id"`
	OrderUUID      uuid.UUID `json:"order_uuid" binding:"required,uuid"`
	Provider       string    `json:"provider" binding:"required,oneof=momo cod vnpay"`
	ClientIP       string    `json:"client_ip"`
	IdempotencyKey string    `json:"idempotency_key"`
}

// Amount fields are integer minor units (xu: 1 dong = 100 xu), matching the
// payment service's internal money type. Never a float: this is a money value
// that a refund limit depends on.
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
