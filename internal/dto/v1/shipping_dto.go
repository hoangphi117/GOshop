package v1dto

import (
	"goshop/internal/db/sqlc"
	"strings"
	"time"
)

// ── Params ───────────────────────────────────────────────

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
	case "return_requested":
		return "Yêu cầu trả hàng"
	case "returned":
		return "Đã hoàn trả hàng"
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
