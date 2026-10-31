package provider

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"goshop/pkg/logger"
	"goshop/services/payment/money"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type momoProvider struct {
	config MoMoConfig
	client *http.Client
}

func NewMoMoProvider(config MoMoConfig) PaymentProvider {
	return &momoProvider{
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

type momoCreateRequest struct {
	PartnerCode string `json:"partnerCode"`
	AccessKey   string `json:"accessKey"`
	RequestType string `json:"requestType"`
	IpnURL      string `json:"ipnUrl"`
	RedirectURL string `json:"redirectUrl"`
	OrderID     string `json:"orderId"`
	Amount      int64  `json:"amount"`
	OrderInfo   string `json:"orderInfo"`
	RequestID   string `json:"requestId"`
	ExtraData   string `json:"extraData"`
	Signature   string `json:"signature"`
	Lang        string `json:"lang"`
}

type momoCreateResponse struct {
	PartnerCode  string `json:"partnerCode"`
	OrderID      string `json:"orderId"`
	RequestID    string `json:"requestId"`
	Amount       int64  `json:"amount"`
	PayURL       string `json:"payUrl"`
	ResultCode   int    `json:"resultCode"`
	Message      string `json:"message"`
	ResponseTime int64  `json:"responseTime"`
}

type momoRefundRequest struct {
	PartnerCode string `json:"partnerCode"`
	OrderID     string `json:"orderId"`   // Mã đơn hoàn (unique request: e.g. orderNumber-refund-timestamp)
	RequestID   string `json:"requestId"` // Unique request id
	Amount      int64  `json:"amount"`    // Số tiền hoàn
	TransID     int64  `json:"transId"`   // Mã giao dịch thanh toán gốc của MoMo
	Lang        string `json:"lang"`
	Description string `json:"description"` // Lý do hoàn
	Signature   string `json:"signature"`
}
type momoRefundResponse struct {
	PartnerCode  string `json:"partnerCode"`
	OrderID      string `json:"orderId"`
	RequestID    string `json:"requestId"`
	Amount       int64  `json:"amount"`
	TransID      int64  `json:"transId"` // Mã giao dịch refund từ MoMo
	ResultCode   int    `json:"resultCode"`
	Message      string `json:"message"`
	ResponseTime int64  `json:"responseTime"`
}

func (p *momoProvider) CreatePaymentURL(ctx context.Context, req CreatePaymentRequest) (CreatePaymentResponse, error) {
	requestID := fmt.Sprintf("%s-%d", req.OrderNumber, time.Now().UnixNano())
	// MoMo's API speaks whole dong; refuse to send a truncated amount.
	amountInt, err := req.Amount.WholeDong()
	if err != nil {
		return CreatePaymentResponse{}, fmt.Errorf("momo requires a whole-dong amount: %w", err)
	}
	requestType := "payWithMethod"
	extraData := ""

	rawSignature := fmt.Sprintf(
		"accessKey=%s&amount=%d&extraData=%s&ipnUrl=%s&orderId=%s&orderInfo=%s&partnerCode=%s&redirectUrl=%s&requestId=%s&requestType=%s",
		p.config.AccessKey, amountInt, extraData, req.WebhookURL, req.OrderNumber, req.Description, p.config.PartnerCode, req.ReturnURL, requestID, requestType,
	)

	signature := hmacSHA256(p.config.SecretKey, rawSignature)

	momoReq := momoCreateRequest{
		PartnerCode: p.config.PartnerCode,
		AccessKey:   p.config.AccessKey,
		RequestType: requestType,
		IpnURL:      req.WebhookURL,
		RedirectURL: req.ReturnURL,
		OrderID:     req.OrderNumber,
		Amount:      amountInt,
		OrderInfo:   req.Description,
		RequestID:   requestID,
		ExtraData:   extraData,
		Signature:   signature,
		Lang:        "vi",
	}

	bodyBytes, err := json.Marshal(momoReq)
	if err != nil {
		return CreatePaymentResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.Endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return CreatePaymentResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return CreatePaymentResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return CreatePaymentResponse{}, fmt.Errorf("failed to read momo response: %w", err)
	}

	var momoResp momoCreateResponse
	if err := json.Unmarshal(respBody, &momoResp); err != nil {
		snippet := string(respBody)
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		return CreatePaymentResponse{}, fmt.Errorf("momo gateway returned invalid JSON (HTTP %d): %s", resp.StatusCode, snippet)
	}

	if momoResp.ResultCode != 0 {
		return CreatePaymentResponse{}, fmt.Errorf("momo error: %s (code %d)", momoResp.Message, momoResp.ResultCode)
	}

	return CreatePaymentResponse{
		ProviderTxnID: momoResp.RequestID,
		PaymentURL:    momoResp.PayURL,
	}, nil
}

type momoWebhookPayload struct {
	PartnerCode  string `json:"partnerCode"`
	OrderID      string `json:"orderId"`
	RequestID    string `json:"requestId"`
	Amount       int64  `json:"amount"`
	OrderInfo    string `json:"orderInfo"`
	OrderType    string `json:"orderType"`
	TransID      int64  `json:"transId"`
	ResultCode   int    `json:"resultCode"`
	Message      string `json:"message"`
	PayType      string `json:"payType"`
	ResponseTime int64  `json:"responseTime"`
	ExtraData    string `json:"extraData"`
	Signature    string `json:"signature"`
}

func (p *momoProvider) VerifyWebhook(payload []byte, headers map[string]string) error {
	var body momoWebhookPayload
	if err := json.Unmarshal(payload, &body); err != nil {
		return errors.New("invalid momo webhook json format")
	}

	rawSignature := fmt.Sprintf(
		"accessKey=%s&amount=%d&extraData=%s&message=%s&orderId=%s&orderInfo=%s&orderType=%s&partnerCode=%s&payType=%s&requestId=%s&responseTime=%d&resultCode=%d&transId=%d",
		p.config.AccessKey, body.Amount, body.ExtraData, body.Message, body.OrderID, body.OrderInfo, body.OrderType,
		body.PartnerCode, body.PayType, body.RequestID, body.ResponseTime, body.ResultCode, body.TransID,
	)

	expectedSignature := hmacSHA256(p.config.SecretKey, rawSignature)
	if !strings.EqualFold(body.Signature, expectedSignature) {
		return errors.New("invalid signature checksum from momo")
	}
	return nil
}

func (p *momoProvider) ParseWebhookPayload(payload []byte) (WebhookResult, error) {
	var body momoWebhookPayload
	if err := json.Unmarshal(payload, &body); err != nil {
		return WebhookResult{}, errors.New("invalid momo webhook json format")
	}

	// MoMo reports whole dong; an out-of-range value is a malformed callback,
	// not something to clamp.
	momoAmount, err := money.FromWholeDong(body.Amount)
	if err != nil {
		return WebhookResult{}, fmt.Errorf("invalid momo webhook amount %d: %w", body.Amount, err)
	}

	status := TxnStatusFailed
	if body.ResultCode == 0 {
		status = TxnStatusSuccess
	}

	return WebhookResult{
		EventID:       fmt.Sprintf("momo_%d_%s", body.TransID, body.OrderID),
		EventType:     fmt.Sprintf("payment.%s", status),
		ProviderTxnID: fmt.Sprintf("%d", body.TransID),
		OrderNumber:   body.OrderID,
		Status:        status,
		Amount:        momoAmount,
		Metadata: map[string]string{
			"pay_type":   body.PayType,
			"order_type": body.OrderType,
			"message":    body.Message,
		},
	}, nil
}

type momoQueryRequest struct {
	PartnerCode string `json:"partnerCode"`
	OrderID     string `json:"orderId"`
	RequestID   string `json:"requestId"`
	Lang        string `json:"lang"`
	Signature   string `json:"signature"`
}

type momoQueryResponse struct {
	PartnerCode  string `json:"partnerCode"`
	OrderID      string `json:"orderId"`
	RequestID    string `json:"requestId"`
	ExtraData    string `json:"extraData"`
	Amount       int64  `json:"amount"`
	TransID      int64  `json:"transId"`
	PayType      string `json:"payType"`
	ResultCode   int    `json:"resultCode"`
	Message      string `json:"message"`
	ResponseTime int64  `json:"responseTime"`
}

// QueryTransactionStatus asks MoMo for the authoritative status of an order.
// MoMo keys lookups on orderId only, so transactionDate is unused here.
func (p *momoProvider) QueryTransactionStatus(ctx context.Context, orderNumber, transactionDate string) (TransactionStatus, error) {
	_ = transactionDate
	requestID := fmt.Sprintf("req-query-%s-%d", orderNumber, time.Now().UnixNano())
	rawSignature := fmt.Sprintf(
		"accessKey=%s&orderId=%s&partnerCode=%s&requestId=%s",
		p.config.AccessKey, orderNumber, p.config.PartnerCode, requestID,
	)
	signature := hmacSHA256(p.config.SecretKey, rawSignature)

	payload := momoQueryRequest{
		PartnerCode: p.config.PartnerCode,
		OrderID:     orderNumber,
		RequestID:   requestID,
		Lang:        "vi",
		Signature:   signature,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return TxnStatusFailed, fmt.Errorf("failed to marshal query payload: %w", err)
	}

	queryURL := p.config.QueryURL
	if queryURL == "" {
		queryURL = "https://test-payment.momo.vn/v2/gateway/api/query"
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", queryURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return TxnStatusFailed, fmt.Errorf("failed to create query request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return TxnStatusFailed, fmt.Errorf("failed to execute query request to MoMo: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return TxnStatusFailed, fmt.Errorf("failed to read momo query response: %w", err)
	}

	var queryResp momoQueryResponse
	if err := json.Unmarshal(respBody, &queryResp); err != nil {
		snippet := string(respBody)
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		return TxnStatusFailed, fmt.Errorf("momo gateway returned invalid query response JSON (HTTP %d): %s", resp.StatusCode, snippet)
	}

	logger.Log.Info().
		Str("order_id", orderNumber).
		Int("result_code", queryResp.ResultCode).
		Str("message", queryResp.Message).
		Int64("trans_id", queryResp.TransID).
		Msg("MoMo query transaction status response")

	switch queryResp.ResultCode {
	case 0:
		return TxnStatusSuccess, nil
	case 1000:
		return TxnStatusPending, nil
	default:
		return TxnStatusExpired, nil
	}
}

func hmacSHA256(key, data string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func (p *momoProvider) Refund(ctx context.Context, req RefundRequest) (RefundResponse, error) {
	transIDInt, err := strconv.ParseInt(req.ProviderTxnID, 10, 64)
	if err != nil {
		return RefundResponse{}, fmt.Errorf("invalid MoMo original transId: %v", err)
	}

	requestID := fmt.Sprintf("req-ref-%s-%d", req.OrderNumber, time.Now().UnixNano())
	refundOrderID := fmt.Sprintf("ref-%s-%d", req.OrderNumber, time.Now().Unix())
	amountInt, err := req.Amount.WholeDong()
	if err != nil {
		return RefundResponse{}, fmt.Errorf("momo requires a whole-dong refund amount: %w", err)
	}
	description := req.Reason
	if description == "" {
		description = fmt.Sprintf("Refund for order %s", req.OrderNumber)
	}

	rawSignature := fmt.Sprintf(
		"accessKey=%s&amount=%d&description=%s&orderId=%s&partnerCode=%s&requestId=%s&transId=%d",
		p.config.AccessKey, amountInt, description, refundOrderID, p.config.PartnerCode, requestID, transIDInt,
	)

	signature := hmacSHA256(p.config.SecretKey, rawSignature)

	refundPayload := momoRefundRequest{
		PartnerCode: p.config.PartnerCode,
		OrderID:     refundOrderID,
		RequestID:   requestID,
		Amount:      amountInt,
		TransID:     transIDInt,
		Lang:        "vi",
		Description: description,
		Signature:   signature,
	}

	bodyBytes, err := json.Marshal(refundPayload)
	if err != nil {
		return RefundResponse{}, err
	}

	refundURL := strings.Replace(p.config.Endpoint, "/create", "/refund", 1)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, refundURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return RefundResponse{}, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return RefundResponse{}, fmt.Errorf("failed to call MoMo refund API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return RefundResponse{}, fmt.Errorf("failed to read momo refund response: %w", err)
	}

	var momoResp momoRefundResponse
	if err := json.Unmarshal(respBody, &momoResp); err != nil {
		snippet := string(respBody)
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		return RefundResponse{}, fmt.Errorf("momo gateway returned invalid refund response JSON (HTTP %d): %s", resp.StatusCode, snippet)
	}

	if momoResp.ResultCode != 0 {
		return RefundResponse{
			Status:  TxnStatusFailed,
			Message: momoResp.Message,
		}, fmt.Errorf("momo refund rejected with code %d: %s", momoResp.ResultCode, momoResp.Message)
	}

	// Report the amount we actually asked for in whole dong; the gateway echo in
	// momoResp.Amount is informational only.
	return RefundResponse{
		RefundTxnID: strconv.FormatInt(momoResp.TransID, 10),
		Amount:      req.Amount,
		Status:      TxnStatusSuccess,
		Message:     momoResp.Message,
	}, nil
}
