// Hand-written to match sqlc v1.31.1 conventions.
// Source: catalog.sql (PRODUCT VARIANTS section)

package sqlc

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// =============================================================================
// PRODUCT VARIANTS
// =============================================================================

const createVariant = `-- name: CreateVariant :one
INSERT INTO product_variants (
    product_id,
    variant_sku,
    variant_title,
    variant_price,
    variant_compare_at_price,
    variant_image_url,
    variant_attributes,
    variant_position,
    variant_status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING variant_id, variant_uuid, product_id, variant_sku, variant_title, variant_price, variant_compare_at_price, variant_image_url, variant_attributes, variant_position, variant_status, variant_created_at, variant_updated_at
`

type CreateVariantParams struct {
	ProductID              int32          `json:"product_id"`
	VariantSku             string         `json:"variant_sku"`
	VariantTitle           string         `json:"variant_title"`
	VariantPrice           pgtype.Numeric `json:"variant_price"`
	VariantCompareAtPrice  pgtype.Numeric `json:"variant_compare_at_price"`
	VariantImageUrl        *string        `json:"variant_image_url"`
	VariantAttributes      []byte         `json:"variant_attributes"`
	VariantPosition        int32          `json:"variant_position"`
	VariantStatus          int32          `json:"variant_status"`
}

func (q *Queries) CreateVariant(ctx context.Context, arg CreateVariantParams) (ProductVariant, error) {
	row := q.db.QueryRow(ctx, createVariant,
		arg.ProductID,
		arg.VariantSku,
		arg.VariantTitle,
		arg.VariantPrice,
		arg.VariantCompareAtPrice,
		arg.VariantImageUrl,
		arg.VariantAttributes,
		arg.VariantPosition,
		arg.VariantStatus,
	)
	return scanVariant(row)
}

const getVariantByUUID = `-- name: GetVariantByUUID :one
SELECT variant_id, variant_uuid, product_id, variant_sku, variant_title, variant_price, variant_compare_at_price, variant_image_url, variant_attributes, variant_position, variant_status, variant_created_at, variant_updated_at FROM product_variants
WHERE variant_uuid = $1
`

func (q *Queries) GetVariantByUUID(ctx context.Context, variantUuid uuid.UUID) (ProductVariant, error) {
	row := q.db.QueryRow(ctx, getVariantByUUID, variantUuid)
	return scanVariant(row)
}

const getVariantByUUIDForUpdate = `-- name: GetVariantByUUIDForUpdate :one
SELECT variant_id, variant_uuid, product_id, variant_sku, variant_title, variant_price, variant_compare_at_price, variant_image_url, variant_attributes, variant_position, variant_status, variant_created_at, variant_updated_at FROM product_variants
WHERE variant_uuid = $1
FOR UPDATE
`

func (q *Queries) GetVariantByUUIDForUpdate(ctx context.Context, variantUuid uuid.UUID) (ProductVariant, error) {
	row := q.db.QueryRow(ctx, getVariantByUUIDForUpdate, variantUuid)
	return scanVariant(row)
}

const listVariantsByProductID = `-- name: ListVariantsByProductID :many
SELECT variant_id, variant_uuid, product_id, variant_sku, variant_title, variant_price, variant_compare_at_price, variant_image_url, variant_attributes, variant_position, variant_status, variant_created_at, variant_updated_at FROM product_variants
WHERE product_id = $1
ORDER BY variant_position ASC, variant_id ASC
`

func (q *Queries) ListVariantsByProductID(ctx context.Context, productID int32) ([]ProductVariant, error) {
	rows, err := q.db.Query(ctx, listVariantsByProductID, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ProductVariant{}
	for rows.Next() {
		v, err := scanVariant(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

const listActiveVariantsByProductID = `-- name: ListActiveVariantsByProductID :many
SELECT variant_id, variant_uuid, product_id, variant_sku, variant_title, variant_price, variant_compare_at_price, variant_image_url, variant_attributes, variant_position, variant_status, variant_created_at, variant_updated_at FROM product_variants
WHERE product_id = $1
AND variant_status = 1
ORDER BY variant_position ASC, variant_id ASC
`

func (q *Queries) ListActiveVariantsByProductID(ctx context.Context, productID int32) ([]ProductVariant, error) {
	rows, err := q.db.Query(ctx, listActiveVariantsByProductID, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ProductVariant{}
	for rows.Next() {
		v, err := scanVariant(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

type UpdateVariantParams struct {
	VariantUuid           uuid.UUID      `json:"variant_uuid"`
	VariantSku            *string        `json:"variant_sku"`
	VariantTitle          *string        `json:"variant_title"`
	VariantPrice          pgtype.Numeric `json:"variant_price"`
	VariantCompareAtPrice pgtype.Numeric `json:"variant_compare_at_price"`
	VariantImageUrl       *string        `json:"variant_image_url"`
	VariantAttributes     []byte         `json:"variant_attributes"`
	VariantPosition       *int32         `json:"variant_position"`
	VariantStatus         *int32         `json:"variant_status"`
}

const updateVariant = `-- name: UpdateVariant :one
UPDATE product_variants SET
    variant_sku                = COALESCE($2::varchar,         variant_sku),
    variant_title              = COALESCE($3::varchar,         variant_title),
    variant_price              = COALESCE($4::numeric,         variant_price),
    variant_compare_at_price   = COALESCE($5::numeric,         variant_compare_at_price),
    variant_image_url          = COALESCE($6::text,            variant_image_url),
    variant_attributes         = COALESCE($7::jsonb,           variant_attributes),
    variant_position           = COALESCE($8::int,             variant_position),
    variant_status             = COALESCE($9::int,             variant_status)
WHERE variant_uuid = $1
RETURNING variant_id, variant_uuid, product_id, variant_sku, variant_title, variant_price, variant_compare_at_price, variant_image_url, variant_attributes, variant_position, variant_status, variant_created_at, variant_updated_at
`

func (q *Queries) UpdateVariant(ctx context.Context, arg UpdateVariantParams) (ProductVariant, error) {
	row := q.db.QueryRow(ctx, updateVariant,
		arg.VariantUuid,
		arg.VariantSku,
		arg.VariantTitle,
		arg.VariantPrice,
		arg.VariantCompareAtPrice,
		arg.VariantImageUrl,
		arg.VariantAttributes,
		arg.VariantPosition,
		arg.VariantStatus,
	)
	return scanVariant(row)
}

const deleteVariant = `-- name: DeleteVariant :one
DELETE FROM product_variants
WHERE variant_uuid = $1
RETURNING variant_id, variant_uuid, product_id, variant_sku, variant_title, variant_price, variant_compare_at_price, variant_image_url, variant_attributes, variant_position, variant_status, variant_created_at, variant_updated_at
`

func (q *Queries) DeleteVariant(ctx context.Context, variantUuid uuid.UUID) (ProductVariant, error) {
	row := q.db.QueryRow(ctx, deleteVariant, variantUuid)
	return scanVariant(row)
}

const getVariantsByUUIDs = `-- name: GetVariantsByUUIDs :many
SELECT variant_id, variant_uuid, product_id, variant_sku, variant_title, variant_price, variant_compare_at_price, variant_image_url, variant_attributes, variant_position, variant_status, variant_created_at, variant_updated_at FROM product_variants
WHERE variant_uuid = ANY($1::uuid[])
`

func (q *Queries) GetVariantsByUUIDs(ctx context.Context, uuids []uuid.UUID) ([]ProductVariant, error) {
	rows, err := q.db.Query(ctx, getVariantsByUUIDs, uuids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ProductVariant{}
	for rows.Next() {
		v, err := scanVariant(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

const countVariantsByProductID = `-- name: CountVariantsByProductID :one
SELECT count(*) FROM product_variants
WHERE product_id = $1
`

func (q *Queries) CountVariantsByProductID(ctx context.Context, productID int32) (int64, error) {
	row := q.db.QueryRow(ctx, countVariantsByProductID, productID)
	var count int64
	err := row.Scan(&count)
	return count, err
}

// scanVariant is a helper that works for both pgx Row and Rows.
type variantScanner interface {
	Scan(dest ...any) error
}

func scanVariant(row variantScanner) (ProductVariant, error) {
	var i ProductVariant
	var attrsRaw []byte
	err := row.Scan(
		&i.VariantID,
		&i.VariantUuid,
		&i.ProductID,
		&i.VariantSku,
		&i.VariantTitle,
		&i.VariantPrice,
		&i.VariantCompareAtPrice,
		&i.VariantImageUrl,
		&attrsRaw,
		&i.VariantPosition,
		&i.VariantStatus,
		&i.VariantCreatedAt,
		&i.VariantUpdatedAt,
	)
	if err != nil {
		return i, err
	}
	if len(attrsRaw) > 0 {
		if err := json.Unmarshal(attrsRaw, &i.VariantAttributes); err != nil {
			return i, err
		}
	}
	return i, nil
}

// Ensure time is used (imported above for ProductVariant.VariantCreatedAt)
var _ = time.Now
