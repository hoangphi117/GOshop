package v1dto

import (
	"goshop/internal/db/sqlc"
	"strings"
	"time"
)

// ── Params / Queries ───────────────────────────────────────────────

type CreateShippingParams struct {
	UUID string `uri:"uuid" binding:"required,uuid"`
}

type GetOrderTimeLineParams struct {
	UUID string `uri:"uuid" binding:"required,uuid"`
}

type CancelShippingParams struct {
	UUID string `uri:"uuid" binding:"required,uuid"`
}

type GetPrintBillParams struct {
	UUID string `uri:"uuid" binding:"required,uuid"`
}

type ReturnUUIDParam struct {
	UUID string `uri:"uuid" binding:"required,uuid"`
}

type CreateOrderReturnParam struct {
	UUID string `uri:"uuid" binding:"required,uuid"`
}

type GetDistrictsQuery struct {
	ProvinceID int `form:"province_id" binding:"required,gt=0"`
}

type GetWardsQuery struct {
	DistrictID int `form:"district_id" binding:"required,gt=0"`
}

type AdminGetReturnsQuery struct {
	Status *int16 `form:"status" binding:"omitempty,oneof=1 2 3 4 5"` // 1: Pending, 2: Approved, 3: Rejected, 4: Completed, 5: Cancelled
	Search string `form:"search" binding:"omitempty,max=100"`
	Page   int32  `form:"page"   binding:"omitempty,gte=1"`
	Limit  int32  `form:"limit"  binding:"omitempty,gte=1,lte=100"`
}

type GetMyOrderReturnsQuery struct {
	Status *int16 `form:"status" binding:"omitempty,oneof=1 2 3 4 5"` // 1: Pending, 2: Approved, 3: Rejected, 4: Completed, 5: Cancelled
	Search string `form:"search" binding:"omitempty,max=100"`
	Page   int32  `form:"page"   binding:"omitempty,gte=1"`
	Limit  int32  `form:"limit"  binding:"omitempty,gte=1,lte=100"`
}

// ── Input ───────────────────────────────────────────────

type CreateShippingInput struct {
	Provider     string `json:"provider" binding:"required,oneof=ghn manual"`
	DistrictID   int    `json:"district_id" binding:"required"`
	WardCode     string `json:"ward_code" binding:"required"`
	Weight       int    `json:"weight" binding:"required,gte=1"` // gram
	Length       int    `json:"length" binding:"required,gte=1"` // cm
	Width        int    `json:"width" binding:"required,gte=1"`  // cm
	Height       int    `json:"height" binding:"required,gte=1"` // cm
	RequiredNote string `json:"required_note" binding:"omitempty,oneof=KHONGCHOXEMHANG CHOTHUHANG CHOXEMHANGKHONGTHU"`
	Note         string `json:"note" binding:"omitempty,max=500"`
}

type CalculateShippingFeeInput struct {
	Provider       string  `json:"provider" binding:"required,oneof=ghn manual"`
	DistrictID     int     `json:"district_id" binding:"required"`
	WardCode       string  `json:"ward_code" binding:"required"`
	Weight         int     `json:"weight" binding:"required,gte=1"`           // gram (tổng khối lượng)
	Length         int     `json:"length" binding:"omitempty,gte=0"`          // cm
	Width          int     `json:"width" binding:"omitempty,gte=0"`           // cm
	Height         int     `json:"height" binding:"omitempty,gte=0"`          // cm
	InsuranceValue float64 `json:"insurance_value" binding:"omitempty,gte=0"` // Giá trị khai giá bảo hiểm
}

type CancelShippingInput struct {
	Reason string `json:"reason" binding:"omitempty,max=500"` // Lý do hủy đơn vận chuyển
}

type CreateOrderReturnInput struct {
	Reason string `form:"reason" json:"reason" binding:"required,min=10,max=1000"`
}

type CancelOrderReturnInput struct {
	Reason string `json:"reason" binding:"omitempty,max=500"`
}

type AdminUpdateReturnInput struct {
	Status    int16   `json:"status"     binding:"required,oneof=2 3 4"` // 2: Approved, 3: Rejected, 4: Completed
	AdminNote *string `json:"admin_note" binding:"omitempty,max=500"`
}

type SubmitReturnShippingInput struct {
	ShippingProvider string `json:"shipping_provider" binding:"required,min=2,max=50"`
	ShippingCode     string `json:"shipping_code" binding:"required,min=3,max=100"`
	Note             string `json:"note" binding:"omitempty,max=500"`
}

// ── DTO ───────────────────────────────────────────────

// Thông tin từng mốc sự kiện trong timeline
type TrackingEventDTO struct {
	EventID    int32     `json:"event_id"`
	Status     string    `json:"status"`      // "pending", "confirmed", "shipped", "delivered",...
	StatusText string    `json:"status_text"` // Bản dịch tiếng Việt hiển thị UI: "Đang giao hàng",...
	Note       *string   `json:"note,omitempty"`
	Source     string    `json:"source"` // "system", "admin", "ghn", "user"
	CreatedAt  time.Time `json:"created_at"`
}

type ProvinceDTO struct {
	ProvinceID   int    `json:"province_id"`
	ProvinceName string `json:"province_name"`
	Code         string `json:"code,omitempty"`
}
type DistrictDTO struct {
	DistrictID   int    `json:"district_id"`
	ProvinceID   int    `json:"province_id"`
	DistrictName string `json:"district_name"`
	Code         string `json:"code,omitempty"`
}
type WardDTO struct {
	WardCode   string `json:"ward_code"`
	DistrictID int    `json:"district_id"`
	WardName   string `json:"ward_name"`
}

type OrderReturnDTO struct {
	ReturnUUID  string    `json:"return_uuid"`
	OrderUUID   string    `json:"order_uuid"`
	OrderNumber string    `json:"order_number"`
	Reason      string    `json:"reason"`
	Images      []string  `json:"images"`
	VideoURL    *string   `json:"video_url,omitempty"`
	Status      string    `json:"status"` // "Pending", "Approved", "Rejected", "Completed"
	CreatedAt   time.Time `json:"created_at"`
}

// ── Response ───────────────────────────────────────────────

type ShippingResponseDTO struct {
	OrderUUID        string     `json:"order_uuid"`
	OrderNumber      string     `json:"order_number"`
	ShippingProvider string     `json:"shipping_provider"`
	ShippingCode     string     `json:"shipping_code"`
	ShippingFee      float64    `json:"shipping_fee"`
	ExpectedDelivery *time.Time `json:"expected_delivery_date"`
	OrderStatus      string     `json:"order_status"` // "Shipped"
	CreatedAt        time.Time  `json:"created_at"`
}

type CreateShippingQueueMessage struct {
	OrderUUID  string              `json:"order_uuid"`
	Input      CreateShippingInput `json:"input"`
	EnqueuedAt time.Time           `json:"enqueued_at"`
}

type GHNWebhookPayload struct {
	OrderCode   string    `json:"OrderCode" binding:"required"` // Mã vận đơn
	Status      string    `json:"Status" binding:"required"`    // delivered, delivering, return,...
	Time        time.Time `json:"Time"`
	CODAmount   float64   `json:"CODAmount"`
	TotalFee    float64   `json:"TotalFee"`
	Description string    `json:"Description"` // Lý do nếu giao thất bại hoặc trả hàng
	Warehouse   string    `json:"Warehouse"`
}

// Response trả về ngay lập tức cho Admin (HTTP 202)
type AsyncShippingResponseDTO struct {
	OrderUUID string `json:"order_uuid"`
	Status    string `json:"status"` // "processing"
	Message   string `json:"message"`
}

type OrderTimelineResponseDTO struct {
	OrderUUID   string             `json:"order_uuid"`
	OrderNumber string             `json:"order_number"`
	Events      []TrackingEventDTO `json:"events"`
}

type ShippingFeeResponseDTO struct {
	Provider    string  `json:"provider"`
	ShippingFee float64 `json:"shipping_fee"`
}

type CancelShippingResponseDTO struct {
	OrderUUID    string `json:"order_uuid"`
	OrderNumber  string `json:"order_number"`
	ShippingCode string `json:"shipping_code"`
	OrderStatus  string `json:"order_status"` // "Confirmed"
	Message      string `json:"message"`
}

type PrintBillResponseDTO struct {
	OrderUUID    string `json:"order_uuid"`
	OrderNumber  string `json:"order_number"`
	ShippingCode string `json:"shipping_code"`
	Token        string `json:"token"`
	PrintA5      string `json:"print_a5"`
	Print80x80   string `json:"print_80x80"`
	Print52x70   string `json:"print_52x70"`
}

type AdminReturnSummaryDTO struct {
	ReturnUUID    string    `json:"return_uuid"`
	OrderUUID     string    `json:"order_uuid"`
	OrderNumber   string    `json:"order_number"`
	CustomerName  string    `json:"customer_name"`  // o.OrderShippingName
	CustomerPhone string    `json:"customer_phone"` // o.OrderShippingPhone
	CustomerEmail string    `json:"customer_email"` // u.UserEmail
	TotalAmount   float64   `json:"total_amount"`
	PaymentMethod string    `json:"payment_method"`
	PaymentStatus string    `json:"payment_status"`
	Reason        string    `json:"reason"`
	Images        []string  `json:"images"`
	VideoURL      *string   `json:"video_url,omitempty"`
	Status        int16     `json:"status"`
	StatusText    string    `json:"status_text"`
	AdminNote     *string   `json:"admin_note,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type UserReturnSummaryDTO struct {
	ReturnUUID       string    `json:"return_uuid"`
	OrderUUID        string    `json:"order_uuid"`
	OrderNumber      string    `json:"order_number"`
	TotalAmount      float64   `json:"total_amount"`
	PaymentMethod    string    `json:"payment_method"`
	PaymentStatus    string    `json:"payment_status"`
	Reason           string    `json:"reason"`
	Images           []string  `json:"images"`
	VideoURL         *string   `json:"video_url,omitempty"`
	Status           int16     `json:"status"`
	StatusName       string    `json:"status_name"` // "Pending", "Approved", "Rejected", "Completed", "Cancelled"
	StatusText       string    `json:"status_text"` // "Chờ duyệt", "Đã chấp thuận (Chờ gửi hàng về)", ...
	AdminNote        *string   `json:"admin_note,omitempty"`
	ShippingProvider *string   `json:"shipping_provider,omitempty"`
	ShippingCode     *string   `json:"shipping_code,omitempty"`
	ShippingNote     *string   `json:"shipping_note,omitempty"`
	BankName         *string   `json:"bank_name,omitempty"`
	BankAccount      *string   `json:"bank_account,omitempty"`
	BankAccountName  *string   `json:"bank_account_name,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type OrderReturnDetailDTO struct {
	ReturnUUID       string             `json:"return_uuid"`
	OrderUUID        string             `json:"order_uuid"`
	OrderNumber      string             `json:"order_number"`
	Reason           string             `json:"reason"`
	Images           []string           `json:"images"`
	VideoURL         *string            `json:"video_url,omitempty"`
	Status           int16              `json:"status"`
	StatusName       string             `json:"status_name"` // "Pending", "Approved", "Rejected", "Completed", "Cancelled"
	StatusText       string             `json:"status_text"` // "Chờ duyệt", "Đã chấp thuận (Chờ gửi hàng về)", ...
	AdminNote        *string            `json:"admin_note,omitempty"`
	ShippingProvider *string            `json:"shipping_provider,omitempty"`
	ShippingCode     *string            `json:"shipping_code,omitempty"`
	ShippingNote     *string            `json:"shipping_note,omitempty"`
	BankName         *string            `json:"bank_name,omitempty"`
	BankAccount      *string            `json:"bank_account,omitempty"`
	BankAccountName  *string            `json:"bank_account_name,omitempty"`
	TotalAmount      float64            `json:"total_amount"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
	Events           []TrackingEventDTO `json:"events"`
}

type ReturnCustomerDTO struct {
	UserUUID string `json:"user_uuid"`
	FullName string `json:"fullname"`
	Email    string `json:"email"`
}

type ReturnOrderInfoDTO struct {
	OrderUUID       string    `json:"order_uuid"`
	OrderNumber     string    `json:"order_number"`
	TotalAmount     float64   `json:"total_amount"`
	RefundedAmount  float64   `json:"refunded_amount"`
	PaymentMethod   string    `json:"payment_method"`
	PaymentStatus   string    `json:"payment_status"`
	OrderStatus     string    `json:"order_status"`
	ShippingName    string    `json:"shipping_name"`
	ShippingPhone   string    `json:"shipping_phone"`
	ShippingAddress string    `json:"shipping_address"`
	CreatedAt       time.Time `json:"created_at"`
}

type AdminReturnDetailDTO struct {
	ReturnUUID       string             `json:"return_uuid"`
	Reason           string             `json:"reason"`
	Images           []string           `json:"images"`
	VideoURL         *string            `json:"video_url,omitempty"`
	Status           int16              `json:"status"`
	StatusName       string             `json:"status_name"` // "Pending", "Approved", "Rejected", "Completed", "Cancelled"
	StatusText       string             `json:"status_text"` // "Chờ duyệt", "Đã chấp thuận (Chờ gửi hàng về)", ...
	AdminNote        *string            `json:"admin_note,omitempty"`
	ShippingProvider *string            `json:"shipping_provider,omitempty"`
	ShippingCode     *string            `json:"shipping_code,omitempty"`
	ShippingNote     *string            `json:"shipping_note,omitempty"`
	BankName         *string            `json:"bank_name,omitempty"`
	BankAccount      *string            `json:"bank_account,omitempty"`
	BankAccountName  *string            `json:"bank_account_name,omitempty"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
	Customer         ReturnCustomerDTO  `json:"customer"`
	Order            ReturnOrderInfoDTO `json:"order"`
	Items            []OrderItemDTO     `json:"items"`
	Events           []TrackingEventDTO `json:"events"`
}

type UpdateReturnBankAccountInput struct {
	BankName        string `json:"bank_name" binding:"required,min=2,max=100"`
	BankAccount     string `json:"bank_account" binding:"required,min=5,max=50"`
	BankAccountName string `json:"bank_account_name" binding:"required,min=2,max=100"`
}

type UpdateReturnBankAccountResponseDTO struct {
	ReturnUUID      string    `json:"return_uuid"`
	OrderUUID       string    `json:"order_uuid"`
	OrderNumber     string    `json:"order_number"`
	BankName        string    `json:"bank_name"`
	BankAccount     string    `json:"bank_account"`
	BankAccountName string    `json:"bank_account_name"`
	Status          int16     `json:"status"`
	StatusName      string    `json:"status_name"`
	StatusText      string    `json:"status_text"`
	UpdatedAt       time.Time `json:"updated_at"`
	Message         string    `json:"message"`
}

type CancelOrderReturnResponseDTO struct {
	ReturnUUID   string    `json:"return_uuid"`
	OrderUUID    string    `json:"order_uuid"`
	OrderNumber  string    `json:"order_number"`
	Status       int16     `json:"status"`      // 5
	StatusName   string    `json:"status_name"` // "Cancelled"
	StatusText   string    `json:"status_text"` // "Đã hủy bởi người mua"
	CancelReason string    `json:"cancel_reason,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
	Message      string    `json:"message"`
}

type AdminReturnRefundInput struct {
	Amount          *float64 `json:"amount" binding:"omitempty,gt=0"`
	Reason          string   `json:"reason" binding:"omitempty,max=500"`
	BankName        string   `json:"bank_name" binding:"omitempty,max=100"`
	BankAccount     string   `json:"bank_account" binding:"omitempty,max=50"`
	BankAccountName string   `json:"bank_account_name" binding:"omitempty,max=100"`
	BankTxnRef      string   `json:"bank_txn_ref" binding:"omitempty,max=100"`
	AccountingNote  string   `json:"accounting_note" binding:"omitempty,max=500"`
}

type AdminReturnRefundResponseDTO struct {
	ReturnUUID       string    `json:"return_uuid"`
	OrderUUID        string    `json:"order_uuid"`
	OrderNumber      string    `json:"order_number"`
	RefundAmount     float64   `json:"refund_amount"`
	TotalRefunded    float64   `json:"total_refunded"`
	OrderTotalAmount float64   `json:"order_total_amount"`
	PaymentMethod    string    `json:"payment_method"`
	PaymentStatus    string    `json:"payment_status"`
	RefundTxnID      string    `json:"refund_txn_id"`
	RefundProvider   string    `json:"refund_provider"`
	RefundedAt       time.Time `json:"refunded_at"`
	Message          string    `json:"message"`
}

type SubmitReturnShippingResponseDTO struct {
	ReturnUUID       string    `json:"return_uuid"`
	OrderUUID        string    `json:"order_uuid"`
	OrderNumber      string    `json:"order_number"`
	ShippingProvider string    `json:"shipping_provider"`
	ShippingCode     string    `json:"shipping_code"`
	Note             *string   `json:"note,omitempty"`
	Status           int16     `json:"status"`
	StatusName       string    `json:"status_name"`
	StatusText       string    `json:"status_text"`
	UpdatedAt        time.Time `json:"updated_at"`
	Message          string    `json:"message"`
}

// ── Helper Functions ────────────────────────────────────────────────

func MapTrackingStatusToText(status string) string {
	switch status {
	case "pending":
		return "Đơn hàng đã được tạo"
	case "confirmed":
		return "Đơn hàng đã được xác nhận"
	case "ready_to_pick":
		return "Chờ bàn giao cho bưu tá"
	case "picking":
		return "Bưu tá đang lấy hàng"
	case "storing":
		return "Hàng đã nhập kho GHN"
	case "shipped", "delivering":
		return "Đang giao hàng"
	case "delivered":
		return "Giao hàng thành công"
	case "cancelled", "auto_cancelled":
		return "Đơn hàng đã hủy"
	case "shipping_cancelled":
		return "Vận đơn giao hàng đã hủy"
	case "return_requested":
		return "Yêu cầu trả hàng"
	case "return_approved":
		return "Yêu cầu trả hàng đã được chấp thuận"
	case "return_rejected":
		return "Yêu cầu trả hàng đã bị từ chối"
	case "return_cancelled":
		return "Yêu cầu trả hàng đã hủy"
	case "returned":
		return "Đã hoàn trả hàng thành công"
	default:
		return status
	}
}

func MapToEventDTOs(events []sqlc.OrderTrackingEvent) []TrackingEventDTO {
	eventDTOs := make([]TrackingEventDTO, 0, len(events))
	for _, event := range events {
		var note *string
		if event.EventNote != nil && strings.TrimSpace(*event.EventNote) != "" {
			note = event.EventNote
		}
		eventDTOs = append(eventDTOs, TrackingEventDTO{
			EventID:    event.EventID,
			Status:     event.EventStatus,
			StatusText: MapTrackingStatusToText(event.EventStatus),
			Note:       note,
			Source:     event.EventSource,
			CreatedAt:  event.EventCreatedAt,
		})
	}
	return eventDTOs
}

func MapReturnStatusToString(status int16) string {
	switch status {
	case 1:
		return "Pending"
	case 2:
		return "Approved"
	case 3:
		return "Rejected"
	case 4:
		return "Completed"
	case 5:
		return "Cancelled"
	default:
		return "Unknown"
	}
}

func MapReturnStatusToVietnamese(status int16) string {
	switch status {
	case 1:
		return "Chờ duyệt"
	case 2:
		return "Đã chấp thuận (Chờ gửi hàng về)"
	case 3:
		return "Đã từ chối"
	case 4:
		return "Đã hoàn tất (Đã nhận hàng & hoàn tiền)"
	case 5:
		return "Đã hủy bởi người mua"
	default:
		return "Không xác định"
	}
}
