CREATE TABLE IF NOT EXISTS payment_transactions (
    pt_id SERIAL PRIMARY KEY,
    pt_uuid UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    pt_order_id INT NOT NULL,
    pt_order_uuid UUID,
    pt_idempotency_key VARCHAR(255) NOT NULL UNIQUE,
    pt_provider VARCHAR(50) NOT NULL,
    pt_txn_ref VARCHAR(255),
    pt_provider_txn_id VARCHAR(255),
    pt_provider_url TEXT,
    pt_amount NUMERIC(15,2) NOT NULL,
    pt_currency VARCHAR(10) NOT NULL DEFAULT 'VND',
    pt_status INT NOT NULL DEFAULT 1,
    pt_refund_of INT REFERENCES payment_transactions(pt_id),
    pt_refund_amount NUMERIC(15,2),
    pt_paid_at TIMESTAMPTZ,
    pt_expired_at TIMESTAMPTZ,
    pt_metadata JSONB,
    pt_created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    pt_updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_payment_transactions_order_id ON payment_transactions(pt_order_id);
CREATE INDEX IF NOT EXISTS idx_payment_transactions_provider_txn_ref ON payment_transactions(pt_provider, pt_txn_ref);
CREATE INDEX IF NOT EXISTS idx_payment_transactions_status_created ON payment_transactions(pt_status, pt_created_at);

CREATE TABLE IF NOT EXISTS webhook_events (
    we_id SERIAL PRIMARY KEY,
    we_provider VARCHAR(50) NOT NULL,
    we_external_event_id VARCHAR(255) NOT NULL,
    we_event_type VARCHAR(100) NOT NULL,
    we_raw_body TEXT NOT NULL,
    we_processed BOOLEAN NOT NULL DEFAULT FALSE,
    we_created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_webhook_provider_event UNIQUE (we_provider, we_external_event_id)
);

CREATE INDEX IF NOT EXISTS idx_webhook_events_processed ON webhook_events(we_processed);

CREATE TABLE IF NOT EXISTS outbox_events (
    oe_id BIGSERIAL PRIMARY KEY,
    oe_aggregate_type VARCHAR(100) NOT NULL,
    oe_aggregate_id VARCHAR(255) NOT NULL,
    oe_event_type VARCHAR(100) NOT NULL,
    oe_payload JSONB NOT NULL,
    oe_status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    oe_created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    oe_processed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_status_created ON outbox_events(oe_status, oe_created_at);
