package v1service

import (
	"context"
	"goshop/internal/repository"
	"goshop/internal/utils"
	catalogcontract "goshop/services/catalog/contract"

	"github.com/google/uuid"
)

type orderCatalogAdapter struct {
	productRepo   repository.ProductRepository
	catalogClient catalogcontract.CatalogClient
}

// NewOrderCatalogAdapter creates an adapter exposing ProductRepository or CatalogClient as CatalogPort.
func NewOrderCatalogAdapter(productRepo repository.ProductRepository, catalogClient ...catalogcontract.CatalogClient) CatalogPort {
	var cClient catalogcontract.CatalogClient
	if len(catalogClient) > 0 {
		cClient = catalogClient[0]
	}
	return &orderCatalogAdapter{
		productRepo:   productRepo,
		catalogClient: cClient,
	}
}

func (a *orderCatalogAdapter) GetProductsForCheckout(ctx context.Context, productUUIDs []uuid.UUID) (map[string]CheckoutProductSnapshot, error) {
	if len(productUUIDs) == 0 {
		return make(map[string]CheckoutProductSnapshot), nil
	}

	if a.catalogClient != nil {
		uuidStrs := make([]string, len(productUUIDs))
		for i, u := range productUUIDs {
			uuidStrs[i] = u.String()
		}

		products, err := a.catalogClient.GetProductsBatch(ctx, uuidStrs, nil)
		if err == nil && len(products) > 0 {
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
	}

	if a.productRepo != nil {
		products, err := a.productRepo.GetByUUIDs(ctx, nil, productUUIDs)
		if err != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to fetch products for checkout", err)
		}

		snapshotMap := make(map[string]CheckoutProductSnapshot, len(products))
		for _, p := range products {
			snapshotMap[p.ProductUuid.String()] = CheckoutProductSnapshot{
				ProductID:       p.ProductID,
				ProductUUID:     p.ProductUuid,
				ProductName:     p.ProductName,
				ProductSlug:     p.ProductSlug,
				ProductSku:      p.ProductSku,
				ProductImageURL: p.ProductImageUrl,
				Price:           utils.NumericToFloat(p.ProductPrice),
				Status:          p.ProductStatus,
			}
		}
		return snapshotMap, nil
	}

	return nil, utils.NewError(utils.ErrCodeInternal, "No catalog provider available")
}
