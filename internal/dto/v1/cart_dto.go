package v1dto

import (
	"goshop/internal/db/sqlc"
	"goshop/internal/utils"

	"github.com/google/uuid"
)

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
	ProductName    string  `json:"product_name"`
	ProductSlug    string  `json:"product_slug"`
	ProductImage   string  `json:"product_image,omitempty"`
	Price          float64 `json:"price"`
	Quantity       int32   `json:"quantity"`
	Subtotal       float64 `json:"subtotal"`
	StockAvailable int32   `json:"stock_available"`
	IsAvailable    bool    `json:"is_available"` // false if product is deleted/inactive/out of stock
}

type AddCartItemInput struct {
	ProductUUID string `json:"product_uuid" binding:"required,uuid"`
	Quantity    int32  `json:"quantity" binding:"required,gte=1,lte=99"`
}

func MapCartItemsToDTO(cartUuid uuid.UUID, rows []sqlc.GetCartItemsWithProductDetailsRow) CartDTO {
	items := make([]CartItemDTO, 0, len(rows))
	var totalAmount float64
	var totalItems int32

	for _, row := range rows {
		price := utils.NumericToFloat(row.ProductPrice)
		subtotal := utils.RoundFloat(price*float64(row.CartItemQuantity), 2)
		totalAmount = utils.RoundFloat(totalAmount+subtotal, 2)
		totalItems += row.CartItemQuantity

		isAvailable := !row.ProductDeletedAt.Valid &&
			row.ProductStatus == ProductStatusActive &&
			utils.Int32Value(row.ProductStock) >= row.CartItemQuantity

		items = append(items, CartItemDTO{
			UUID:           row.CartItemUuid.String(),
			ProductUUID:    row.ProductUuid.String(),
			ProductName:    row.ProductName,
			ProductSlug:    row.ProductSlug,
			Price:          price,
			Quantity:       row.CartItemQuantity,
			Subtotal:       subtotal,
			StockAvailable: utils.Int32Value(row.ProductStock),
			IsAvailable:    isAvailable,
		})
	}

	return CartDTO{
		UUID:        cartUuid.String(),
		TotalItems:  totalItems,
		TotalAmount: totalAmount,
		Items:       items,
	}
}
