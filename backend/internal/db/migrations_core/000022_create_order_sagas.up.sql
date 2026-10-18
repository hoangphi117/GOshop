-- Migration 000022: Create order_sagas table and active index for Saga Orchestrator
CREATE TABLE IF NOT EXISTS order_sagas (
    order_uuid       UUID PRIMARY KEY,
    user_uuid        UUID NOT NULL,
    idempotency_key  VARCHAR(100) NOT NULL,
    state            VARCHAR(20) NOT NULL DEFAULT 'STARTED',
    last_step        VARCHAR(30),
    payload          JSONB NOT NULL,
    snapshot         JSONB,
    failure_reason   TEXT,
    attempts         INT NOT NULL DEFAULT 0,
    next_retry_at    TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_saga_state CHECK (state IN ('STARTED', 'COMPENSATING', 'CONFIRMED', 'FAILED')),
    CONSTRAINT uq_saga_idem   UNIQUE (user_uuid, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_order_sagas_active
    ON order_sagas (updated_at)
    WHERE state IN ('STARTED', 'COMPENSATING');

CREATE OR REPLACE VIEW orders.order_sagas AS SELECT * FROM public.order_sagas;
