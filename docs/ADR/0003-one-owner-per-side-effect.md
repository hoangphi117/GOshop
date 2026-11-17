# ADR 0003 — One owner per side effect

- **Status:** Accepted
- **Date:** 2026-11
- **Scope:** order, payment, inventory, notification, analytics, coupon
- **Code:** `backend/gomod/services/*`, `backend/gomod/contracts/events/events.go`

## Context

Checkout touches five services. The natural shortcut is for the orchestrator (order) to finish
the job itself: refund the payment, restock the item, send the email. That works until two
services hold different opinions about the same fact, and nobody can tell which one is true.

## Decision

Every observable side effect has exactly one owning service. Order publishes **intent**
events; owners execute.

| Effect | Owner | Triggered by |
|---|---|---|
| Order and return state | order | its own HTTP/gRPC calls |
| Stock deduction and restock | inventory | gRPC `ReserveStock`, event `order.cancelled`, `order_return.completed` |
| Coupon usage claim and release | coupon | gRPC `ApplyCoupon`, event `order.cancelled` |
| Payment transaction and refund | payment | its own HTTP calls, provider IPN |
| Notification persistence and delivery | notification | lifecycle events |
| Reporting projections | analytics | lifecycle events |
| Product media | media | uploads, `media.uploaded` |

Order does not write another service's tables, and no service calls another service's
database. Restocking on a return is inventory's job, triggered by an order event — not a
direct `UPDATE` from order.

## Consequences

**Positive**

- Each fact has one writer, so there is one place to debug it.
- Side effects are retriable independently: a failed notification does not fail a return.
- The event table in `docs/ARCHITECTURE.md` doubles as the ownership map.

**Trade-offs**

- A side effect happens *after* the fact, not with it (see
  [ADR 0004](0004-outbox-and-idempotency.md)).
- Testing a return restock requires simulating an event, not calling a function.

## Rejected alternatives

- **Orchestrator performs every side effect** — order would need credentials for the payment
  gateway and the stock ledger, and would hold a transaction open across three networks.
- **Choreography with no designated owner** — every service reacting to every event is how
  you get two restocks for one return.
- **Shared "domain" service with all writes** — that is the monolith with extra network hops.
