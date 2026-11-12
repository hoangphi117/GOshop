-- Co-visitation: which products people look at together.
--
-- Vector closeness answers "this looks like what you browsed". It cannot answer
-- "people who looked at this also looked at that", which is the strongest single
-- signal in e-commerce and the one thing a shared-template description destroys:
-- every product reads as the same sentence, so a camera and a memory card are
-- as far apart as a camera and a phone.
--
-- The table is a materialised aggregate, not a live view: it is written by
-- recommendations.RefreshCovisitation and read on the shelf path, so a request
-- never joins the 174-row event log.
--
-- Its source is analytics events rather than order_items because a demo store
-- has traffic long before it has orders: 174 views against 2 order lines. The
-- purchase signal is folded in at the same weight class when orders exist.
--
-- Undirected by construction: every pair is stored once with the smaller uuid
-- first, so a lookup is one indexed union rather than two scans.
CREATE TABLE product_covisitation (
    source_uuid UUID    NOT NULL,
    target_uuid UUID    NOT NULL,
    visitors    INT     NOT NULL,
    last_seen   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (source_uuid, target_uuid),
    CHECK (source_uuid <> target_uuid)
);

-- The shelf path looks up one side only, and it looks it up in bulk.
CREATE INDEX product_covisitation_target_idx ON product_covisitation (target_uuid, visitors DESC);
CREATE INDEX product_covisitation_visitors_idx ON product_covisitation (visitors DESC);
