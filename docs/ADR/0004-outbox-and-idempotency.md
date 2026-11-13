# ADR 0004 — Outbox and idempotency instead of distributed transactions

- **Status:** Accepted
- **Date:** 2026-11
- **Scope:** order, payment, inventory, analytics, every consumer
- **Code:** `backend/gomod/shared/pkg/kafka`, `backend/gomod/services/*/internal/migrations/*outbox*`

## Context

A checkout changes data in order and must tell four other services. Two obvious designs both
fail: write the row and publish the event separately (the publish can fail after the commit),
or wrap everything in a distributed transaction (no such thing without 2PC, and 2PC across
Kafka and Postgres is not a realistic option).

## Decision

**Write the event with the row.** Every producing service has an `outbox_events` table. The
business change and the outbox insert commit in the same local transaction. A background
worker polls unpublished rows and publishes them to Kafka, then marks them processed. A
broker outage delays events; it never loses them.

**Consumers are idempotent.** Delivery is at-least-once, so the effect must be too. Each
consumer dedupes on `event_id` plus a business key:

| Consumer | Key |
|---|---|
| analytics | Postgres `processed_events` ledger (survives restarts; replaced the old 24h Redis TTL that double-applied after a flush) |
| payment | provider transaction id + idempotency key, CAS on `PENDING → PAID` |
| inventory | `(ref_type, ref_uuid, product_id)` on the stock movement |
| order | saga state + `Idempotency-Key` on checkout |

**Failed messages go to dead-letter topics**, drained by `scripts/check-dlq.ps1` /
`clear-dlq.ps1` rather than silently dropped.

## Consequences

**Positive**

- No dual-write window: an order cannot exist without its event.
- Publication can retry without rewriting business logic.
- Duplicate delivery is safe, so the broker's guarantees do not need to be perfect.

**Trade-offs**

- Read models are eventually consistent; the dashboard lags a checkout by one event.
- Outbox tables grow until the worker drains them; the worker is a single point of delay.
- A seed that writes SQL directly emits no events, so projections must be rebuilt explicitly
  (`docs/SERVICES.md`, `scripts/seed.sql` analytics section).

## Rejected alternatives

- **Publish inside the request handler, before commit** — a rollback leaves a phantom event; a
  crash between publish and commit does the same.
- **`LISTEN/NOTIFY` or a DB trigger calling Kafka** — moves the integration into the database
  and leaves no retryable row.
- **Kafka transactions (`read_committed`) as the only guarantee** — still does not cover the
  Postgres write, so the outbox would be needed anyway.
- **Redis-TTL dedupe** — what this repo did before: a cache flush replays 24h of events and
  double-counts revenue.
