package v1dto

import (
	"time"

	"goshop/internal/db/sqlc"
	"goshop/internal/utils"

	"github.com/google/uuid"
)

type OrderDTO struct {
	UUID            string         `json:"uuid"`
	OrderNumber     string         `json:"order_number"`
	UserUUID        string         `json:"user_uuid,omitempty"`
	UserName        string         `json:"user_name,omitempty"`
	ShippingName    string         `json:"shipping_name"`
	ShippingPhone   string         `json:"shipping_phone"`
	ShippingAddress string         `json:"shipping_address"`
	CouponCode      *string        `json:"coupon_code,omitempty"`
	SubtotalAmount  float64        `json:"subtotal_amount,omitempty"`
	DiscountAmount  float64        `json:"discount_amount,omitempty"`
	ShippingFee     float64        `json:"shipping_fee,omitempty"`
	TotalAmount     float64        `json:"total_amount"`
	RefundedAmount  float64        `json:"refunded_amount"`
	PaymentMethod   string         `json:"payment_method"`
	PaymentStatus   string         `json:"payment_status"`
	Note            string         `json:"note,omitempty"`
	Status          string         `json:"status"`
	Items           []OrderItemDTO `json:"items"`
	CreatedAt       string         `json:"created_at"`
}

type OrderItemDTO struct {
	UUID        string  `json:"uuid"`
	ProductUUID string  `json:"product_uuid"`
	ProductName string  `json:"product_name"`
	ProductSlug string  `json:"product_slug,omitempty"`
	ImageURL    string  `json:"image_url,omitempty"`
	Price       float64 `json:"price"`
	Quantity    int32   `json:"quantity"`
	Subtotal    float64 `json:"subtotal"`
}

type CreateOrderInput struct {
	Items           []CreateOrderItemInput `json:"items" binding:"required,min=1,dive"`
	ShippingName    string                 `json:"shipping_name" binding:"required,min=2,max=100"`
	ShippingPhone   string                 `json:"shipping_phone" binding:"required,min=9,max=20"`
	ShippingAddress string                 `json:"shipping_address" binding:"required,min=5"`
	ShippingFee     float64                `json:"shipping_fee" binding:"required,gte=0"`
	PaymentMethod   int32                  `json:"payment_method" binding:"required,oneof=1 2 3"`
	Note            string                 `json:"note" binding:"omitempty,max=500"`
	CouponCode      *string                `json:"coupon_code" binding:"omitempty,min=2,max=50"`
}

type OrderSummaryDTO struct {
	UUID            string  `json:"uuid"`
	OrderNumber     string  `json:"order_number"`
	ShippingName    string  `json:"shipping_name"`
	ShippingPhone   string  `json:"shipping_phone"`
	ShippingAddress string  `json:"shipping_address"`
	Note            *string `json:"note,omitempty"`
	CouponCode      *string `json:"coupon_code,omitempty"`
	SubtotalAmount  float64 `json:"subtotal_amount,omitempty"`
	DiscountAmount  float64 `json:"discount_amount,omitempty"`
	ShippingFee     float64 `json:"shipping_fee,omitempty"`
	TotalAmount     float64 `json:"total_amount"`
	RefundedAmount  float64 `json:"refunded_amount"`
	PaymentMethod   string  `json:"payment_method"`
	PaymentStatus   string  `json:"payment_status"`
	Status          string         `json:"status"`
	Items           []OrderItemDTO `json:"items,omitempty"`
	CreatedAt       string         `json:"created_at"`
}

type SyncTrackingResponseDTO struct {
	OrderUUID        string             `json:"order_uuid"`
	OrderNumber      string             `json:"order_number"`
	ShippingCode     string             `json:"shipping_code"`
	ShippingProvider string             `json:"shipping_provider"`
	CurrentStatus    string             `json:"current_status"`
	StatusText       string             `json:"status_text"`
	IsDelivered      bool               `json:"is_delivered"`
	LastSyncAt       time.Time          `json:"last_sync_at"`
	Timeline         []TrackingEventDTO `json:"timeline"`
	Message          string             `json:"message"`
}

type SyncActiveShippingStatsDTO struct {
	ScannedCount   int       `json:"scanned_count"`
	SyncedCount    int       `json:"synced_count"`
	DeliveredCount int       `json:"delivered_count"`
	FailedCount    int       `json:"failed_count"`
	SyncedAt       time.Time `json:"synced_at"`
}

type GetOrdersParam struct {
	Search  string `form:"search" binding:"omitempty,min=2,max=50"`
	Page    int32  `form:"page" binding:"omitempty,gte=1"`
	Limit   int32  `form:"limit" binding:"omitempty,gte=1,lte=100"`
	OrderBy string `form:"order_by" binding:"omitempty,oneof=order_created_at order_total_amount"`
	Sort    string `form:"sort" binding:"omitempty,oneof=asc desc"`
	Status  int32  `form:"status" binding:"omitempty,oneof=1 2 3 4 5 6"`
}

type GetOrderByUUIDParam struct {
	UUID string `uri:"uuid" binding:"required,uuid"`
}

type UpdateOrderStatusInput struct {
	Status int32 `json:"status" binding:"required,oneof=1 2 3 4 5 6"`
}

type UpdateOrderPaymentStatusInput struct {
	PaymentStatus  int32    `json:"payment_status" binding:"required,oneof=2 3 4"`
	RefundAmount   *float64 `json:"refund_amount" binding:"omitempty,gt=0"`
	TransactionRef *string  `json:"transaction_ref" binding:"omitempty,max=100"`
	Note           *string  `json:"note" binding:"omitempty,max=500"`
}

type UpdateOrderShippingInfoInput struct {
	ShippingName    *string `json:"shipping_name" binding:"omitempty,min=2,max=100"`
	ShippingPhone   *string `json:"shipping_phone" binding:"omitempty,min=9,max=20"`
	ShippingAddress *string `json:"shipping_address" binding:"omitempty,min=5"`
	Note            *string `json:"note" binding:"omitempty,max=500"`
}

type CreateOrderItemInput struct {
	ProductUUID string `json:"product_uuid" binding:"required,uuid"`
	Quantity    int32  `json:"quantity" binding:"required,gte=1,lte=99"`
}

func MapToOrderItemRowDTO(item sqlc.GetOrderItemsByOrderIDRow) OrderItemDTO {
	imageURL := ""
	if item.ProductImageUrl != nil {
		imageURL = *item.ProductImageUrl
	}

	return OrderItemDTO{
		UUID:        item.OrderItemUuid.String(),
		ProductUUID: item.ProductUuid.String(),
		ProductName: item.OrderItemProductName,
		ProductSlug: item.ProductSlug,
		ImageURL:    imageURL,
		Price:       utils.NumericToFloat(item.OrderItemPrice),
		Quantity:    item.OrderItemQuantity,
		Subtotal:    utils.NumericToFloat(item.OrderItemSubtotal),
	}
}

var vnLocation = time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)

func formatVNTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(vnLocation).Format("2006-01-02 15:04:05")
}

func MapGetOrderByUUIDRowToOrderDTO(order sqlc.GetOrderByUUIDRow, items []sqlc.GetOrderItemsByOrderIDRow) OrderDTO {
	itemsDTO := make([]OrderItemDTO, len(items))
	for i, item := range items {
		itemsDTO[i] = MapToOrderItemRowDTO(item)
	}
	note := ""
	if order.OrderNote != nil {
		note = *order.OrderNote
	}
	return OrderDTO{
		UUID:            order.OrderUuid.String(),
		OrderNumber:     order.OrderNumber,
		UserUUID:        order.UserUuid.String(),
		UserName:        order.UserFullname,
		ShippingName:    order.OrderShippingName,
		ShippingPhone:   order.OrderShippingPhone,
		ShippingAddress: order.OrderShippingAddress,
		CouponCode:      order.OrderCouponCode,
		SubtotalAmount:  utils.NumericToFloat(order.OrderSubtotalAmount),
		DiscountAmount:  utils.NumericToFloat(order.OrderDiscountAmount),
		ShippingFee:     utils.NumericToFloat(order.OrderShippingFee),
		TotalAmount:     utils.NumericToFloat(order.OrderTotalAmount),
		RefundedAmount:  utils.NumericToFloat(order.OrderRefundedAmount),
		PaymentMethod:   MapPaymentMethodToString(order.OrderPaymentMethod),
		PaymentStatus:   MapPaymentStatusToString(order.OrderPaymentStatus),
		Note:            note,
		Status:          MapOrderStatusToString(order.OrderStatus),
		Items:           itemsDTO,
		CreatedAt:       formatVNTime(order.OrderCreatedAt),
	}
}

func MapToOrderDTO(order sqlc.Order, items []sqlc.GetOrderItemsByOrderIDRow) OrderDTO {
	itemsDTO := make([]OrderItemDTO, len(items))
	for i, item := range items {
		itemsDTO[i] = MapToOrderItemRowDTO(item)
	}
	note := ""
	if order.OrderNote != nil {
		note = *order.OrderNote
	}
	return OrderDTO{
		UUID:            order.OrderUuid.String(),
		OrderNumber:     order.OrderNumber,
		ShippingName:    order.OrderShippingName,
		ShippingPhone:   order.OrderShippingPhone,
		ShippingAddress: order.OrderShippingAddress,
		ShippingFee:     utils.NumericToFloat(order.OrderShippingFee),
		CouponCode:      order.OrderCouponCode,
		SubtotalAmount:  utils.NumericToFloat(order.OrderSubtotalAmount),
		DiscountAmount:  utils.NumericToFloat(order.OrderDiscountAmount),
		TotalAmount:     utils.NumericToFloat(order.OrderTotalAmount),
		RefundedAmount:  utils.NumericToFloat(order.OrderRefundedAmount),
		PaymentMethod:   MapPaymentMethodToString(order.OrderPaymentMethod),
		PaymentStatus:   MapPaymentStatusToString(order.OrderPaymentStatus),
		Note:            note,
		Status:          MapOrderStatusToString(order.OrderStatus),
		Items:           itemsDTO,
		CreatedAt:       formatVNTime(order.OrderCreatedAt),
	}
}

func MapToOrderSummaryDTO(order sqlc.Order) OrderSummaryDTO {
	return OrderSummaryDTO{
		UUID:            order.OrderUuid.String(),
		OrderNumber:     order.OrderNumber,
		ShippingName:    order.OrderShippingName,
		ShippingPhone:   order.OrderShippingPhone,
		ShippingAddress: order.OrderShippingAddress,
		Note:            order.OrderNote,
		CouponCode:      order.OrderCouponCode,
		SubtotalAmount:  utils.NumericToFloat(order.OrderSubtotalAmount),
		DiscountAmount:  utils.NumericToFloat(order.OrderDiscountAmount),
		ShippingFee:     utils.NumericToFloat(order.OrderShippingFee),
		TotalAmount:     utils.NumericToFloat(order.OrderTotalAmount),
		RefundedAmount:  utils.NumericToFloat(order.OrderRefundedAmount),
		PaymentMethod:   MapPaymentMethodToString(order.OrderPaymentMethod),
		PaymentStatus:   MapPaymentStatusToString(order.OrderPaymentStatus),
		Status:          MapOrderStatusToString(order.OrderStatus),
		CreatedAt:       formatVNTime(order.OrderCreatedAt),
	}
}

func (input *UpdateOrderStatusInput) MapUpdateStatusInputToModel(orderUUID uuid.UUID) sqlc.UpdateOrderStatusParams {
	return sqlc.UpdateOrderStatusParams{
		OrderUuid:   orderUUID,
		OrderStatus: input.Status,
	}
}

func (input *UpdateOrderPaymentStatusInput) MapUpdatePaymentStatusInputToModel(orderUUID uuid.UUID) sqlc.UpdateOrderPaymentStatusParams {
	return sqlc.UpdateOrderPaymentStatusParams{
		OrderUuid:     orderUUID,
		PaymentStatus: input.PaymentStatus,
	}
}

func (input *UpdateOrderShippingInfoInput) MapUpdateShippingInputToModel(uuid uuid.UUID) sqlc.UpdateOrderShippingInfoParams {
	return sqlc.UpdateOrderShippingInfoParams{
		OrderShippingName:    input.ShippingName,
		OrderShippingPhone:   input.ShippingPhone,
		OrderShippingAddress: input.ShippingAddress,
		OrderNote:            input.Note,
		OrderUuid:            uuid,
	}
}

func MapOrderStatusToString(status int32) string {
	switch status {
	case 1:
		return "Pending"
	case 2:
		return "Confirmed"
	case 3:
		return "Shipped"
	case 4:
		return "Delivered"
	case 5:
		return "Cancelled"
	case 6:
		return "Returned"
	default:
		return "Unknown"
	}
}
func MapPaymentMethodToString(method int32) string {
	switch method {
	case 1:
		return "COD"
	case 2:
		return "Bank Transfer"
	case 3:
		return "E-Wallet"
	default:
		return "Unknown"
	}
}

func MapPaymentStatusToString(status int32) string {
	switch status {
	case 1:
		return "Unpaid"
	case 2:
		return "Paid"
	case 3:
		return "Partially Refunded"
	case 4:
		return "Refunded"
	default:
		return "Unknown"
	}
}

func MapOrderStatusToVietnamese(status int32) string {
	return MapOrderStatusToString(status)
}

type ConfirmDeliveryResponseDTO struct {
	OrderUUID         string     `json:"order_uuid"`
	OrderNumber       string     `json:"order_number"`
	OrderStatus       int32      `json:"order_status"`
	StatusName        string     `json:"status_name"`
	StatusText        string     `json:"status_text"`
	PaymentMethod     string     `json:"payment_method"`
	PaymentStatus     int32      `json:"payment_status"`
	PaymentStatusName string     `json:"payment_status_name"`
	DeliveredAt       *time.Time `json:"delivered_at"`
	Message           string     `json:"message"`
}
