package dto

import "time"

type StockMovementDTO struct {
	MovementID      int64     `json:"movement_id,omitempty"`
	MovementUUID    string    `json:"movement_uuid"`
	ProductID       *int32    `json:"product_id,omitempty"`
	ProductUUID     string    `json:"product_uuid"`
	ProductName     string    `json:"product_name"`
	ProductSKU      *string   `json:"product_sku,omitempty"`
	ProductImageURL *string   `json:"product_image_url,omitempty"`
	ChangeType      string    `json:"change_type"`
	QuantityChange  int32     `json:"quantity_change"`
	StockBefore     int32     `json:"stock_before"`
	StockAfter      int32     `json:"stock_after"`
	ReferenceID     *string   `json:"reference_id,omitempty"`
	ReferenceType   *string   `json:"reference_type,omitempty"`
	ActorID         *int32    `json:"actor_id,omitempty"`
	ActorUUID       *string   `json:"actor_uuid,omitempty"`
	ActorName       *string   `json:"actor_name,omitempty"`
	Note            *string   `json:"note,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type GetStockMovementsParam struct {
	Page        int     `form:"page" binding:"omitempty,min=1"`
	Limit       int     `form:"limit" binding:"omitempty,min=1,max=100"`
	ProductUUID *string `form:"product_uuid" binding:"omitempty,uuid"`
	ChangeType  *string `form:"change_type" binding:"omitempty"`
	FromDate    *string `form:"from_date" binding:"omitempty"`
	ToDate      *string `form:"to_date" binding:"omitempty"`
	Search      *string `form:"search" binding:"omitempty"`
}

type AdjustStockInput struct {
	ProductUUID    string  `json:"product_uuid" binding:"required,uuid"`
	QuantityChange int32   `json:"quantity_change" binding:"required"`
	ChangeType     string  `json:"change_type" binding:"required,oneof=RESTOCK MANUAL_ADJUSTMENT DAMAGED_DISCARD INITIAL_STOCK"`
	Note           *string `json:"note" binding:"omitempty,max=500"`
}

type ItemStockDTO struct {
	ProductID         int32  `json:"product_id"`
	ProductUUID       string `json:"product_uuid"`
	ProductName       string `json:"product_name"`
	ProductSKU        string `json:"product_sku"`
	ProductImageURL   string `json:"product_image_url"`
	Quantity          int32  `json:"quantity"`
	ReservedQuantity  int32  `json:"reserved_quantity"`
	AvailableQuantity int32  `json:"available_quantity"`
}

type DeductItem struct {
	ProductUUID string `json:"product_uuid"`
	Quantity    int32  `json:"quantity"`
}

type RestoreItem struct {
	ProductID   int32  `json:"product_id"`
	ProductUUID string `json:"product_uuid"`
	Quantity    int32  `json:"quantity"`
}
