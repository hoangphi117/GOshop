package shipping

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	v1dto "goshop/internal/dto/v1"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultGHNSandboxURL = "https://dev-online-gateway.ghn.vn"
	ghnProviderName      = "ghn"
	defaultServiceTypeID = 2 // Chuẩn/Thương mại điện tử
	paymentTypeShop      = 1 // Shop trả cước vận chuyển
	paymentTypeReceiver  = 2 // Người nhận trả cước vận chuyển
)

// GHNAdapter implements ShippingProvider cho đơn vị Giao Hàng Nhanh (GHN)
type GHNAdapter struct {
	token      string
	shopID     int
	baseURL    string
	httpClient *http.Client
}

// NewGHNAdapter tạo instance mới của GHNAdapter
func NewGHNAdapter(token string, shopID int, baseURL string) *GHNAdapter {
	if baseURL == "" {
		baseURL = defaultGHNSandboxURL
	}
	return &GHNAdapter{
		token:   token,
		shopID:  shopID,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Name trả về định danh của provider
func (a *GHNAdapter) Name() string {
	return ghnProviderName
}

// ── GHN API Models ─────────────────────────────────────────────────────────────

type ghnBaseResponse struct {
	Code        int             `json:"code"`
	Message     string          `json:"message"`
	MessageCode string          `json:"message_code"`
	Data        json.RawMessage `json:"data"`
}

type ghnCreateOrderPayload struct {
	PaymentTypeID int       `json:"payment_type_id"` // 1: Shop/Người gửi trả cước, 2: Người nhận trả cước
	Note          string    `json:"note,omitempty"`
	RequiredNote  string    `json:"required_note"` // CHOXEMHANGKHONGTHU, CHOTHUHANG, KHONGCHOXEMHANG
	ToName        string    `json:"to_name"`
	ToPhone       string    `json:"to_phone"`
	ToAddress     string    `json:"to_address"`
	ToWardCode    string    `json:"to_ward_code"`
	ToDistrictID  int       `json:"to_district_id"`
	CODAmount     int       `json:"cod_amount"`
	Weight        int       `json:"weight"`
	Length        int       `json:"length,omitempty"`
	Width         int       `json:"width,omitempty"`
	Height        int       `json:"height,omitempty"`
	ServiceTypeID int       `json:"service_type_id"`
	Items         []ghnItem `json:"items"`
	CallbackURL   string    `json:"callback_url,omitempty"`
}

type ghnItem struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
	Price    int    `json:"price,omitempty"`
	Weight   int    `json:"weight,omitempty"`
}

// ── Payload ─────────────────────────────────────────────────────────────

type ghnCancelPayload struct {
	OrderCodes []string `json:"order_codes"`
}

type ghnDetailPayload struct {
	OrderCode string `json:"order_code"`
}

type ghnGenTokenPayload struct {
	OrderCodes []string `json:"order_codes"`
}

type ghnFeePayload struct {
	ServiceTypeID  int    `json:"service_type_id"`
	ToDistrictID   int    `json:"to_district_id"`
	ToWardCode     string `json:"to_ward_code"`
	Weight         int    `json:"weight"`
	Length         int    `json:"length,omitempty"`
	Width          int    `json:"width,omitempty"`
	Height         int    `json:"height,omitempty"`
	InsuranceValue int    `json:"insurance_value,omitempty"`
}

type ghnProvinceRaw struct {
	ProvinceID   int    `json:"ProvinceID"`
	ProvinceName string `json:"ProvinceName"`
	Code         string `json:"Code"`
}
type ghnDistrictRaw struct {
	DistrictID   int    `json:"DistrictID"`
	ProvinceID   int    `json:"ProvinceID"`
	DistrictName string `json:"DistrictName"`
	Code         string `json:"Code"`
}
type ghnWardRaw struct {
	WardCode   string `json:"WardCode"`
	DistrictID int    `json:"DistrictID"`
	WardName   string `json:"WardName"`
}

// ── Response ─────────────────────────────────────────────────────────────=

type ghnDetailResponseData struct {
	OrderCode string `json:"order_code"`
	Status    string `json:"status"`
	Log       []struct {
		Status      string    `json:"status"`
		UpdatedDate time.Time `json:"updated_date"`
	} `json:"log"`
}

type ghnGenTokenResponseData struct {
	Token string `json:"token"`
}

type ghnCreateOrderResponseData struct {
	OrderCode            string    `json:"order_code"`
	TotalFee             float64   `json:"total_fee"`
	ExpectedDeliveryTime time.Time `json:"expected_delivery_time"`
}

type ghnFeeResponseData struct {
	Total      float64 `json:"total"`
	ServiceFee float64 `json:"service_fee"`
}

// ── Implementation of ShippingProvider ─────────────────────────────────────────

func (a *GHNAdapter) CreateShippingOrder(ctx context.Context, req CreateOrderRequest) (*CreateOrderResponse, error) {
	requiredNote := req.RequiredNote
	if requiredNote == "" {
		requiredNote = "CHOXEMHANGKHONGTHU"
	}

	weight := req.Weight
	if weight <= 0 {
		weight = 200 // Mặc định 200g
	}

	items := make([]ghnItem, 0, len(req.Items))
	for _, item := range req.Items {
		itemWeight := item.Weight
		if itemWeight <= 0 {
			itemWeight = weight / len(req.Items)
			if itemWeight <= 0 {
				itemWeight = 50
			}
		}
		items = append(items, ghnItem{
			Name:     item.Name,
			Quantity: item.Quantity,
			Price:    int(item.Price),
			Weight:   itemWeight,
		})
	}

	payload := ghnCreateOrderPayload{
		PaymentTypeID: paymentTypeShop, // Shop trả cước vận chuyển
		Note:          req.Note,
		RequiredNote:  requiredNote,
		ToName:        req.ToName,
		ToPhone:       req.ToPhone,
		ToAddress:     req.ToAddress,
		ToWardCode:    req.ToWardCode,
		ToDistrictID:  req.ToDistrictID,
		CODAmount:     int(req.CODAmount),
		Weight:        weight,
		Length:        req.Length,
		Width:         req.Width,
		Height:        req.Height,
		ServiceTypeID: defaultServiceTypeID,
		Items:         items,
		CallbackURL:   req.CallbackURL,
	}

	endpoint := fmt.Sprintf("%s/shiip/public-api/v2/shipping-order/create", a.baseURL)
	body, err := a.doRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("ghn create order request failed: %w", err)
	}

	var resData ghnCreateOrderResponseData
	if err := json.Unmarshal(body, &resData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ghn response data: %w", err)
	}

	var expectedDelivery *time.Time
	if !resData.ExpectedDeliveryTime.IsZero() {
		expectedDelivery = &resData.ExpectedDeliveryTime
	}

	return &CreateOrderResponse{
		ShippingCode:     resData.OrderCode,
		ShippingFee:      resData.TotalFee,
		ExpectedDelivery: expectedDelivery,
	}, nil
}

func (a *GHNAdapter) CalculateFee(ctx context.Context, req CreateOrderRequest) (float64, error) {
	weight := req.Weight
	if weight <= 0 {
		weight = 200
	}

	payload := ghnFeePayload{
		ServiceTypeID:  defaultServiceTypeID,
		ToDistrictID:   req.ToDistrictID,
		ToWardCode:     req.ToWardCode,
		Weight:         weight,
		Length:         req.Length,
		Width:          req.Width,
		Height:         req.Height,
		InsuranceValue: int(req.CODAmount),
	}

	endpoint := fmt.Sprintf("%s/shiip/public-api/v2/shipping-order/fee", a.baseURL)
	body, err := a.doRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return 0, fmt.Errorf("ghn calculate fee failed: %w", err)
	}

	var resData ghnFeeResponseData
	if err := json.Unmarshal(body, &resData); err != nil {
		return 0, fmt.Errorf("failed to parse ghn fee response: %w", err)
	}

	return resData.Total, nil
}

func (a *GHNAdapter) CancelShippingOrder(ctx context.Context, shippingCode string) error {
	payload := ghnCancelPayload{
		OrderCodes: []string{shippingCode},
	}

	endpoint := fmt.Sprintf("%s/shiip/public-api/v2/switch-status/cancel", a.baseURL)
	_, err := a.doRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return fmt.Errorf("ghn cancel order failed: %w", err)
	}

	return nil
}

func (a *GHNAdapter) GetTrackingInfo(ctx context.Context, shippingCode string) (*TrackingInfo, error) {
	payload := ghnDetailPayload{
		OrderCode: shippingCode,
	}

	endpoint := fmt.Sprintf("%s/shiip/public-api/v2/shipping-order/detail", a.baseURL)
	body, err := a.doRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("ghn get detail failed: %w", err)
	}

	var resData ghnDetailResponseData
	if err := json.Unmarshal(body, &resData); err != nil {
		return nil, fmt.Errorf("failed to parse ghn detail response: %w", err)
	}

	var updatedAt *time.Time
	if len(resData.Log) > 0 {
		lastLog := resData.Log[len(resData.Log)-1]
		updatedAt = &lastLog.UpdatedDate
	} else {
		now := time.Now()
		updatedAt = &now
	}

	return &TrackingInfo{
		Status:     resData.Status,
		StatusText: mapGHNStatusText(resData.Status),
		UpdatedAt:  updatedAt,
	}, nil
}

func (a *GHNAdapter) GetPrintBillURL(ctx context.Context, shippingCode string) (*PrintBillResponse, error) {
	payload := ghnGenTokenPayload{
		OrderCodes: []string{shippingCode},
	}

	endpoint := fmt.Sprintf("%s/shiip/public-api/v2/a5/gen-token", a.baseURL)
	body, err := a.doRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("ghn gen print token failed: %w", err)
	}

	var resData ghnGenTokenResponseData
	if err := json.Unmarshal(body, &resData); err != nil {
		return nil, fmt.Errorf("failed to parse ghn gen token response: %w", err)
	}
	return &PrintBillResponse{
		Token:   resData.Token,
		PrintA5: fmt.Sprintf("%s/a5/public-api/printA5?token=%s", a.baseURL, resData.Token),
		Print80: fmt.Sprintf("%s/a5/public-api/print80x80?token=%s", a.baseURL, resData.Token),
		Print52: fmt.Sprintf("%s/a5/public-api/print52x70?token=%s", a.baseURL, resData.Token),
	}, nil
}

func (a *GHNAdapter) GetProvinces(ctx context.Context) ([]v1dto.ProvinceDTO, error) {
	endpoint := fmt.Sprintf("%s/shiip/public-api/master-data/province", a.baseURL)
	body, err := a.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("ghn get provinces failed: %w", err)
	}

	var raws []ghnProvinceRaw
	if err := json.Unmarshal(body, &raws); err != nil {
		return nil, fmt.Errorf("failed to parse ghn provinces response: %w", err)
	}

	res := make([]v1dto.ProvinceDTO, len(raws))
	for i, raw := range raws {
		res[i] = v1dto.ProvinceDTO{
			ProvinceID:   raw.ProvinceID,
			ProvinceName: raw.ProvinceName,
			Code:         raw.Code,
		}
	}

	return res, nil
}

func (a *GHNAdapter) GetDistricts(ctx context.Context, provinceID int) ([]v1dto.DistrictDTO, error) {
	endpoint := fmt.Sprintf("%s/shiip/public-api/master-data/district", a.baseURL)
	var body json.RawMessage
	var err error
	if provinceID > 0 {
		body, err = a.doRequest(ctx, http.MethodPost, endpoint, map[string]int{"province_id": provinceID})
	} else {
		body, err = a.doRequest(ctx, http.MethodGet, endpoint, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("ghn get districts failed: %w", err)
	}

	var raws []ghnDistrictRaw
	if err := json.Unmarshal(body, &raws); err != nil {
		return nil, fmt.Errorf("failed to parse ghn districts response: %w", err)
	}

	res := make([]v1dto.DistrictDTO, len(raws))
	for i, raw := range raws {
		res[i] = v1dto.DistrictDTO{
			DistrictID:   raw.DistrictID,
			ProvinceID:   raw.ProvinceID,
			DistrictName: raw.DistrictName,
			Code:         raw.Code,
		}
	}

	return res, nil
}

func (a *GHNAdapter) GetWards(ctx context.Context, districtID int) ([]v1dto.WardDTO, error) {
	endpoint := fmt.Sprintf("%s/shiip/public-api/master-data/ward?district_id=%d", a.baseURL, districtID)
	body, err := a.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("ghn get wards failed: %w", err)
	}

	var raws []ghnWardRaw
	if err := json.Unmarshal(body, &raws); err != nil {
		return nil, fmt.Errorf("failed to parse ghn wards response: %w", err)
	}

	res := make([]v1dto.WardDTO, len(raws))
	for i, raw := range raws {
		res[i] = v1dto.WardDTO{
			DistrictID: raw.DistrictID,
			WardName:   raw.WardName,
			WardCode:   raw.WardCode,
		}
	}

	return res, nil
}

// ── HTTP Helper ────────────────────────────────────────────────────────────────

func (a *GHNAdapter) doRequest(ctx context.Context, method, endpoint string, payload any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request payload: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Token", a.token)
	if a.shopID > 0 {
		httpReq.Header.Set("ShopId", strconv.Itoa(a.shopID))
	}

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request error: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var baseRes ghnBaseResponse
	if err := json.Unmarshal(respBytes, &baseRes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ghn base response (HTTP %d): %w", resp.StatusCode, err)
	}

	if baseRes.Code != 200 {
		return nil, fmt.Errorf("ghn api error (code %d): %s (message_code: %s)", baseRes.Code, baseRes.Message, baseRes.MessageCode)
	}

	return baseRes.Data, nil
}

func mapGHNStatusText(status string) string {
	switch status {
	case "ready_to_pick":
		return "Ready to pick"
	case "picking":
		return "Picking up"
	case "storing":
		return "Stored at warehouse"
	case "delivering":
		return "Delivering"
	case "delivered":
		return "Delivered"
	case "delivery_fail":
		return "Delivery failed"
	case "waiting_to_return":
		return "Waiting for return confirmation"
	case "return":
		return "Returning"
	case "returned":
		return "Returned"
	case "cancel":
		return "Cancelled"
	default:
		return status
	}
}
