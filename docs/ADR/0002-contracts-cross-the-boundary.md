# ADR 0002 — Contracts cross the boundary, not tables

- **Status:** Accepted
- **Date:** 2026-11
- **Scope:** every service
- **Code:** `backend/gomod/shared/proto`, `backend/gomod/contracts/events`

## Context

With one database per service ([ADR 0001](0001-order-owns-its-data.md)), two services still
need to talk: order needs a product snapshot and a stock deduction on the request path;
analytics needs to know an order was created later.

A synchronous call and an async event solve different problems, and mixing them up is how
distributed systems start lying to themselves.

## Decision

**Synchronous reads and commands** use versioned gRPC contracts under
`backend/gomod/shared/proto/<service>/v1`. Breaking a contract means a new package version,
never an edit in place.

Today: `identity`, `catalog`, `cart`, `order`, `payment`, `inventory`, `coupon`, `chat`.
Those eight expose gRPC; the rest are HTTP-only or event-only (see `docs/SERVICES.md`).

**Asynchronous lifecycle changes** use Kafka events with routing keys declared once in
`backend/gomod/contracts/events/events.go`. Producers write the event to a transactional
outbox in the same transaction as the business change
([ADR 0004](0004-outbox-and-idempotency.md)).

Event envelopes carry `event_id`, `event_version`, `event_type`, `aggregate_id`, `trace_id`
and `occurred_at`.

## Consequences

**Positive**

- Callers compile against a contract, not against a table.
- Service-level tests use `bufconn`; no network needed for contract tests.
- `buf lint` and the breaking-change check run in CI.

**Trade-offs**

- Adding a field is easy; changing a field's meaning is a version bump plus a migration of
  every caller.
- The gateway spec and the proto contracts are two separate surfaces to keep honest.

## Rejected alternatives

- **Shared DTO package imported by all services** — turns every service into a build
  dependency of every other, and a field rename becomes a monorepo-wide refactor.
- **REST everywhere instead of gRPC** — the request path needs a deadline and a status code
  convention per call; gRPC gives both without hand-rolling retry semantics per client.
- **Events for everything, including request-path reads** — a checkout cannot wait for a
  round trip through Kafka to learn a product's price.
