package service

import (
	"context"
	inventorycontract "goshop/services/inventory/contract"
	"goshop/shared/pkg/utils"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.FailedPrecondition, codes.ResourceExhausted:
				return utils.NewError(utils.ErrCodeConflict, st.Message())
			case codes.NotFound, codes.InvalidArgument:
				return utils.NewError(utils.ErrCodeBadRequest, st.Message())
			}
		}
		return err
	}
	return nil
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
			ProductID:   it.ProductID,
			ProductUUID: it.ProductUUID,
			Quantity:    it.Quantity,
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
