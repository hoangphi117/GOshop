-- name: CreatePaymentTransaction :one
INSERT INTO payment_transactions (
    pt_order_id, 
    pt_order_uuid,
    pt_idempotency_key, 
    pt_provider,
    pt_txn_ref,
    pt_provider_txn_id, 
    pt_provider_url,
    pt_amount, 
    pt_currency, 
    pt_status,
    pt_paid_at
) VALUES (
    sqlc.arg(order_id),
    sqlc.narg(order_uuid),
    sqlc.arg(idempotency_key),
    sqlc.arg(provider),
    sqlc.arg(txn_ref),
    sqlc.narg(provider_txn_id),
    sqlc.narg(provider_url),
    sqlc.arg(amount),
    sqlc.arg(currency),
    sqlc.arg(status),
    CASE WHEN sqlc.arg(status)::int = 2 THEN now() ELSE NULL END
)
RETURNING *;

-- name: GetPaymentByIdempotencyKey :one
SELECT * FROM payment_transactions WHERE pt_idempotency_key = $1;

-- name: GetPaymentByUUID :one
SELECT * FROM payment_transactions WHERE pt_uuid = $1;

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
    pt_metadata        = COALESCE(sqlc.narg('metadata')::jsonb, pt_metadata),
    pt_updated_at      = now()
WHERE pt_uuid = sqlc.arg(pt_uuid)
  AND pt_status = ANY(sqlc.arg(allowed_from_statuses)::int[])
RETURNING *;

-- name: GetPaymentTransactionsByOrderID :many
SELECT * FROM payment_transactions
WHERE pt_order_id = $1
ORDER BY pt_created_at DESC;

-- name: GetPaymentTransactionsByOrderUUID :many
SELECT * FROM payment_transactions
WHERE pt_order_uuid = $1
ORDER BY pt_created_at DESC;

-- name: CreateRefundTransaction :one
INSERT INTO payment_transactions (
    pt_order_id, 
    pt_order_uuid,
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
    sqlc.narg(order_uuid),
    sqlc.arg(idempotency_key),
    sqlc.arg(provider),
    sqlc.narg(provider_txn_id),
    sqlc.arg(amount),
    sqlc.arg(currency),
    sqlc.arg(status),
    sqlc.narg(refund_of),
    sqlc.arg(refund_amount),
    now()
)
RETURNING *;

-- name: GetPendingTransactionsOlderThan :many
SELECT *
FROM payment_transactions
WHERE pt_status = 1
  AND pt_provider != 'cod'
  AND pt_created_at < sqlc.arg(older_than)
ORDER BY pt_created_at ASC
LIMIT sqlc.arg(lim);

-- name: UpdatePaymentProviderInfo :one
UPDATE payment_transactions
SET pt_provider_url = sqlc.arg(provider_url),
    pt_provider_txn_id = COALESCE(sqlc.narg('provider_txn_id'), pt_provider_txn_id),
    pt_updated_at = now()
WHERE pt_uuid = sqlc.arg(pt_uuid)
RETURNING *;

-- name: CreateOutboxEvent :one
INSERT INTO outbox_events (
    oe_aggregate_type,
    oe_aggregate_id,
    oe_event_type,
    oe_payload,
    oe_status
) VALUES (
    $1, $2, $3, $4, 'PENDING'
)
RETURNING *;

-- name: GetPendingOutboxEvents :many
SELECT * FROM outbox_events
WHERE oe_status = 'PENDING'
ORDER BY oe_created_at ASC
LIMIT $1;

-- name: MarkOutboxEventPublished :exec
UPDATE outbox_events
SET oe_status = 'PUBLISHED',
    oe_processed_at = now()
WHERE oe_id = $1;
