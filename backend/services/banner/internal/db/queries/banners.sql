-- name: GetActiveBannersByPosition :many
SELECT banner_id, banner_uuid, banner_title, banner_image_url, banner_link_url,
       banner_position, banner_order, banner_is_active, banner_start_at, banner_end_at,
       banner_created_at, banner_updated_at
FROM banners
WHERE banner_position = $1
  AND banner_is_active = true
  AND banner_deleted_at IS NULL
  AND (banner_start_at IS NULL OR banner_start_at <= now())
  AND (banner_end_at IS NULL OR banner_end_at >= now())
ORDER BY banner_order ASC, banner_created_at DESC;

-- name: CreateBanner :one
INSERT INTO banners (
    banner_title,
    banner_image_url,
    banner_link_url,
    banner_position,
    banner_order,
    banner_is_active,
    banner_start_at,
    banner_end_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: GetBannerByUUID :one
SELECT * FROM banners
WHERE banner_uuid = $1
LIMIT 1;

-- name: AdminListBanners :many
SELECT * FROM banners
WHERE (
    sqlc.narg('position')::text IS NULL OR banner_position = sqlc.narg('position')
) AND (
    sqlc.narg('is_active')::bool IS NULL OR banner_is_active = sqlc.narg('is_active')
) AND (
    sqlc.narg('deleted')::bool IS NULL
    OR (sqlc.narg('deleted')::bool = true AND banner_deleted_at IS NOT NULL)
    OR (sqlc.narg('deleted')::bool = false AND banner_deleted_at IS NULL)
)
ORDER BY banner_order ASC, banner_created_at DESC
LIMIT $1 OFFSET $2;

-- name: AdminCountBanners :one
SELECT count(*) FROM banners
WHERE (
    sqlc.narg('position')::text IS NULL OR banner_position = sqlc.narg('position')
) AND (
    sqlc.narg('is_active')::bool IS NULL OR banner_is_active = sqlc.narg('is_active')
) AND (
    sqlc.narg('deleted')::bool IS NULL
    OR (sqlc.narg('deleted')::bool = true AND banner_deleted_at IS NOT NULL)
    OR (sqlc.narg('deleted')::bool = false AND banner_deleted_at IS NULL)
);

-- name: UpdateBanner :one
UPDATE banners
SET
    banner_title      = COALESCE(sqlc.narg('banner_title'), banner_title),
    banner_image_url  = COALESCE(sqlc.narg('banner_image_url'), banner_image_url),
    banner_link_url   = COALESCE(sqlc.narg('banner_link_url'), banner_link_url),
    banner_position   = COALESCE(sqlc.narg('banner_position'), banner_position),
    banner_order      = COALESCE(sqlc.narg('banner_order'), banner_order),
    banner_is_active  = COALESCE(sqlc.narg('banner_is_active'), banner_is_active),
    banner_start_at   = COALESCE(sqlc.narg('banner_start_at'), banner_start_at),
    banner_end_at     = COALESCE(sqlc.narg('banner_end_at'), banner_end_at),
    banner_updated_at = now()
WHERE banner_uuid = sqlc.arg('banner_uuid')
  AND banner_deleted_at IS NULL
RETURNING *;

-- name: UpdateBannerStatus :one
UPDATE banners
SET
    banner_is_active  = sqlc.arg('banner_is_active'),
    banner_updated_at = now()
WHERE banner_uuid = sqlc.arg('banner_uuid')
  AND banner_deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteBanner :one
UPDATE banners
SET
    banner_deleted_at = now(),
    banner_is_active  = false,
    banner_updated_at = now()
WHERE banner_uuid = sqlc.arg('banner_uuid')
  AND banner_deleted_at IS NULL
RETURNING *;

-- name: RestoreBanner :one
UPDATE banners
SET
    banner_deleted_at = NULL,
    banner_updated_at = now()
WHERE banner_uuid = sqlc.arg('banner_uuid')
  AND banner_deleted_at IS NOT NULL
RETURNING *;

-- name: TrashBanner :one
DELETE FROM banners
WHERE banner_uuid = sqlc.arg('banner_uuid')
  AND banner_deleted_at IS NOT NULL
RETURNING *;
