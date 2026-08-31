-- name: CreatePaymentTransaction :one
INSERT INTO payment_transactions (
    pt_order_id, pt_idempotency_key, pt_provider,
    pt_provider_txn_id, pt_provider_url,
    pt_amount, pt_currency, pt_status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetPaymentByIdempotencyKey :one
SELECT * FROM payment_transactions WHERE pt_idempotency_key = $1;

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