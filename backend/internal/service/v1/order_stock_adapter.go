package v1service

import (
	"context"
)

type orderStockAdapter struct {
	inventoryService InventoryService
}

// NewOrderStockAdapter bridges InventoryService to OrderStockPort.
func NewOrderStockAdapter(inventoryService InventoryService) OrderStockPort {
	return &orderStockAdapter{inventoryService: inventoryService}
}

func (a *orderStockAdapter) LockAndDeductStockWithRef(
	ctx context.Context,
	items []DeductItem,
	refID string,
	refType string,
	actorID *int32,
	note *string,
) error {
	_, err := a.inventoryService.LockAndDeductStockWithRef(ctx, items, refID, refType, actorID, note)
	return err
}

func (a *orderStockAdapter) RestoreStockWithRef(
	ctx context.Context,
	items []RestoreItem,
	refID string,
	refType string,
	actorID *int32,
	note *string,
) error {
	return a.inventoryService.RestoreStockWithRef(ctx, items, refID, refType, actorID, note)
}
