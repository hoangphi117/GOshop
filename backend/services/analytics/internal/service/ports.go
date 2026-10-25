package service

import (
	"context"
	"time"

	catalogcontract "goshop/services/catalog/contract"
	inventorycontract "goshop/services/inventory/contract"
	ordercontract "goshop/services/order/contract"

	"github.com/google/uuid"
)

// The analytics domain never reads other services' databases. Cross-domain
// reads go through these narrow ports, implemented by the gRPC contract
// clients. Declared here as subsets so tests can stub them without gRPC.

// CustomerCounter reads new-customer counts from the identity service
// (satisfied by identitycontract.GRPCIdentityClient).
type CustomerCounter interface {
	CountCustomersCreatedBetween(ctx context.Context, fromTime, toTime time.Time) (int64, error)
}

// ProductCatalog is the read-only catalog port: batched product dimensions used
// to enrich top-product and low-stock rows with names and images.
type ProductCatalog interface {
	GetProductsBatch(ctx context.Context, uuids []string, ids []int32) ([]catalogcontract.ProductItemInfo, error)
}

// InventoryStockReader reads low-stock alerts from the inventory service.
type InventoryStockReader interface {
	ListLowStockProducts(ctx context.Context, threshold, limit int32) ([]inventorycontract.StockItemInfo, int64, error)
}

// OrderReader enriches projection rows with order details at consume time.
// Implemented by ordercontract.OrderClient.
type OrderReader interface {
	GetOrderSummary(ctx context.Context, orderUUID uuid.UUID) (*ordercontract.OrderSummary, error)
}
