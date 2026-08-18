package repository

import (
	"context"
	"goshop/internal/db/sqlc"
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
