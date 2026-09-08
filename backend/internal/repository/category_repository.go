package repository

import (
	"context"
	"fmt"
	"goshop/internal/db"
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

func (r *SqlCategoryRepository) CountCategories(ctx context.Context, search string, deleted bool) (int64, error) {
	count, err := r.db.CountCategories(ctx, sqlc.CountCategoriesParams{
		Search:  &search,
		Deleted: &deleted,
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *SqlCategoryRepository) GetAll(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.Category, error) {
	query := `SELECT category_id, category_uuid, category_name, category_slug, category_description, category_status, category_deleted_at, category_created_at, category_updated_at
			FROM categories
			WHERE (
				$1::TEXT IS NULL
				OR $1::TEXT = ''
				OR unaccent(category_name) ILIKE '%' || unaccent($1) || '%'
				OR unaccent(category_slug) ILIKE '%' || unaccent($1) || '%'
			)`

	if deleted {
		query += " AND category_deleted_at IS NOT NULL"
	} else {
		query += " AND category_deleted_at IS NULL"
	}

	order := "ASC"
	if sort == "desc" {
		order = "DESC"
	}

	switch orderBy {
	case "category_id", "category_name", "category_created_at":
		query += fmt.Sprintf(" ORDER BY %s %s", orderBy, order)
	default:
		query += " ORDER BY category_id ASC"
	}

	query += " LIMIT $2 OFFSET $3"

	rows, err := db.DBPool.Query(ctx, query, search, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := []sqlc.Category{}
	for rows.Next() {
		var i sqlc.Category
		if err := rows.Scan(
			&i.CategoryID,
			&i.CategoryUuid,
			&i.CategoryName,
			&i.CategorySlug,
			&i.CategoryDescription,
			&i.CategoryStatus,
			&i.CategoryDeletedAt,
			&i.CategoryCreatedAt,
			&i.CategoryUpdatedAt,
		); err != nil {
			return nil, err
		}
		categories = append(categories, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return categories, nil
}

func (r *SqlCategoryRepository) GetBySlug(ctx context.Context, slug string) (sqlc.Category, error) {
	category, err := r.db.GetCategoryBySlug(ctx, slug)
	if err != nil {
		return sqlc.Category{}, err
	}
	return category, nil
}

func (r *SqlCategoryRepository) GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.Category, error) {
	category, err := r.db.GetCategoryByUuid(ctx, uuid)
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
