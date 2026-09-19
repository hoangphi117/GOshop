package v1dto

import (
	"goshop/internal/db/sqlc"

	"github.com/google/uuid"
)

type StockMovementDTO struct {
	MovementUUID    string  `json:"movement_uuid"`
	ProductUUID     string  `json:"product_uuid"`
	ProductName     string  `json:"product_name"`
	ProductSKU      *string `json:"product_sku,omitempty"`
	ProductImageURL *string `json:"product_image_url,omitempty"`
	ChangeType      string  `json:"change_type"`
	QuantityChange  int32   `json:"quantity_change"`
	StockBefore     int32   `json:"stock_before"`
	StockAfter      int32   `json:"stock_after"`
	ReferenceID     *string `json:"reference_id,omitempty"`
	ReferenceType   *string `json:"reference_type,omitempty"`
	ActorUUID       *string `json:"actor_uuid,omitempty"`
	ActorName       *string `json:"actor_name,omitempty"`
	Note            *string `json:"note,omitempty"`
	CreatedAt       string  `json:"created_at"`
}

type GetStockMovementsParam struct {
	Page        int32   `form:"page" binding:"omitempty,gte=1"`
	Limit       int32   `form:"limit" binding:"omitempty,gte=1,lte=100"`
	ProductUUID *string `form:"product_uuid" binding:"omitempty,uuid"`
	ChangeType  *string `form:"change_type" binding:"omitempty,max=30"`
	FromDate    *string `form:"from_date" binding:"omitempty"`
	ToDate      *string `form:"to_date" binding:"omitempty"`
	Search      *string `form:"search" binding:"omitempty,max=100"`
}

type AdjustStockInput struct {
	ProductUUID    string  `json:"product_uuid" binding:"required,uuid"`
	QuantityChange int32   `json:"quantity_change" binding:"required,ne=0"`
	ChangeType     string  `json:"change_type" binding:"required,oneof=RESTOCK MANUAL_ADJUSTMENT DAMAGED_DISCARD INITIAL_STOCK"`
	Note           *string `json:"note" binding:"omitempty,max=500"`
}

func MapStockMovementRowToDTO(row sqlc.ListStockMovementsRow) StockMovementDTO {
	var actorUUID *string
	if row.ActorUuid.Valid {
		s := uuid.UUID(row.ActorUuid.Bytes).String()
		actorUUID = &s
	}

	return StockMovementDTO{
		MovementUUID:    row.MovementUuid.String(),
		ProductUUID:     row.ProductUuid.String(),
		ProductName:     row.ProductName,
		ProductSKU:      row.ProductSku,
		ProductImageURL: row.ProductImageUrl,
		ChangeType:      row.ChangeType,
		QuantityChange:  row.QuantityChange,
		StockBefore:     row.StockBefore,
		StockAfter:      row.StockAfter,
		ReferenceID:     row.ReferenceID,
		ReferenceType:   row.ReferenceType,
		ActorUUID:       actorUUID,
		ActorName:       row.ActorName,
		Note:            row.Note,
		CreatedAt:       formatVNTime(row.CreatedAt),
	}
}
