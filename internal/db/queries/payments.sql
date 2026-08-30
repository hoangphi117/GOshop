-- name: CreatePaymentTransaction :one
INSERT INTO payment_transactions (
    pt_order_id, pt_idempotency_key, pt_provider,
    pt_provider_txn_id, pt_provider_url,
    pt_amount, pt_currency, pt_status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetPaymentByIdempotencyKey :one
SELECT * FROM payment_transactions WHERE pt_idempotency_key = $1;

-- name: InsertWebhookEvent :one
INSERT INTO webhook_events (we_provider, we_external_event_id, we_event_type, we_raw_body)
VALUES ($1, $2, $3, $4)
ON CONFLICT (we_provider, we_external_event_id) DO NOTHING
RETURNING *;

-- name: MarkWebhookEventProcessed :exec
UPDATE webhook_events SET we_processed = TRUE WHERE we_id = $1;