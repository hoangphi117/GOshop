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