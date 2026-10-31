package provider

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"goshop/pkg/logger"
	"goshop/services/payment/money"
)

type vnpayProvider struct {
	config VNPayConfig
	client *http.Client
}

// NewVNPayProvider creates a new instance of VNPay PaymentProvider.
func NewVNPayProvider(config VNPayConfig) PaymentProvider {
	return &vnpayProvider{
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// CreatePaymentURL generates the redirect payment URL for VNPay.
func (p *vnpayProvider) CreatePaymentURL(ctx context.Context, req CreatePaymentRequest) (CreatePaymentResponse, error) {
	if p.config.TmnCode == "" || p.config.HashSecret == "" {
		return CreatePaymentResponse{}, errors.New("vnpay configuration missing TmnCode or HashSecret")
	}

	tz := time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)
	now := time.Now().In(tz)
	createDate := now.Format("20060102150405")
	expireDate := now.Add(15 * time.Minute).Format("20060102150405")

	clientIP := req.ClientIP
	if clientIP == "" || clientIP == "::1" {
		clientIP = "127.0.0.1"
	}

	// VNPay's vnp_Amount unit is exactly one xu, which is our Amount unit, so
	// the wire value is the amount itself: no multiplication, no rounding.
	params := url.Values{}
	params.Set("vnp_Version", "2.1.0")
	params.Set("vnp_Command", "pay")
	params.Set("vnp_TmnCode", p.config.TmnCode)
	params.Set("vnp_Amount", strconv.FormatInt(req.Amount.MinorUnits(), 10))
	params.Set("vnp_CreateDate", createDate)
	params.Set("vnp_CurrCode", "VND")
	params.Set("vnp_IpAddr", clientIP)
	params.Set("vnp_Locale", "vn")
	params.Set("vnp_OrderInfo", req.Description)
	params.Set("vnp_OrderType", "other")
	params.Set("vnp_ReturnUrl", req.ReturnURL)
	params.Set("vnp_TxnRef", req.OrderNumber)
	params.Set("vnp_ExpireDate", expireDate)

	signData := buildVNPaySignData(params)
	secureHash := hmacSHA512(p.config.HashSecret, signData)

	endpoint := p.config.Endpoint
	if endpoint == "" {
		endpoint = "https://sandbox.vnpayment.vn/paymentv2/vpcpay.html"
	}

	paymentURL := fmt.Sprintf("%s?%s&vnp_SecureHash=%s", endpoint, signData, secureHash)

	return CreatePaymentResponse{
		ProviderTxnID: req.OrderNumber,
		PaymentURL:    paymentURL,
	}, nil
}

// VerifyWebhook validates the HMAC-SHA512 signature of the VNPay IPN webhook payload.
func (p *vnpayProvider) VerifyWebhook(payload []byte, headers map[string]string) error {
	values, err := parseVNPayValues(payload)
	if err != nil {
		return fmt.Errorf("failed to parse VNPay webhook values: %w", err)
	}

	receivedHash := values.Get("vnp_SecureHash")
	if receivedHash == "" {
		return errors.New("missing vnp_SecureHash in VNPay webhook")
	}

	signData := buildVNPaySignData(values)
	expectedHash := hmacSHA512(p.config.HashSecret, signData)

	if !strings.EqualFold(receivedHash, expectedHash) {
		return errors.New("invalid VNPay webhook signature")
	}

	return nil
}

// ParseWebhookPayload extracts transaction details from VNPay IPN webhook.
func (p *vnpayProvider) ParseWebhookPayload(payload []byte) (WebhookResult, error) {
	values, err := parseVNPayValues(payload)
	if err != nil {
		return WebhookResult{}, fmt.Errorf("failed to parse VNPay webhook values: %w", err)
	}

	orderNumber := values.Get("vnp_TxnRef")
	providerTxnID := values.Get("vnp_TransactionNo")
	responseCode := values.Get("vnp_ResponseCode")
	txnStatus := values.Get("vnp_TransactionStatus")
	rawAmount := values.Get("vnp_Amount")

	var amount money.Amount
	if rawAmount != "" {
		// vnp_Amount is already in xu, which is exactly our minor unit.
		parsed, parseErr := strconv.ParseInt(rawAmount, 10, 64)
		if parseErr != nil {
			return WebhookResult{}, fmt.Errorf("invalid vnp_Amount (%s): %w", rawAmount, parseErr)
		}
		amount = money.FromMinorUnits(parsed)
	}

	status := TxnStatusFailed
	if responseCode == "00" && (txnStatus == "00" || txnStatus == "") {
		status = TxnStatusSuccess
	}

	metadata := make(map[string]string)
	for k := range values {
		metadata[k] = values.Get(k)
	}

	eventID := fmt.Sprintf("vnpay:%s:%s", orderNumber, providerTxnID)
	if providerTxnID == "" {
		eventID = fmt.Sprintf("vnpay:%s", orderNumber)
	}

	return WebhookResult{
		EventID:       eventID,
		EventType:     "vnpay.payment",
		ProviderTxnID: providerTxnID,
		OrderNumber:   orderNumber,
		Amount:        amount,
		Status:        status,
		Metadata:      metadata,
	}, nil
}

// QueryTransactionStatus asks VNPay's querydr API for the authoritative status of
// a transaction.
//
// txnRef is the merchant-side reference (vnp_TxnRef, i.e. our order number) and
// transactionDate is the transaction's create date in VNPay's yyMMddHHmmss
// format. VNPay matches on those two fields: it has no way to look a
// transaction up by providerTxnID alone, so callers must pass the stored
// merchant reference and its date. Statuses other than a definitive success or
// failure are reported as TxnStatusPending so callers never act on an
// inconclusive answer.
func (p *vnpayProvider) QueryTransactionStatus(ctx context.Context, txnRef, transactionDate string) (TransactionStatus, error) {
	if p.config.QueryURL == "" {
		return TxnStatusPending, nil
	}

	tz := time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)
	now := time.Now().In(tz)
	requestID := fmt.Sprintf("%d", now.UnixNano())
	createDate := now.Format("20060102150405")
	if transactionDate == "" {
		transactionDate = createDate
	}

	reqData := map[string]string{
		"vnp_RequestId":       requestID,
		"vnp_Version":         "2.1.0",
		"vnp_Command":         "querydr",
		"vnp_TmnCode":         p.config.TmnCode,
		"vnp_TxnRef":          txnRef,
		"vnp_OrderInfo":       fmt.Sprintf("Query status for order %s", txnRef),
		"vnp_TransactionDate": transactionDate,
		"vnp_CreateDate":      createDate,
		"vnp_IpAddr":          "127.0.0.1",
	}

	signRaw := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s",
		reqData["vnp_RequestId"],
		reqData["vnp_Version"],
		reqData["vnp_Command"],
		reqData["vnp_TmnCode"],
		reqData["vnp_TxnRef"],
		reqData["vnp_TransactionDate"],
		reqData["vnp_CreateDate"],
		reqData["vnp_IpAddr"],
		reqData["vnp_OrderInfo"],
	)
	reqData["vnp_SecureHash"] = hmacSHA512(p.config.HashSecret, signRaw)

	bodyBytes, err := json.Marshal(reqData)
	if err != nil {
		return TxnStatusPending, fmt.Errorf("failed to marshal querydr request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.QueryURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return TxnStatusPending, fmt.Errorf("failed to build querydr http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return TxnStatusPending, fmt.Errorf("querydr request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return TxnStatusPending, fmt.Errorf("failed to read querydr response: %w", err)
	}

	var result map[string]string
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return TxnStatusPending, fmt.Errorf("failed to parse querydr response json: %w", err)
	}

	if result["vnp_ResponseCode"] == "00" && result["vnp_TransactionStatus"] == "00" {
		return TxnStatusSuccess, nil
	}
	if result["vnp_TransactionStatus"] == "01" {
		return TxnStatusPending, nil
	}

	return TxnStatusFailed, nil
}

// Refund initiates a refund request to VNPay merchant API.
func (p *vnpayProvider) Refund(ctx context.Context, req RefundRequest) (RefundResponse, error) {
	if p.config.QueryURL == "" {
		return RefundResponse{
			Status:  TxnStatusFailed,
			Message: "VNPay QueryURL not configured for refunds",
		}, errors.New("vnpay query URL not configured")
	}

	tz := time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)
	now := time.Now().In(tz)
	requestID := fmt.Sprintf("%d", now.UnixNano())
	createDate := now.Format("20060102150405")

	transDate := req.TransactionDate
	if transDate == "" {
		transDate = createDate
	}

	clientIP := req.ClientIP
	if clientIP == "" || clientIP == "::1" {
		clientIP = "127.0.0.1"
	}

	createdBy := req.CreatedBy
	if createdBy == "" {
		createdBy = "system"
	}

	transactionType := "02" // 02: Full refund, 03: Partial refund
	if req.TotalAmount > 0 && req.Amount < req.TotalAmount {
		transactionType = "03"
	}

	// VNPay refund amounts are in xu, our native minor unit.
	vnpAmount := req.Amount.MinorUnits()

	reqData := map[string]string{
		"vnp_RequestId":       requestID,
		"vnp_Version":         "2.1.0",
		"vnp_Command":         "refund",
		"vnp_TmnCode":         p.config.TmnCode,
		"vnp_TransactionType": transactionType,
		"vnp_TxnRef":          req.OrderNumber,
		"vnp_Amount":          strconv.FormatInt(vnpAmount, 10),
		"vnp_OrderInfo":       req.Reason,
		"vnp_TransactionNo":   req.ProviderTxnID,
		"vnp_TransactionDate": transDate,
		"vnp_CreateBy":        createdBy,
		"vnp_CreateDate":      createDate,
		"vnp_IpAddr":          clientIP,
	}

	signRaw := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s",
		reqData["vnp_RequestId"],
		reqData["vnp_Version"],
		reqData["vnp_Command"],
		reqData["vnp_TmnCode"],
		reqData["vnp_TransactionType"],
		reqData["vnp_TxnRef"],
		reqData["vnp_Amount"],
		reqData["vnp_TransactionNo"],
		reqData["vnp_TransactionDate"],
		reqData["vnp_CreateBy"],
		reqData["vnp_CreateDate"],
		reqData["vnp_IpAddr"],
		reqData["vnp_OrderInfo"],
	)
	reqData["vnp_SecureHash"] = hmacSHA512(p.config.HashSecret, signRaw)

	bodyBytes, err := json.Marshal(reqData)
	if err != nil {
		return RefundResponse{Status: TxnStatusFailed}, fmt.Errorf("failed to marshal vnpay refund request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.QueryURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return RefundResponse{Status: TxnStatusFailed}, fmt.Errorf("failed to create vnpay refund http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return RefundResponse{Status: TxnStatusFailed}, fmt.Errorf("vnpay refund http request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return RefundResponse{Status: TxnStatusFailed}, fmt.Errorf("failed to read vnpay refund response: %w", err)
	}

	var result map[string]string
	if err := json.Unmarshal(respBytes, &result); err != nil {
		logger.Log.Error().Err(err).Str("body", string(respBytes)).Msg("Failed to unmarshal VNPay refund response")
		return RefundResponse{Status: TxnStatusFailed, Message: string(respBytes)}, fmt.Errorf("failed to parse vnpay refund json: %w", err)
	}

	responseCode := result["vnp_ResponseCode"]
	message := result["vnp_Message"]
	refundTxnID := result["vnp_TransactionNo"]

	if responseCode == "00" {
		return RefundResponse{
			RefundTxnID: refundTxnID,
			Amount:      req.Amount,
			Status:      TxnStatusSuccess,
			Message:     message,
		}, nil
	}

	return RefundResponse{
		RefundTxnID: refundTxnID,
		Amount:      req.Amount,
		Status:      TxnStatusFailed,
		Message:     fmt.Sprintf("VNPay refund failed with code %s: %s", responseCode, message),
	}, nil
}

// buildVNPaySignData creates query string sorted alphabetically by keys, excluding secure hash.
func buildVNPaySignData(values url.Values) string {
	var keys []string
	for k := range values {
		if k == "vnp_SecureHash" || k == "vnp_SecureHashType" {
			continue
		}
		if values.Get(k) != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, url.QueryEscape(values.Get(k))))
	}
	return strings.Join(pairs, "&")
}

// hmacSHA512 calculates HMAC-SHA512 hash in hexadecimal lowercase.
func hmacSHA512(secret, data string) string {
	h := hmac.New(sha512.New, []byte(secret))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// parseVNPayValues parses byte payload as either form URL-encoded or JSON.
func parseVNPayValues(payload []byte) (url.Values, error) {
	trimmed := strings.TrimSpace(string(payload))
	if strings.HasPrefix(trimmed, "{") {
		var rawMap map[string]any
		if err := json.Unmarshal(payload, &rawMap); err != nil {
			return nil, fmt.Errorf("invalid json payload: %w", err)
		}
		values := url.Values{}
		for k, v := range rawMap {
			values.Set(k, fmt.Sprintf("%v", v))
		}
		return values, nil
	}

	values, err := url.ParseQuery(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid url query payload: %w", err)
	}
	return values, nil
}
