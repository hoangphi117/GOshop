# ADR 0010 — Analytics is a read model, not a query over order tables

- **Status:** Accepted
- **Date:** 2026-11
- **Scope:** analytics, order (event producer), seed
- **Code:** `backend/gomod/services/analytics`, `backend/gomod/services/order/internal/worker` (saga), `scripts/seed.sql`

## Context

The dashboard needs revenue, order counts, status distribution, top products and low-stock
alerts. The obvious implementation is `SELECT ... FROM orders` with joins and aggregates — the
data is right there.

That fails three ways in this architecture: order tables belong to another service
([ADR 0001](0001-order-owns-its-data.md)); the aggregates are heavy enough that the dashboard
would compete with checkout for the same database; and the dashboard needs product and user
dimensions that live in *other* databases again.

## Decision

Analytics owns `shopping_analytics` with two projection tables and no access to any other
service's schema:

- `analytics_orders_projection`
- `analytics_order_items_projection`

They are filled by a Kafka consumer (`order_projection_consumer.go`) that dispatches by routing
key (`app.go`, `dispatchAnalyticsEvent`), claims each `event_id` through the Postgres
`processed_events` ledger, and **upserts** — the same event twice produces the same row.

Product dimensions (name, category, image) are resolved over gRPC at composition time, not
stored.

**Consequence for tooling:** anything that writes orders without emitting events leaves the
projections stale. `scripts/seed.sql` therefore has its own `shopping_analytics` section that
rebuilds both projections from `shopping_order` via `dblink` — otherwise a fresh seed shows
two orders on the dashboard forever.

## Consequences

**Positive**

- Dashboard queries never touch the checkout database's write path.
- Revenue is computed from one place, with one definition of "paid".
- Projections can be rebuilt from scratch without replaying the business logic.

**Trade-offs**

- Eventually consistent: a checkout appears on the dashboard one event later.
- The projection schema must be extended deliberately when a KPI needs a new column.
- Two writers exist (consumer in production, seed SQL locally), so the column list must match
  in both; the seed's analytics section is the thing to update when the consumer changes.

## Rejected alternatives

- **Query `shopping_order` directly from analytics** — cross-service SQL, competing load, and
  the dashboard would break on every order schema change.
- **Materialized views in the order database** — still the order database, still wrong owner.
- **Redis-only dedupe (24h TTL)** — what this repo did first: a cache flush replays events and
  double-counts revenue.
- **Event sourcing the whole order domain** — far larger change than a read model, and the
  order tables are already the source of truth.
