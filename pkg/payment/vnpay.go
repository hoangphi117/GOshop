package payment

// import (
// 	"bytes"
// 	"context"
// 	"crypto/hmac"
// 	"crypto/sha512"
// 	"encoding/hex"
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"net/http"
// 	"net/url"
// 	"sort"
// 	"strconv"
// 	"strings"
// 	"time"
// )

// type vnpayProvider struct {
// 	config VNPayConfig
// 	client *http.Client
// }

// func NewVNPayProvider(config VNPayConfig) PaymentProvider {
// 	return &vnpayProvider{
// 		config: config,
// 		client: &http.Client{Timeout: 10 * time.Second},
// 	}
// }

// func (p *vnpayProvider) CreatePaymentURL(ctx context.Context, req CreatePaymentRequest) (CreatePaymentResponse, error) {
// 	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
// 	if err != nil {
// 		loc = time.FixedZone("GMT+7", 7*3600)
// 	}
// 	now := time.Now().In(loc)
// 	createDate := now.Format("20060102150405")

// 	// VNPay tính số tiền nhân 100 (VND không có số thập phân)
// 	amountInt := int64(req.Amount * 100)

// 	clientIP := req.ClientIP
// 	if clientIP == "" || clientIP == "::1" || strings.Contains(clientIP, ":") {
// 		clientIP = "127.0.0.1"
// 	}

// 	params := url.Values{}
// 	params.Set("vnp_Version", "2.1.0")
// 	params.Set("vnp_Command", "pay")
// 	params.Set("vnp_TmnCode", p.config.TmnCode)
// 	params.Set("vnp_Amount", strconv.FormatInt(amountInt, 10))
// 	params.Set("vnp_CurrCode", "VND")
// 	params.Set("vnp_TxnRef", req.OrderNumber)
// 	params.Set("vnp_OrderInfo", req.Description)
// 	params.Set("vnp_OrderType", "other")
// 	params.Set("vnp_Locale", "vn")
// 	params.Set("vnp_ReturnUrl", req.ReturnURL)
// 	params.Set("vnp_IpAddr", clientIP)
// 	params.Set("vnp_CreateDate", createDate)

// 	signData := buildVNPaySignData(params)
// 	signature := hmacSHA512(p.config.HashSecret, signData)
// 	params.Set("vnp_SecureHash", signature)

// 	paymentURL := fmt.Sprintf(
// 		"%s?%s",
// 		p.config.BaseURL,
// 		params.Encode(),
// 	)

// 	return CreatePaymentResponse{
// 		ProviderTxnID: req.OrderNumber,
// 		PaymentURL:    paymentURL,
// 	}, nil
// }

// func (p *vnpayProvider) VerifyWebhook(payload []byte, headers map[string]string) error {
// 	values, err := url.ParseQuery(string(payload))
// 	if err != nil {
// 		return errors.New("invalid payload query format")
// 	}

// 	receivedHash := values.Get("vnp_SecureHash")
// 	if receivedHash == "" {
// 		return errors.New("missing vnp_SecureHash")
// 	}

// 	values.Del("vnp_SecureHash")
// 	values.Del("vnp_SecureHashType")

// 	signData := buildVNPaySignData(values)
// 	expectedHash := hmacSHA512(p.config.HashSecret, signData)

// 	if !strings.EqualFold(receivedHash, expectedHash) {
// 		return errors.New("invalid signature checksum")
// 	}

// 	return nil
// }

// func (p *vnpayProvider) ParseWebhookPayload(payload []byte) (WebhookResult, error) {
// 	values, err := url.ParseQuery(string(payload))
// 	if err != nil {
// 		return WebhookResult{}, err
// 	}

// 	responseCode := values.Get("vnp_ResponseCode")
// 	txnStatus := values.Get("vnp_TransactionStatus")
// 	amountRaw, _ := strconv.ParseFloat(values.Get("vnp_Amount"), 64)

// 	status := TxnStatusFailed
// 	if responseCode == "00" && txnStatus == "00" {
// 		status = TxnStatusSuccess
// 	}

// 	return WebhookResult{
// 		EventID:       fmt.Sprintf("vnpay_%s_%s", values.Get("vnp_TransactionNo"), values.Get("vnp_TxnRef")),
// 		EventType:     fmt.Sprintf("payment.%s", status),
// 		ProviderTxnID: values.Get("vnp_TransactionNo"),
// 		OrderNumber:   values.Get("vnp_TxnRef"),
// 		Status:        status,
// 		Amount:        amountRaw / 100,
// 		Metadata: map[string]string{
// 			"bank_code": values.Get("vnp_BankCode"),
// 			"card_type": values.Get("vnp_CardType"),
// 			"pay_date":  values.Get("vnp_PayDate"),
// 		},
// 	}, nil
// }

// func (p *vnpayProvider) QueryTransactionStatus(ctx context.Context, providerTxnID string) (TransactionStatus, error) {
// 	return TxnStatusPending, nil
// }

// type vnpayRefundRequest struct {
// 	VnpRequestID       string `json:"vnp_RequestId"`
// 	VnpVersion         string `json:"vnp_Version"`
// 	VnpCommand         string `json:"vnp_Command"`
// 	VnpTmnCode         string `json:"vnp_TmnCode"`
// 	VnpTransactionType string `json:"vnp_TransactionType"`
// 	VnpTxnRef          string `json:"vnp_TxnRef"`
// 	VnpAmount          int64  `json:"vnp_Amount"`
// 	VnpOrderInfo       string `json:"vnp_OrderInfo"`
// 	VnpTransactionNo   string `json:"vnp_TransactionNo"`
// 	VnpTransactionDate string `json:"vnp_TransactionDate"`
// 	VnpCreateBy        string `json:"vnp_CreateBy"`
// 	VnpCreateDate      string `json:"vnp_CreateDate"`
// 	VnpIPAddr          string `json:"vnp_IpAddr"`
// 	VnpSecureHash      string `json:"vnp_SecureHash"`
// }

// type vnpayRefundResponse struct {
// 	VnpResponseCode    string `json:"vnp_ResponseCode"`
// 	VnpMessage         string `json:"vnp_Message"`
// 	VnpTransactionNo   string `json:"vnp_TransactionNo"`
// 	VnpAmount          string `json:"vnp_Amount"`
// 	VnpSecureHash      string `json:"vnp_SecureHash"`
// }

// func (p *vnpayProvider) Refund(ctx context.Context, req RefundRequest) (RefundResponse, error) {
// 	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
// 	if loc == nil {
// 		loc = time.FixedZone("GMT+7", 7*3600)
// 	}
// 	now := time.Now().In(loc)
// 	createDate := now.Format("20060102150405")
// 	requestID := fmt.Sprintf("req-%d", now.UnixNano())

// 	txnType := "02"
// 	if req.Amount >= req.TotalAmount {
// 		txnType = "03"
// 	}

// 	amountInt := int64(req.Amount * 100)
// 	clientIP := req.ClientIP
// 	if clientIP == "" || clientIP == "::1" || strings.Contains(clientIP, ":") {
// 		clientIP = "127.0.0.1"
// 	}

// 	createdBy := req.CreatedBy
// 	if createdBy == "" {
// 		createdBy = "Admin"
// 	}

// 	orderInfo := req.Reason
// 	if orderInfo == "" {
// 		orderInfo = fmt.Sprintf("Hoan tien don hang %s", req.OrderNumber)
// 	}

// 	txnDate := req.TransactionDate
// 	if txnDate == "" {
// 		txnDate = createDate
// 	}

// 	rawSignature := fmt.Sprintf(
// 		"%s|2.1.0|refund|%s|%s|%s|%d|%s|%s|%s|%s|%s|%s",
// 		requestID, p.config.TmnCode, txnType, req.OrderNumber, amountInt,
// 		req.ProviderTxnID, txnDate, createdBy, createDate, clientIP, orderInfo,
// 	)

// 	signature := hmacSHA512(p.config.HashSecret, rawSignature)

// 	refundPayload := vnpayRefundRequest{
// 		VnpRequestID:       requestID,
// 		VnpVersion:         "2.1.0",
// 		VnpCommand:         "refund",
// 		VnpTmnCode:         p.config.TmnCode,
// 		VnpTransactionType: txnType,
// 		VnpTxnRef:          req.OrderNumber,
// 		VnpAmount:          amountInt,
// 		VnpOrderInfo:       orderInfo,
// 		VnpTransactionNo:   req.ProviderTxnID,
// 		VnpTransactionDate: txnDate,
// 		VnpCreateBy:        createdBy,
// 		VnpCreateDate:      createDate,
// 		VnpIPAddr:          clientIP,
// 		VnpSecureHash:      signature,
// 	}

// 	bodyBytes, err := json.Marshal(refundPayload)
// 	if err != nil {
// 		return RefundResponse{}, err
// 	}

// 	queryURL := p.config.QueryURL
// 	if queryURL == "" {
// 		queryURL = "https://sandbox.vnpayment.vn/merchant_webapi/api/transaction"
// 	}

// 	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, queryURL, bytes.NewBuffer(bodyBytes))
// 	if err != nil {
// 		return RefundResponse{}, err
// 	}
// 	httpReq.Header.Set("Content-Type", "application/json")

// 	resp, err := p.client.Do(httpReq)
// 	if err != nil {
// 		return RefundResponse{}, fmt.Errorf("failed to call VNPay refund API: %w", err)
// 	}
// 	defer resp.Body.Close()

// 	var vnpResp vnpayRefundResponse
// 	if err := json.NewDecoder(resp.Body).Decode(&vnpResp); err != nil {
// 		return RefundResponse{}, fmt.Errorf("failed to decode VNPay refund response: %w", err)
// 	}

// 	if vnpResp.VnpResponseCode != "00" {
// 		return RefundResponse{
// 			Status:  TxnStatusFailed,
// 			Message: vnpResp.VnpMessage,
// 		}, fmt.Errorf("vnpay refund rejected with code %s: %s", vnpResp.VnpResponseCode, vnpResp.VnpMessage)
// 	}

// 	return RefundResponse{
// 		RefundTxnID: vnpResp.VnpTransactionNo,
// 		Amount:      req.Amount,
// 		Status:      TxnStatusSuccess,
// 		Message:     "Refund successful",
// 	}, nil
// }

// func buildVNPaySignData(params url.Values) string {
// 	keys := make([]string, 0, len(params))

// 	for k := range params {
// 		if strings.HasPrefix(k, "vnp_") && params.Get(k) != "" {
// 			keys = append(keys, k)
// 		}
// 	}

// 	sort.Strings(keys)

// 	var b strings.Builder

// 	for i, k := range keys {
// 		if i > 0 {
// 			b.WriteString("&")
// 		}

// 		b.WriteString(k)
// 		b.WriteString("=")
// 		b.WriteString(params.Get(k))
// 	}

// 	return b.String()
// }

// func hmacSHA512(key, data string) string {
// 	h := hmac.New(sha512.New, []byte(key))
// 	h.Write([]byte(data))
// 	return hex.EncodeToString(h.Sum(nil))
// }
