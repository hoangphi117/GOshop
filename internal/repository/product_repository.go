package repository

import (
	"context"
	"fmt"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	"goshop/internal/utils"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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

func (r *SqlProductRepository) CountProducts(
	ctx context.Context,
	search, categoryUUID string,
	minPrice, maxPrice float64,
	status int32,
	deleted bool,
) (int64, error) {

	params := sqlc.CountProductsParams{
		Deleted: &deleted,
	}

	if search != "" {
		params.Search = &search
	}

	if categoryUUID != "" {
		if parsedUUID, err := uuid.Parse(categoryUUID); err == nil {
			params.CategoryUuid = pgtype.UUID{Bytes: parsedUUID, Valid: true}
		}
	}

	if minPrice > 0 {
		params.MinPrice = utils.FloatToNumeric(minPrice)
	}

	if maxPrice > 0 {
		params.MaxPrice = utils.FloatToNumeric(maxPrice)
	}

	if status > 0 {
		params.Status = &status
	}

	log.Println("CountProducts Params:", params)

	// 7. Gọi sqlc Query
	count, err := r.db.CountProducts(ctx, params)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *SqlProductRepository) GetAll(
	ctx context.Context,
	search, categoryUUID string,
	minPrice, maxPrice float64,
	status int32,
	orderBy, sort string,
	limit, offset int32,
	deleted bool,
) ([]sqlc.GetProductByUUIDRow, error) {

	query := `
		SELECT
			p.product_id,
			p.product_uuid,
			p.product_name,
			p.product_slug,
			p.product_description,
			p.product_price,
			p.product_stock,
			p.product_sku,
			p.product_category_id,
			p.product_image_url,
			p.product_status,
			p.product_deleted_at,
			p.product_created_at,
			p.product_updated_at,
			c.category_uuid AS category_uuid,
			c.category_name AS category_name
		FROM products p
		JOIN categories c ON c.category_id = p.product_category_id
		WHERE 1=1
	`

	args := []any{}
	paramIdx := 1

	if deleted {
		query += " AND p.product_deleted_at IS NOT NULL"
	} else {
		query += " AND p.product_deleted_at IS NULL"
	}

	if strings.TrimSpace(search) != "" {
		query += fmt.Sprintf(" AND (unaccent(p.product_name) ILIKE '%%' || unaccent($%d) || '%%' OR unaccent(p.product_slug) ILIKE '%%' || unaccent($%d) || '%%')", paramIdx, paramIdx)
		args = append(args, strings.TrimSpace(search))
		paramIdx++
	}

	if strings.TrimSpace(categoryUUID) != "" {
		query += fmt.Sprintf(" AND c.category_uuid = $%d::uuid", paramIdx)
		args = append(args, strings.TrimSpace(categoryUUID))
		paramIdx++
	}

	if minPrice > 0 {
		query += fmt.Sprintf(" AND p.product_price >= $%d", paramIdx)
		args = append(args, minPrice)
		paramIdx++
	}

	if maxPrice > 0 {
		query += fmt.Sprintf(" AND p.product_price <= $%d", paramIdx)
		args = append(args, maxPrice)
		paramIdx++
	}

	if status > 0 {
		query += fmt.Sprintf(" AND p.product_status = $%d", paramIdx)
		args = append(args, status)
		paramIdx++
	}

	sortOrder := "DESC"
	if strings.ToLower(strings.TrimSpace(sort)) == "asc" {
		sortOrder = "ASC"
	}

	switch strings.TrimSpace(orderBy) {
	case "product_name":
		query += fmt.Sprintf(" ORDER BY p.product_name %s, p.product_id ASC", sortOrder)
	case "product_price":
		query += fmt.Sprintf(" ORDER BY p.product_price %s, p.product_id ASC", sortOrder)
	default:
		query += fmt.Sprintf(" ORDER BY p.product_created_at %s, p.product_id ASC", sortOrder)
	}

	// Pagination
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", paramIdx, paramIdx+1)
	args = append(args, limit, offset)

	rows, err := db.DBPool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := make([]sqlc.GetProductByUUIDRow, 0)
	for rows.Next() {
		var i sqlc.GetProductByUUIDRow
		if err := rows.Scan(
			&i.ProductID,
			&i.ProductUuid,
			&i.ProductName,
			&i.ProductSlug,
			&i.ProductDescription,
			&i.ProductPrice,
			&i.ProductStock,
			&i.ProductSku,
			&i.ProductCategoryID,
			&i.ProductImageUrl,
			&i.ProductStatus,
			&i.ProductDeletedAt,
			&i.ProductCreatedAt,
			&i.ProductUpdatedAt,
			&i.CategoryUuid,
			&i.CategoryName,
		); err != nil {
			return nil, err
		}
		products = append(products, i)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return products, nil
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
