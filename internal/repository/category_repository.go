package repository

import (
	"context"
	"goshop/internal/db/sqlc"

	"github.com/google/uuid"
)

type SqlCategoryRepository struct {
	db sqlc.Querier
}

func NewSqlCategoryRepository(db sqlc.Querier) CategoryRepository {
	return &SqlCategoryRepository{
		db: db,
	}
}

func (r *SqlCategoryRepository) Create(ctx context.Context, categoryParams sqlc.CreateCategoryParams) (sqlc.Category, error) {
	category, err := r.db.CreateCategory(ctx, categoryParams)
	if err != nil {
		return sqlc.Category{}, err
	}
	return category, err
}

func (r *SqlCategoryRepository) GetBySlug(ctx context.Context, slug string) (sqlc.Category, error) {
	category, err := r.db.GetCategoryBySlug(ctx, slug)
	if err != nil {
		return sqlc.Category{}, err
	}
	return category, nil
}

func (r *SqlCategoryRepository) Update(ctx context.Context, categoryParams sqlc.UpdateCategoryParams) (sqlc.Category, error) {
	category, err := r.db.UpdateCategory(ctx, categoryParams)
	if err != nil {
		return sqlc.Category{}, err
	}
	return category, nil
}

func (r *SqlCategoryRepository) SoftDelete(ctx context.Context, uuid uuid.UUID) (sqlc.Category, error) {
	category, err := r.db.SoftDeleteCategory(ctx, uuid)
	if err != nil {
		return sqlc.Category{}, err
	}
	return category, nil
}

func (r *SqlCategoryRepository) Restore(ctx context.Context, uuid uuid.UUID) (sqlc.Category, error) {
	category, err := r.db.RestoreCategory(ctx, uuid)
	if err != nil {
		return sqlc.Category{}, err
	}
	return category, nil
}

func (r *SqlCategoryRepository) Trash(ctx context.Context, uuid uuid.UUID) (sqlc.Category, error) {
	category, err := r.db.TrashCategory(ctx, uuid)
	if err != nil {
		return sqlc.Category{}, err
	}
	return category, nil
}
