package v1service

import (
	"context"
	inventorycontract "goshop/services/inventory/contract"
)

type orderStockGRPCAdapter struct {
	client inventorycontract.InventoryClient
}

// NewOrderStockGRPCAdapter bridges gRPC InventoryClient to OrderStockPort.
func NewOrderStockGRPCAdapter(client inventorycontract.InventoryClient) OrderStockPort {
	return &orderStockGRPCAdapter{client: client}
}

func (a *orderStockGRPCAdapter) LockAndDeductStockWithRef(
	ctx context.Context,
	items []DeductItem,
	refID string,
	refType string,
	actorID *int32,
	note *string,
) error {
	stockItems := make([]inventorycontract.StockItem, len(items))
	for i, it := range items {
		stockItems[i] = inventorycontract.StockItem{
			ProductUUID: it.ProductUUID,
			Quantity:    it.Quantity,
		}
	}
	_, err := a.client.ReserveStock(ctx, inventorycontract.ReserveStockInput{
		Items:         stockItems,
		ReferenceID:   refID,
		ReferenceType: refType,
		ActorID:       actorID,
		Note:          note,
	})
	return err
}

func (a *orderStockGRPCAdapter) RestoreStockWithRef(
	ctx context.Context,
	items []RestoreItem,
	refID string,
	refType string,
	actorID *int32,
	note *string,
) error {
	stockItems := make([]inventorycontract.StockItem, len(items))
	for i, it := range items {
		stockItems[i] = inventorycontract.StockItem{
			ProductID: it.ProductID,
			Quantity:  it.Quantity,
		}
	}
	return a.client.ReleaseStock(ctx, inventorycontract.ReleaseStockInput{
		Items:         stockItems,
		ReferenceID:   refID,
		ReferenceType: refType,
		ActorID:       actorID,
		Note:          note,
	})
}

