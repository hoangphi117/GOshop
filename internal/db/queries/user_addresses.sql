-- name: CreateUserAddress :one
INSERT INTO user_addresses (
    ua_user_id, ua_label, ua_name, ua_phone, ua_address, ua_is_default
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: CountUserAddresses :one
SELECT COUNT(*) FROM user_addresses
WHERE ua_user_id = $1;

-- name: ClearDefaultAddress :exec
UPDATE user_addresses
SET ua_is_default = FALSE
WHERE ua_user_id = $1 AND ua_is_default = TRUE;

-- name: UpdateUserAddress :one
UPDATE user_addresses SET
    ua_label   = COALESCE(sqlc.narg('label'), ua_label),
    ua_name    = COALESCE(sqlc.narg('name'), ua_name),
    ua_phone   = COALESCE(sqlc.narg('phone'), ua_phone),
    ua_address = COALESCE(sqlc.narg('address'), ua_address)
WHERE ua_uuid = $1 AND ua_user_id = $2
RETURNING *;

-- name: SetDefaultAddress :one
UPDATE user_addresses
SET ua_is_default = TRUE
WHERE ua_uuid = $1 AND ua_user_id = $2
RETURNING *;

-- name: DeleteUserAddress :one
DELETE FROM user_addresses
WHERE ua_uuid = $1 AND ua_user_id = $2
RETURNING ua_is_default;

-- name: SetLatestAddressAsDefault :exec
UPDATE user_addresses
SET ua_is_default = TRUE
WHERE ua_id = (
    SELECT sub.ua_id FROM user_addresses sub
    WHERE sub.ua_user_id = $1
    ORDER BY sub.ua_created_at DESC
    LIMIT 1
);
