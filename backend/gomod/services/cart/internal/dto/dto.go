package dto

import "time"

const (
	ProductStatusActive = 1
)

type CartDTO struct {
	UUID        string        `json:"uuid"`
	TotalItems  int32         `json:"total_items"`
	TotalAmount float64       `json:"total_amount"`
	Items       []CartItemDTO `json:"items"`
}

type CartItemDTO struct {
	UUID           string  `json:"uuid"`
	ProductUUID    string  `json:"product_uuid"`
	ProductID      int32   `json:"product_id,omitempty"`
	ProductName    string  `json:"product_name"`
	ProductSlug    string  `json:"product_slug"`
	ProductImage   string  `json:"product_image,omitempty"`
	Price          float64 `json:"price"`
	Quantity       int32   `json:"quantity"`
	Subtotal       float64 `json:"subtotal"`
	StockAvailable int32   `json:"stock_available"`
	IsAvailable    bool    `json:"is_available"`
}

type AddCartItemInput struct {
	ProductUUID string `json:"product_uuid" binding:"required,uuid"`
	Quantity    int32  `json:"quantity" binding:"required,gte=1,lte=99"`
}

type UpdateCartItemQuantityInput struct {
	Quantity *int32 `json:"quantity" binding:"required,gte=0,lte=99"`
}

type UpdateCartItemUUID struct {
	UUID string `uri:"item_uuid" binding:"required,uuid"`
}

type CartItemRecord struct {
	CartItemID        int32
	CartItemUUID      string
	CartID            int32
	ProductUUID       string
	ProductID         *int32
	Quantity          int32
	CartItemCreatedAt time.Time
	CartItemUpdatedAt time.Time
}
