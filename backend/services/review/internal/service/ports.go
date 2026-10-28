package service

import (
	"context"

	catalogcontract "goshop/services/catalog/contract"
	identitycontract "goshop/services/identity/contract"
	ordercontract "goshop/services/order/contract"

	"github.com/google/uuid"
)

// ProductCatalog is the read-only port the review domain needs from catalog.
// It is a subset of catalogcontract.CatalogClient, declared here so tests can
// stub it without dragging in gRPC.
type ProductCatalog interface {
	GetProductByUUID(ctx context.Context, u string) (*catalogcontract.ProductItemInfo, error)
	GetProductsBatch(ctx context.Context, uuids []string, ids []int32) ([]catalogcontract.ProductItemInfo, error)
}

// PurchaseVerifier is the read-only port for "did this user buy the product in
// this order, and may they still review it". Implemented by the order-service
// gRPC contract.
type PurchaseVerifier interface {
	VerifyPurchase(ctx context.Context, orderUUID uuid.UUID, userID, productID int32, userUUID, productUUID *uuid.UUID) (*ordercontract.PurchaseVerification, error)
}

// UserReader looks up profile data used to denormalize reviewer name/email at write time.
type UserReader interface {
	GetUserByID(ctx context.Context, userID int32) (*identitycontract.InternalUserDTO, error)
}
