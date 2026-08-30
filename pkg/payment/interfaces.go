package payment

import "context"

type TransactionStatus string

const (
	TxnStatusPending TransactionStatus = "pending"
	TxnStatusSuccess TransactionStatus = "success"
	TxnStatusFailed  TransactionStatus = "failed"
	TxnStatusExpired TransactionStatus = "expired"
)

type CreatePaymentRequest struct {
	OrderNumber string
	Amount      float64
	Currency    string
	Description string
	ReturnURL   string
	WebhookURL  string
	ClientIP    string
}

type CreatePaymentResponse struct {
	ProviderTxnID string
	PaymentURL    string
}

type WebhookResult struct {
	EventID       string
	EventType     string
	ProviderTxnID string
	OrderNumber   string
	Amount        float64
	Status        TransactionStatus
	Metadata      map[string]string
}

type PaymentProvider interface {
	CreatePaymentURL(ctx context.Context, req CreatePaymentRequest) (CreatePaymentResponse, error)
	VerifyWebhook(payload []byte, headers map[string]string) error
	ParseWebhookPayload(payload []byte) (WebhookResult, error)
	QueryTransactionStatus(ctx context.Context, providerTxnID string) (TransactionStatus, error)
}
