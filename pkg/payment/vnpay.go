package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type vnpayProvider struct {
	config VNPayConfig
}

func NewVNPayProvider(config VNPayConfig) PaymentProvider {
	return &vnpayProvider{config: config}
}

func (p *vnpayProvider) CreatePaymentURL(ctx context.Context, req CreatePaymentRequest) (CreatePaymentResponse, error) {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.FixedZone("GMT+7", 7*3600)
	}
	now := time.Now().In(loc)
	createDate := now.Format("20060102150405")

	// VNPay tính số tiền nhân 100 (VND không có số thập phân)
	amountInt := int64(req.Amount * 100)

	clientIP := req.ClientIP
	if clientIP == "" || clientIP == "::1" || strings.Contains(clientIP, ":") {
		clientIP = "127.0.0.1"
	}

	params := url.Values{}
	params.Set("vnp_Version", "2.1.0")
	params.Set("vnp_Command", "pay")
	params.Set("vnp_TmnCode", p.config.TmnCode)
	params.Set("vnp_Amount", strconv.FormatInt(amountInt, 10))
	params.Set("vnp_CurrCode", "VND")
	params.Set("vnp_TxnRef", req.OrderNumber)
	params.Set("vnp_OrderInfo", req.Description)
	params.Set("vnp_OrderType", "other")
	params.Set("vnp_Locale", "vn")
	params.Set("vnp_ReturnUrl", req.ReturnURL)
	params.Set("vnp_IpAddr", clientIP)
	params.Set("vnp_CreateDate", createDate)

	// Sắp xếp params theo alphabet để tính hash
	signData := buildVNPaySignData(params)
	fmt.Println("SignData:", signData)

	signature := hmacSHA512(
		p.config.HashSecret,
		signData,
	)

	fmt.Println("Signature:", signature)

	params.Set("vnp_SecureHash", signature)

	paymentURL := fmt.Sprintf(
		"%s?%s",
		p.config.BaseURL,
		params.Encode(),
	)

	return CreatePaymentResponse{
		ProviderTxnID: req.OrderNumber,
		PaymentURL:    paymentURL,
	}, nil
}

func (p *vnpayProvider) VerifyWebhook(payload []byte, headers map[string]string) error {
	// VNPay IPN thường gửi params dưới dạng query string hoặc form urlencoded
	values, err := url.ParseQuery(string(payload))
	if err != nil {
		return errors.New("invalid payload query format")
	}

	receivedHash := values.Get("vnp_SecureHash")
	if receivedHash == "" {
		return errors.New("missing vnp_SecureHash")
	}

	// Xóa hash để tính lại chữ ký
	values.Del("vnp_SecureHash")
	values.Del("vnp_SecureHashType")

	signData := buildVNPaySignData(values)
	expectedHash := hmacSHA512(p.config.HashSecret, signData)

	if !strings.EqualFold(receivedHash, expectedHash) {
		return errors.New("invalid signature checksum")
	}

	return nil
}

func (p *vnpayProvider) ParseWebhookPayload(payload []byte) (WebhookResult, error) {
	values, err := url.ParseQuery(string(payload))
	if err != nil {
		return WebhookResult{}, err
	}

	responseCode := values.Get("vnp_ResponseCode")
	txnStatus := values.Get("vnp_TransactionStatus")
	amountRaw, _ := strconv.ParseFloat(values.Get("vnp_Amount"), 64)

	status := TxnStatusFailed
	if responseCode == "00" && txnStatus == "00" {
		status = TxnStatusSuccess
	}

	return WebhookResult{
		EventID:       fmt.Sprintf("vnpay_%s_%s", values.Get("vnp_TransactionNo"), values.Get("vnp_TxnRef")),
		EventType:     fmt.Sprintf("payment.%s", status),
		ProviderTxnID: values.Get("vnp_TransactionNo"),
		OrderNumber:   values.Get("vnp_TxnRef"),
		Status:        status,
		Amount:        amountRaw / 100, // Đổi lại số tiền thực tế
		Metadata: map[string]string{
			"bank_code": values.Get("vnp_BankCode"),
			"card_type": values.Get("vnp_CardType"),
			"pay_date":  values.Get("vnp_PayDate"),
		},
	}, nil
}

func (p *vnpayProvider) QueryTransactionStatus(ctx context.Context, providerTxnID string) (TransactionStatus, error) {
	// TODO: Gọi HTTP request đến VNPay QueryDR API
	// Tạm thời trả về pending cho demo
	return TxnStatusPending, nil
}

// Helper hàm sắp xếp và nối chuỗi VNPay
func buildVNPaySignData(params url.Values) string {
	keys := make([]string, 0, len(params))

	for k := range params {
		if strings.HasPrefix(k, "vnp_") && params.Get(k) != "" {
			keys = append(keys, k)
		}
	}

	sort.Strings(keys)

	var b strings.Builder

	for i, k := range keys {
		if i > 0 {
			b.WriteString("&")
		}

		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(params.Get(k))
	}

	return b.String()
}

func hmacSHA512(key, data string) string {
	h := hmac.New(sha512.New, []byte(key))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}
