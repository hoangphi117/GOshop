-- Product embeddings and per-visitor taste profiles.
--
-- Both tables are read models fed by an out-of-band worker (catalog outbox
-- events -> embedder sidecar -> product_embedding; analytics user_events ->
-- user_affinity). Nothing here is written by the HTTP handlers, so a stale
-- table degrades a shelf but never fails a request.

-- all-MiniLM-L6-v2 emits 384 floats. Keeping the dimension in the column type
-- is what lets an HNSW index exist at all: vector without a dimension cannot
-- be indexed, and the model is pinned in the embedder sidecar.
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS product_embedding (
    product_uuid  UUID PRIMARY KEY,
    embedding     vector(384) NOT NULL,
    model         TEXT        NOT NULL DEFAULT 'all-MiniLM-L6-v2',
    -- Text that produced the vector. Comparing it with the catalog's current
    -- text is how the backfill notices a product whose copy was edited.
    source_hash   TEXT        NOT NULL,
    embedded_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Cosine distance matches the normalised output of the model; inner product
-- would rank longer vectors higher for no reason here.
CREATE INDEX IF NOT EXISTS idx_product_embedding_hnsw
    ON product_embedding USING hnsw (embedding vector_cosine_ops);

-- Affinity is computed on read from analytics_mirror.user_events, so this table
-- is no longer written by anything. It is kept (empty) only because rolling a
-- migration back on a live database is riskier than leaving a dead table; drop
-- it in a follow-up migration once the read path has run in production.
--
-- One row per (visitor, product). visitor_key is 'u:<user_id>' for a signed-in
-- shopper or 'a:<anon_id>' for a guest. Scores would be decayed weights, not
-- counters: the same product viewed twice in a minute must not outrank a
-- purchase that happened last week.
CREATE TABLE IF NOT EXISTS user_affinity (
    visitor_key   TEXT             NOT NULL,
    product_uuid  UUID             NOT NULL,
    score         DOUBLE PRECISION NOT NULL DEFAULT 0,
    last_event_at TIMESTAMPTZ      NOT NULL DEFAULT now(),
    PRIMARY KEY (visitor_key, product_uuid)
);

CREATE INDEX IF NOT EXISTS idx_user_affinity_visitor
    ON user_affinity (visitor_key, score DESC);
