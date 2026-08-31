package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
	amountInt := int64(req.Amount)
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
	defer resp.Body.Close()

	var momoResp momoCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&momoResp); err != nil {
		return CreatePaymentResponse{}, err
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
		Amount:        float64(body.Amount),
		Metadata: map[string]string{
			"pay_type":   body.PayType,
			"order_type": body.OrderType,
			"message":    body.Message,
		},
	}, nil
}

func (p *momoProvider) QueryTransactionStatus(ctx context.Context, providerTxnID string) (TransactionStatus, error) {
	return TxnStatusPending, nil
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
	amountInt := int64(req.Amount)
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
	defer resp.Body.Close()

	var momoResp momoRefundResponse
	if err := json.NewDecoder(resp.Body).Decode(&momoResp); err != nil {
		return RefundResponse{}, fmt.Errorf("failed to decode MoMo refund response: %w", err)
	}

	if momoResp.ResultCode != 0 {
		return RefundResponse{
			Status:  TxnStatusFailed,
			Message: momoResp.Message,
		}, fmt.Errorf("momo refund rejected with code %d: %s", momoResp.ResultCode, momoResp.Message)
	}

	return RefundResponse{
		RefundTxnID: strconv.FormatInt(momoResp.TransID, 10),
		Amount:      float64(momoResp.Amount),
		Status:      TxnStatusSuccess,
		Message:     momoResp.Message,
	}, nil
}
