package v1dto

import "time"

type CreateShippingParams struct {
	UUID string `uri:"uuid" binding:"required,uuid"`
}

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

// Response trả về ngay lập tức cho Admin (HTTP 202)
type AsyncShippingResponseDTO struct {
	OrderUUID string `json:"order_uuid"`
	Status    string `json:"status"` // "processing"
	Message   string `json:"message"`
}
