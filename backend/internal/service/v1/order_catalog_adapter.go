package v1service

import (
	"context"
	"goshop/internal/repository"
	"goshop/internal/utils"

	"github.com/google/uuid"
)

type orderCatalogAdapter struct {
	productRepo repository.ProductRepository
}

// NewOrderCatalogAdapter creates an adapter exposing ProductRepository as CatalogPort.
func NewOrderCatalogAdapter(productRepo repository.ProductRepository) CatalogPort {
	return &orderCatalogAdapter{productRepo: productRepo}
}

func (a *orderCatalogAdapter) GetProductsForCheckout(ctx context.Context, productUUIDs []uuid.UUID) (map[string]CheckoutProductSnapshot, error) {
	if len(productUUIDs) == 0 {
		return make(map[string]CheckoutProductSnapshot), nil
	}

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
