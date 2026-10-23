package provider

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

type RefundRequest struct {
	OrderNumber     string
	ProviderTxnID   string  // Mã giao dịch ngân hàng gốc từ gateway (ví dụ: "2345678901" MoMo hoặc "14567890" VNPay)
	Amount          float64 // Số tiền yêu cầu hoàn (VND)
	TotalAmount     float64 // Tổng tiền đơn hàng gốc
	Reason          string
	TransactionDate string
	CreatedBy       string
	ClientIP        string
}
type RefundResponse struct {
	RefundTxnID string
	Amount      float64
	Status      TransactionStatus // "success" / "failed"
	Message     string            // Thông báo từ Gateway
}

type PaymentProvider interface {
	CreatePaymentURL(ctx context.Context, req CreatePaymentRequest) (CreatePaymentResponse, error)
	VerifyWebhook(payload []byte, headers map[string]string) error
	ParseWebhookPayload(payload []byte) (WebhookResult, error)
	// QueryTransactionStatus asks the provider for the authoritative status of a
	// transaction. txnRef is the merchant reference (our order number) and
	// transactionDate is the provider-formatted transaction date; both are
	// required because providers cannot resolve a transaction from our txn ref
	// without the date it was created.
	QueryTransactionStatus(ctx context.Context, txnRef, transactionDate string) (TransactionStatus, error)
	Refund(ctx context.Context, req RefundRequest) (RefundResponse, error)
}
