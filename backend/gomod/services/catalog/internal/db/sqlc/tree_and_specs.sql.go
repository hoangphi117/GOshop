package sqlc

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// =============================================================================
// CATEGORY TREE
// =============================================================================

const getCategoriesWithParent = `-- name: GetCategoriesWithParent :many
SELECT category_id, category_uuid, category_name, category_slug, category_description, category_status, category_deleted_at, category_created_at, category_updated_at, parent_id, category_level
FROM categories
WHERE category_deleted_at IS NULL
ORDER BY category_level ASC, category_name ASC
`

func (q *Queries) GetCategoriesWithParent(ctx context.Context) ([]Category, error) {
	rows, err := q.db.Query(ctx, getCategoriesWithParent)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Category
	for rows.Next() {
		var i Category
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
			&i.ParentID,
			&i.CategoryLevel,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const updateCategoryParent = `-- name: UpdateCategoryParent :exec
UPDATE categories
SET parent_id = $2, category_level = $3, category_updated_at = NOW()
WHERE category_uuid = $1
`

func (q *Queries) UpdateCategoryParent(ctx context.Context, u uuid.UUID, parentID *int32, level int32) error {
	_, err := q.db.Exec(ctx, updateCategoryParent, u, parentID, level)
	return err
}

// =============================================================================
// PRODUCT SPECIFICATIONS
// =============================================================================

const updateProductSpecifications = `-- name: UpdateProductSpecifications :exec
UPDATE products
SET product_specifications = $2, product_updated_at = NOW()
WHERE product_uuid = $1
`

func (q *Queries) UpdateProductSpecifications(ctx context.Context, u uuid.UUID, specs map[string]string) error {
	specBytes, err := json.Marshal(specs)
	if err != nil {
		return err
	}
	_, err = q.db.Exec(ctx, updateProductSpecifications, u, specBytes)
	return err
}

const getProductSpecifications = `-- name: GetProductSpecifications :one
SELECT product_specifications
FROM products
WHERE product_uuid = $1 AND product_deleted_at IS NULL
`

func (q *Queries) GetProductSpecifications(ctx context.Context, u uuid.UUID) (map[string]string, error) {
	row := q.db.QueryRow(ctx, getProductSpecifications, u)
	var raw []byte
	if err := row.Scan(&raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return make(map[string]string), nil
	}
	var res map[string]string
	if err := json.Unmarshal(raw, &res); err != nil {
		return make(map[string]string), nil
	}
	return res, nil
}

const getProductsSpecificationsBatch = `-- name: GetProductsSpecificationsBatch :many
SELECT product_id, product_specifications
FROM products
WHERE product_id = ANY($1::int[]) AND product_deleted_at IS NULL
`

type ProductSpecRow struct {
	ProductID             int32             `json:"product_id"`
	ProductSpecifications map[string]string `json:"product_specifications"`
}

func (q *Queries) GetProductsSpecificationsBatch(ctx context.Context, productIDs []int32) (map[int32]map[string]string, error) {
	rows, err := q.db.Query(ctx, getProductsSpecificationsBatch, productIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[int32]map[string]string)
	for rows.Next() {
		var pid int32
		var raw []byte
		if err := rows.Scan(&pid, &raw); err != nil {
			return nil, err
		}
		var specs map[string]string
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &specs)
		}
		if specs == nil {
			specs = make(map[string]string)
		}
		res[pid] = specs
	}
	return res, rows.Err()
}
