package repository

import (
	"context"
	"goshop/internal/db/sqlc"

	"github.com/google/uuid"
)

type SqlProductRepository struct {
	db sqlc.Querier
}

func NewSqlProductRepository(db sqlc.Querier) ProductRepository {
	return &SqlProductRepository{
		db: db,
	}
}

func (r *SqlProductRepository) Create(ctx context.Context, productParams sqlc.CreateProductParams) (sqlc.CreateProductRow, error) {
	product, err := r.db.CreateProduct(ctx, productParams)
	if err != nil {
		return sqlc.CreateProductRow{}, err
	}
	return product, nil
}

func (r *SqlProductRepository) GetBySlug(ctx context.Context, slug string) (sqlc.GetProductBySlugRow, error) {
	product, err := r.db.GetProductBySlug(ctx, slug)
	if err != nil {
		return sqlc.GetProductBySlugRow{}, err
	}
	return product, nil
}

func (r *SqlProductRepository) GetByUUID(ctx context.Context, productUuid uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
	product, err := r.db.GetProductByUUID(ctx, productUuid)
	if err != nil {
		return sqlc.GetProductByUUIDRow{}, err
	}
	return product, nil
}

func (r *SqlProductRepository) Update(ctx context.Context, productParams sqlc.UpdateProductParams) (sqlc.UpdateProductRow, error) {
	product, err := r.db.UpdateProduct(ctx, productParams)
	if err != nil {
		return sqlc.UpdateProductRow{}, err
	}

	return product, nil
}

func (r *SqlProductRepository) SoftDelete(ctx context.Context, uuid uuid.UUID) (sqlc.Product, error) {
	product, err := r.db.SoftDeleteProduct(ctx, uuid)
	if err != nil {
		return sqlc.Product{}, err
	}
	return product, nil
}

func (r *SqlProductRepository) Restore(ctx context.Context, uuid uuid.UUID) (sqlc.Product, error) {
	product, err := r.db.RestoreProduct(ctx, uuid)
	if err != nil {
		return sqlc.Product{}, err
	}
	return product, nil
}

func (r *SqlProductRepository) Trash(ctx context.Context, uuid uuid.UUID) (sqlc.Product, error) {
	product, err := r.db.TrashProduct(ctx, uuid)
	if err != nil {
		return sqlc.Product{}, err
	}
	return product, nil
}
