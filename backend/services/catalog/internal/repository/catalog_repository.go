package repository

import (
	"context"
	"encoding/json"
	"fmt"
	catalogsqlc "goshop/services/catalog/internal/db/sqlc"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CatalogRepository interface {
	// Categories
	CreateCategory(ctx context.Context, params catalogsqlc.CreateCategoryParams) (catalogsqlc.Category, error)
	CountCategories(ctx context.Context, params catalogsqlc.CountCategoriesParams) (int64, error)
	GetCategoryBySlug(ctx context.Context, slug string) (catalogsqlc.Category, error)
	GetCategoryByUUID(ctx context.Context, u uuid.UUID) (catalogsqlc.Category, error)
	GetCategoryByID(ctx context.Context, id int32) (catalogsqlc.Category, error)
	ListCategories(ctx context.Context, activeOnly *bool) ([]catalogsqlc.Category, error)
	UpdateCategory(ctx context.Context, params catalogsqlc.UpdateCategoryParams) (catalogsqlc.Category, error)
	SoftDeleteCategory(ctx context.Context, u uuid.UUID) (catalogsqlc.Category, error)
	RestoreCategory(ctx context.Context, u uuid.UUID) (catalogsqlc.Category, error)
	TrashCategory(ctx context.Context, u uuid.UUID) (catalogsqlc.Category, error)

	// Products
	CreateProduct(ctx context.Context, params catalogsqlc.CreateProductParams) (catalogsqlc.CreateProductRow, error)
	CountProducts(ctx context.Context, params catalogsqlc.CountProductsParams) (int64, error)
	GetProductBySlug(ctx context.Context, slug string) (catalogsqlc.GetProductBySlugRow, error)
	GetProductByUUID(ctx context.Context, u uuid.UUID) (catalogsqlc.GetProductByUUIDRow, error)
	GetProductByID(ctx context.Context, id int32) (catalogsqlc.GetProductByIDRow, error)
	GetProductsByUUIDs(ctx context.Context, uuids []uuid.UUID) ([]catalogsqlc.GetProductsByUUIDsRow, error)
	GetProductsByIDs(ctx context.Context, ids []int32) ([]catalogsqlc.GetProductsByIDsRow, error)
	GetAllProducts(
		ctx context.Context,
		search, categoryUUID string,
		minPrice, maxPrice, minRating float64,
		status int32,
		orderBy, sort string,
		limit, offset int32,
		deleted bool,
	) ([]catalogsqlc.GetProductByUUIDRow, error)
	UpdateProduct(ctx context.Context, params catalogsqlc.UpdateProductParams) (catalogsqlc.UpdateProductRow, error)
	SoftDeleteProduct(ctx context.Context, u uuid.UUID) (catalogsqlc.Product, error)
	RestoreProduct(ctx context.Context, u uuid.UUID) (catalogsqlc.Product, error)
	TrashProduct(ctx context.Context, u uuid.UUID) (catalogsqlc.Product, error)

	// Outbox
	InsertOutboxEvent(ctx context.Context, aggregateType, aggregateID, eventType string, payload any) (catalogsqlc.OutboxEvent, error)
	GetPendingOutboxEvents(ctx context.Context, limit int32) ([]catalogsqlc.OutboxEvent, error)
	MarkOutboxEventProcessed(ctx context.Context, eventID int64) error
}

type SqlCatalogRepository struct {
	queries *catalogsqlc.Queries
	pool    *pgxpool.Pool
}

func NewSqlCatalogRepository(pool *pgxpool.Pool) CatalogRepository {
	return &SqlCatalogRepository{
		queries: catalogsqlc.New(pool),
		pool:    pool,
	}
}

// Categories
func (r *SqlCatalogRepository) CreateCategory(ctx context.Context, params catalogsqlc.CreateCategoryParams) (catalogsqlc.Category, error) {
	return r.queries.CreateCategory(ctx, params)
}

func (r *SqlCatalogRepository) CountCategories(ctx context.Context, params catalogsqlc.CountCategoriesParams) (int64, error) {
	return r.queries.CountCategories(ctx, params)
}

func (r *SqlCatalogRepository) GetCategoryBySlug(ctx context.Context, slug string) (catalogsqlc.Category, error) {
	return r.queries.GetCategoryBySlug(ctx, slug)
}

func (r *SqlCatalogRepository) GetCategoryByUUID(ctx context.Context, u uuid.UUID) (catalogsqlc.Category, error) {
	return r.queries.GetCategoryByUUID(ctx, u)
}

func (r *SqlCatalogRepository) GetCategoryByID(ctx context.Context, id int32) (catalogsqlc.Category, error) {
	return r.queries.GetCategoryByID(ctx, id)
}

func (r *SqlCatalogRepository) ListCategories(ctx context.Context, activeOnly *bool) ([]catalogsqlc.Category, error) {
	return r.queries.ListCategories(ctx, activeOnly)
}

func (r *SqlCatalogRepository) UpdateCategory(ctx context.Context, params catalogsqlc.UpdateCategoryParams) (catalogsqlc.Category, error) {
	return r.queries.UpdateCategory(ctx, params)
}

func (r *SqlCatalogRepository) SoftDeleteCategory(ctx context.Context, u uuid.UUID) (catalogsqlc.Category, error) {
	return r.queries.SoftDeleteCategory(ctx, u)
}

func (r *SqlCatalogRepository) RestoreCategory(ctx context.Context, u uuid.UUID) (catalogsqlc.Category, error) {
	return r.queries.RestoreCategory(ctx, u)
}

func (r *SqlCatalogRepository) TrashCategory(ctx context.Context, u uuid.UUID) (catalogsqlc.Category, error) {
	return r.queries.TrashCategory(ctx, u)
}

// Products
func (r *SqlCatalogRepository) CreateProduct(ctx context.Context, params catalogsqlc.CreateProductParams) (catalogsqlc.CreateProductRow, error) {
	return r.queries.CreateProduct(ctx, params)
}

func (r *SqlCatalogRepository) CountProducts(ctx context.Context, params catalogsqlc.CountProductsParams) (int64, error) {
	return r.queries.CountProducts(ctx, params)
}

func (r *SqlCatalogRepository) GetProductBySlug(ctx context.Context, slug string) (catalogsqlc.GetProductBySlugRow, error) {
	return r.queries.GetProductBySlug(ctx, slug)
}

func (r *SqlCatalogRepository) GetProductByUUID(ctx context.Context, u uuid.UUID) (catalogsqlc.GetProductByUUIDRow, error) {
	return r.queries.GetProductByUUID(ctx, u)
}

func (r *SqlCatalogRepository) GetProductByID(ctx context.Context, id int32) (catalogsqlc.GetProductByIDRow, error) {
	return r.queries.GetProductByID(ctx, id)
}

func (r *SqlCatalogRepository) GetProductsByUUIDs(ctx context.Context, uuids []uuid.UUID) ([]catalogsqlc.GetProductsByUUIDsRow, error) {
	return r.queries.GetProductsByUUIDs(ctx, uuids)
}

func (r *SqlCatalogRepository) GetProductsByIDs(ctx context.Context, ids []int32) ([]catalogsqlc.GetProductsByIDsRow, error) {
	return r.queries.GetProductsByIDs(ctx, ids)
}

func (r *SqlCatalogRepository) GetAllProducts(
	ctx context.Context,
	search, categoryUUID string,
	minPrice, maxPrice, minRating float64,
	status int32,
	orderBy, sort string,
	limit, offset int32,
	deleted bool,
) ([]catalogsqlc.GetProductByUUIDRow, error) {
	query := `
		SELECT
			p.product_id,
			p.product_uuid,
			p.product_name,
			p.product_slug,
			p.product_description,
			p.product_price,
			p.product_sku,
			p.product_category_id,
			p.product_image_url,
			p.product_status,
			p.product_deleted_at,
			p.product_created_at,
			p.product_updated_at,
			p.product_avg_rating,
			p.product_review_count,
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
		query += fmt.Sprintf(" AND (p.product_name ILIKE '%%' || $%d || '%%' OR p.product_slug ILIKE '%%' || $%d || '%%')", paramIdx, paramIdx)
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

	if minRating > 0 {
		query += fmt.Sprintf(" AND p.product_avg_rating >= $%d", paramIdx)
		args = append(args, minRating)
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
	case "product_avg_rating", "rating":
		query += fmt.Sprintf(" ORDER BY p.product_avg_rating %s, p.product_id ASC", sortOrder)
	default:
		query += fmt.Sprintf(" ORDER BY p.product_created_at %s, p.product_id ASC", sortOrder)
	}

	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", paramIdx, paramIdx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := make([]catalogsqlc.GetProductByUUIDRow, 0)
	for rows.Next() {
		var i catalogsqlc.GetProductByUUIDRow
		if err := rows.Scan(
			&i.ProductID,
			&i.ProductUuid,
			&i.ProductName,
			&i.ProductSlug,
			&i.ProductDescription,
			&i.ProductPrice,
			&i.ProductSku,
			&i.ProductCategoryID,
			&i.ProductImageUrl,
			&i.ProductStatus,
			&i.ProductDeletedAt,
			&i.ProductCreatedAt,
			&i.ProductUpdatedAt,
			&i.ProductAvgRating,
			&i.ProductReviewCount,
			&i.CategoryUuid,
			&i.CategoryName,
		); err != nil {
			return nil, err
		}
		products = append(products, i)
	}

	return products, rows.Err()
}

func (r *SqlCatalogRepository) UpdateProduct(ctx context.Context, params catalogsqlc.UpdateProductParams) (catalogsqlc.UpdateProductRow, error) {
	return r.queries.UpdateProduct(ctx, params)
}

func (r *SqlCatalogRepository) SoftDeleteProduct(ctx context.Context, u uuid.UUID) (catalogsqlc.Product, error) {
	return r.queries.SoftDeleteProduct(ctx, u)
}

func (r *SqlCatalogRepository) RestoreProduct(ctx context.Context, u uuid.UUID) (catalogsqlc.Product, error) {
	return r.queries.RestoreProduct(ctx, u)
}

func (r *SqlCatalogRepository) TrashProduct(ctx context.Context, u uuid.UUID) (catalogsqlc.Product, error) {
	return r.queries.TrashProduct(ctx, u)
}

func (r *SqlCatalogRepository) InsertOutboxEvent(ctx context.Context, aggregateType, aggregateID, eventType string, payload any) (catalogsqlc.OutboxEvent, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return catalogsqlc.OutboxEvent{}, err
	}
	return r.queries.InsertOutboxEvent(ctx, catalogsqlc.InsertOutboxEventParams{
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       b,
	})
}

func (r *SqlCatalogRepository) GetPendingOutboxEvents(ctx context.Context, limit int32) ([]catalogsqlc.OutboxEvent, error) {
	return r.queries.GetPendingOutboxEvents(ctx, limit)
}

func (r *SqlCatalogRepository) MarkOutboxEventProcessed(ctx context.Context, eventID int64) error {
	return r.queries.MarkOutboxEventProcessed(ctx, eventID)
}
