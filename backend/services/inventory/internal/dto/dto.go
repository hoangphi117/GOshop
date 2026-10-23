package dto

import "time"

type StockMovementDTO struct {
	MovementID      int64     `json:"movement_id,omitempty"`
	MovementUUID    string    `json:"movement_uuid"`
	ProductID       *int32    `json:"product_id,omitempty"`
	ProductUUID     string    `json:"product_uuid"`
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
	ProductUUID string `json:"product_uuid" binding:"required,uuid"`
	// ProductID links the stock row back to the catalog's internal ID; it is
	// only written when the stock row is first created.
	ProductID      *int32 `json:"product_id" binding:"omitempty,min=1"`
	QuantityChange int32  `json:"quantity_change" binding:"required"`
	ChangeType     string  `json:"change_type" binding:"required,oneof=RESTOCK MANUAL_ADJUSTMENT DAMAGED_DISCARD INITIAL_STOCK"`
	Note           *string `json:"note" binding:"omitempty,max=500"`
	// ReferenceID makes the adjustment idempotent: retrying with the same
	// reference returns the original movement instead of applying it twice.
	// Only honoured for INITIAL_STOCK.
	ReferenceID *string `json:"reference_id" binding:"omitempty,max=100"`
	// TargetQuantity sets the stock to an absolute value in one transaction;
	// inherently idempotent. Exclusive with QuantityChange for callers that
	// mean "make stock equal X".
	TargetQuantity *int32 `json:"target_quantity" binding:"omitempty,min=0"`
}

type ItemStockDTO struct {
	ProductID         int32  `json:"product_id"`
	ProductUUID       string `json:"product_uuid"`
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
