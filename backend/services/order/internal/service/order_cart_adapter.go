package service

import (
	"context"
	cartcontract "goshop/services/cart/contract"

	"github.com/google/uuid"
)

type orderCartAdapter struct {
	client cartcontract.CartClient
}

// NewOrderCartAdapter bridges gRPC CartClient to OrderCartPort and OrderCartUUIDPort.
func NewOrderCartAdapter(client cartcontract.CartClient) OrderCartPort {
	return &orderCartAdapter{client: client}
}

func (a *orderCartAdapter) RemovePurchasedItems(ctx context.Context, userID int32, productIDs []int32) error {
	if a.client == nil {
		return nil
	}
	_, err := a.client.RemovePurchasedItems(ctx, "", userID, nil, productIDs)
	return err
}

func (a *orderCartAdapter) RemovePurchasedItemsByUUID(ctx context.Context, userUUID uuid.UUID, productUUIDs []uuid.UUID) error {
	if a.client == nil {
		return nil
	}
	uuidStrs := make([]string, len(productUUIDs))
	for i, u := range productUUIDs {
		uuidStrs[i] = u.String()
	}
	_, err := a.client.RemovePurchasedItems(ctx, userUUID.String(), 0, uuidStrs, nil)
	return err
}
