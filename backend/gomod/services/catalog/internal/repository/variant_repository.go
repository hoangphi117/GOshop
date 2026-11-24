package repository

import (
	"context"

	catalogsqlc "goshop/services/catalog/internal/db/sqlc"

	"github.com/google/uuid"
)

// VariantRepository handles product variant DB operations.
// It is embedded in CatalogRepository to avoid adding a new service dependency.

// CatalogRepository variant methods — interface additions
// (declared in catalog_repository.go interface block; implementations here)

func (r *SqlCatalogRepository) CreateVariant(ctx context.Context, params catalogsqlc.CreateVariantParams) (catalogsqlc.ProductVariant, error) {
	return r.queries.CreateVariant(ctx, params)
}

func (r *SqlCatalogRepository) GetVariantByUUID(ctx context.Context, u uuid.UUID) (catalogsqlc.ProductVariant, error) {
	return r.queries.GetVariantByUUID(ctx, u)
}

func (r *SqlCatalogRepository) GetVariantByUUIDForUpdate(ctx context.Context, u uuid.UUID) (catalogsqlc.ProductVariant, error) {
	return r.queries.GetVariantByUUIDForUpdate(ctx, u)
}

func (r *SqlCatalogRepository) ListVariantsByProductID(ctx context.Context, productID int32) ([]catalogsqlc.ProductVariant, error) {
	return r.queries.ListVariantsByProductID(ctx, productID)
}

func (r *SqlCatalogRepository) ListActiveVariantsByProductID(ctx context.Context, productID int32) ([]catalogsqlc.ProductVariant, error) {
	return r.queries.ListActiveVariantsByProductID(ctx, productID)
}

func (r *SqlCatalogRepository) UpdateVariant(ctx context.Context, params catalogsqlc.UpdateVariantParams) (catalogsqlc.ProductVariant, error) {
	return r.queries.UpdateVariant(ctx, params)
}

func (r *SqlCatalogRepository) DeleteVariant(ctx context.Context, u uuid.UUID) (catalogsqlc.ProductVariant, error) {
	return r.queries.DeleteVariant(ctx, u)
}

func (r *SqlCatalogRepository) GetVariantsByUUIDs(ctx context.Context, uuids []uuid.UUID) ([]catalogsqlc.ProductVariant, error) {
	return r.queries.GetVariantsByUUIDs(ctx, uuids)
}

func (r *SqlCatalogRepository) CountVariantsByProductID(ctx context.Context, productID int32) (int64, error) {
	return r.queries.CountVariantsByProductID(ctx, productID)
}
