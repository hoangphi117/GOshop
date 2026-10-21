package service

import (
	"context"
	"goshop/internal/utils"
	catalogcontract "goshop/services/catalog/contract"

	"github.com/google/uuid"
)

type orderCatalogAdapter struct {
	catalogClient catalogcontract.CatalogClient
}

// NewOrderCatalogAdapter creates an adapter exposing CatalogClient as CatalogPort (UUID-only, no local DB fallback).
func NewOrderCatalogAdapter(catalogClient catalogcontract.CatalogClient) CatalogPort {
	return &orderCatalogAdapter{
		catalogClient: catalogClient,
	}
}

func (a *orderCatalogAdapter) GetProductsForCheckout(ctx context.Context, productUUIDs []uuid.UUID) (map[string]CheckoutProductSnapshot, error) {
	if len(productUUIDs) == 0 {
		return make(map[string]CheckoutProductSnapshot), nil
	}

	if a.catalogClient == nil {
		return nil, utils.NewError(utils.ErrCodeInternal, "Catalog client is not initialized")
	}

	uuidStrs := make([]string, len(productUUIDs))
	for i, u := range productUUIDs {
		uuidStrs[i] = u.String()
	}

	products, err := a.catalogClient.GetProductsBatch(ctx, uuidStrs, nil)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to fetch products for checkout from catalog service", err)
	}

	snapshotMap := make(map[string]CheckoutProductSnapshot, len(products))
	for _, p := range products {
		pu, _ := uuid.Parse(p.ProductUUID)
		sku := p.ProductSKU
		img := p.ProductImageURL
		snapshotMap[p.ProductUUID] = CheckoutProductSnapshot{
			ProductID:       p.ProductID,
			ProductUUID:     pu,
			ProductName:     p.ProductName,
			ProductSlug:     p.ProductSlug,
			ProductSku:      &sku,
			ProductImageURL: &img,
			Price:           p.ProductPrice,
			Status:          p.ProductStatus,
		}
	}
	return snapshotMap, nil
}
