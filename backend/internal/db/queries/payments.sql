-- name: CreatePaymentTransaction :one
INSERT INTO payment_transactions (
    pt_order_id, 
    pt_idempotency_key, 
    pt_provider,
    pt_txn_ref,
    pt_provider_txn_id, 
    pt_provider_url,
    pt_amount, 
    pt_currency, 
    pt_status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetPaymentByIdempotencyKey :one
SELECT * FROM payment_transactions WHERE pt_idempotency_key = $1;

-- name: GetPaymentByTxnRef :one
SELECT * FROM payment_transactions
WHERE pt_provider = $1 AND pt_txn_ref = $2;

-- name: GetPaymentByProviderTxnID :one
SELECT * FROM payment_transactions
WHERE pt_provider = $1 AND pt_provider_txn_id = $2;

-- name: InsertWebhookEvent :one
INSERT INTO webhook_events (we_provider, we_external_event_id, we_event_type, we_raw_body)
VALUES ($1, $2, $3, $4)
ON CONFLICT (we_provider, we_external_event_id) DO NOTHING
RETURNING *;

-- name: MarkWebhookEventProcessed :exec
UPDATE webhook_events SET we_processed = TRUE WHERE we_id = $1;

-- name: UpdatePaymentStatusAtomically :one
UPDATE payment_transactions
SET
    pt_status          = sqlc.arg(new_status),
    pt_provider_txn_id = COALESCE(sqlc.narg('provider_txn_id'), pt_provider_txn_id),
    pt_paid_at         = CASE WHEN sqlc.arg(new_status)::int = 2 THEN now() ELSE pt_paid_at END,
    pt_expired_at      = CASE WHEN sqlc.arg(new_status)::int = 4 THEN now() ELSE pt_expired_at END,
    pt_metadata        = COALESCE(sqlc.narg('metadata')::jsonb, pt_metadata)
WHERE pt_uuid = sqlc.arg(pt_uuid)
  AND pt_status = ANY(sqlc.arg(allowed_from_statuses)::int[])
RETURNING *;

-- name: GetPaymentTransactionsByOrderID :many
SELECT * FROM payment_transactions
WHERE pt_order_id = $1
ORDER BY pt_created_at DESC;

-- name: CreateRefundTransaction :one
INSERT INTO payment_transactions (
    pt_order_id, 
    pt_idempotency_key, 
    pt_provider,
    pt_provider_txn_id, 
    pt_amount, 
    pt_currency, 
    pt_status,
    pt_refund_of, 
    pt_refund_amount,
    pt_paid_at
) VALUES (
    sqlc.arg(order_id),
    sqlc.arg(idempotency_key),
    sqlc.arg(provider),
    sqlc.arg(provider_txn_id),
    sqlc.arg(amount),
    sqlc.arg(currency),
    sqlc.arg(status),
    sqlc.arg(refund_of),
    sqlc.arg(refund_amount),
    now()
)
RETURNING *;

-- name: GetPendingTransactionsOlderThan :many
SELECT 
    pt.pt_id,
    pt.pt_uuid,
    pt.pt_order_id,
    pt.pt_idempotency_key,
    pt.pt_provider,
    pt.pt_txn_ref,
    pt.pt_provider_txn_id,
    pt.pt_provider_url,
    pt.pt_amount,
    pt.pt_currency,
    pt.pt_status,
    pt.pt_refund_of,
    pt.pt_refund_amount,
    pt.pt_paid_at,
    pt.pt_expired_at,
    pt.pt_metadata,
    pt.pt_created_at,
    pt.pt_updated_at,
    o.order_uuid,
    o.order_number,
    o.order_total_amount,
    u.user_uuid,
    u.user_fullname
FROM payment_transactions pt
JOIN orders o ON o.order_id = pt.pt_order_id
JOIN users u ON u.user_id = o.order_user_id
WHERE pt.pt_status = 1
  AND pt.pt_provider != 'cod'
  AND pt.pt_created_at < sqlc.arg(older_than)
ORDER BY pt.pt_created_at ASC
LIMIT sqlc.arg(lim);


