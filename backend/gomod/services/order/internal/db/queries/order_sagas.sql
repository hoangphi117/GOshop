-- name: StartSaga :one
INSERT INTO order_sagas (
    order_uuid,
    user_uuid,
    idempotency_key,
    state,
    last_step,
    payload,
    snapshot
) VALUES (
    $1, $2, $3, 'STARTED', $4, $5, $6
)
ON CONFLICT (user_uuid, idempotency_key) DO NOTHING
RETURNING *;

-- name: ConfirmSaga :execrows
UPDATE order_sagas
SET state = 'CONFIRMED',
    updated_at = NOW()
WHERE order_uuid = $1 AND state = 'STARTED';

-- name: ClaimForCompensation :execrows
UPDATE order_sagas
SET state = 'COMPENSATING',
    failure_reason = $2,
    next_retry_at = NOW() + INTERVAL '30 seconds',
    updated_at = NOW()
WHERE order_uuid = $1 AND state = 'STARTED';

-- name: MarkSagaFailed :execrows
UPDATE order_sagas
SET state = 'FAILED',
    failure_reason = COALESCE($2, failure_reason),
    updated_at = NOW()
WHERE order_uuid = $1 AND state = 'COMPENSATING';

-- name: ScheduleSagaRetry :execrows
UPDATE order_sagas
SET attempts = attempts + 1,
    failure_reason = $2,
    next_retry_at = NOW() + (INTERVAL '30 seconds' * power(2, LEAST(attempts, 6))),
    updated_at = NOW()
WHERE order_uuid = $1 AND state = 'COMPENSATING';

-- name: ClaimStaleSagas :many
UPDATE order_sagas
SET state = 'COMPENSATING',
    updated_at = NOW()
WHERE order_uuid IN (
    SELECT order_uuid
    FROM order_sagas
    WHERE (state = 'STARTED' AND updated_at < NOW() - INTERVAL '2 minutes')
       OR (state = 'COMPENSATING' AND next_retry_at <= NOW())
    ORDER BY updated_at ASC
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: UpdateSagaStep :execrows
UPDATE order_sagas
SET last_step = $2,
    snapshot = $3,
    updated_at = NOW()
WHERE order_uuid = $1 AND state = 'STARTED';

-- name: GetSagaByOrderUUID :one
SELECT * FROM order_sagas
WHERE order_uuid = $1;

-- name: GetSagaByIdempotencyKey :one
SELECT * FROM order_sagas
WHERE user_uuid = $1 AND idempotency_key = $2;
