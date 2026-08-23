package v1dto

import (
	"goshop/internal/db/sqlc"
	"goshop/internal/utils"
)

type OrderDTO struct {
	UUID            string         `json:"uuid"`
	OrderNumber     string         `json:"order_number"`
	UserUUID        string         `json:"user_uuid,omitempty"`
	UserName        string         `json:"user_name,omitempty"`
	ShippingName    string         `json:"shipping_name"`
	ShippingPhone   string         `json:"shipping_phone"`
	ShippingAddress string         `json:"shipping_address"`
	TotalAmount     float64        `json:"total_amount"`
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
	PaymentMethod   int32                  `json:"payment_method" binding:"required,oneof=1 2 3"`
	Note            string                 `json:"note" binding:"omitempty,max=500"`
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
		TotalAmount:     utils.NumericToFloat(order.OrderTotalAmount),
		PaymentMethod:   MapPaymentMethodToString(order.OrderPaymentMethod),
		PaymentStatus:   MapPaymentStatusToString(order.OrderPaymentStatus),
		Note:            note,
		Status:          MapOrderStatusToString(order.OrderStatus),
		Items:           itemsDTO,
		CreatedAt:       order.OrderCreatedAt.Format("2006-01-02 15:04:05"),
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
		return "Refunded"
	default:
		return "Unknown"
	}
}
