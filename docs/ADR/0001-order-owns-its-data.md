# ADR 0001 — Order owns its data

- **Status:** Accepted
- **Date:** 2026-11 (extracted from `services/order/ADR.md`)
- **Scope:** order, and every service that calls it
- **Code:** `backend/gomod/services/order`

## Context

GOshop was a monolith. Order is being extracted into its own module with its own database.
The extraction has to stop being cosmetic: if another service can still `SELECT` from
`orders.*`, the boundary exists only in the folder tree.

## Decision

Order owns the `shopping_order` database and the `orders` schema:

- orders and order items
- order status and payment-status projection fields
- returns, return status, return timeline
- order tracking events
- saga state
- order-owned outbox rows

No other service queries these tables. Shared PostgreSQL infrastructure (one container, many
databases) is acceptable locally, but ownership is expressed by database and schema.

## Consequences

**Positive**

- Schema changes ship with the service that owns them.
- Migrations run independently per service.
- Ownership is visible in `docker-compose.prod.yml` (one database per service).

**Trade-offs**

- Every cross-domain read becomes a gRPC call, a gateway call, or an event.
- Local Compose still shares one PostgreSQL process; the boundary is enforced by convention
  and by the absence of foreign keys, not by the network.

## Rejected alternatives

- **Direct cross-service database reads** — a schema change would break unrelated services and
  nobody could say who owns a column.
- **A shared `public` schema with prefixed tables** — moves the collision from databases to
  table names, and every `sqlc` run regenerates code for tables it does not own.

## Related

- `backend/gomod/services/order/README.md` — ownership, interfaces, failure modes
- `docs/ADR/0002-contracts-cross-the-boundary.md`
