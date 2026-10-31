# Order Service Architecture Decisions

## Status

Accepted for the learning project.

## Context

GOshop is extracting Order from the monolith. The project needs a clear boundary without adding production-only infrastructure such as Kubernetes, service mesh, or distributed tracing platforms.

## Decision 1: Order owns its data

Order owns the `shopping_order` database and the `orders` schema.

Order-owned tables include:

- Orders.
- Order items.
- Returns.
- Order tracking events.
- Saga state.
- Outbox events.

Other services must not query these tables directly. Shared PostgreSQL infrastructure is acceptable for local learning, but ownership remains separate by database and schema.

## Decision 2: Contracts cross the boundary

Synchronous reads and commands use versioned gRPC contracts under `proto/order/v1`.

Asynchronous lifecycle changes use RabbitMQ events. Order persists events in its outbox within the same transaction as the business change. Publication can retry without losing the original event metadata.

Consumers must tolerate unknown fields and use `event_id` plus a business key for side-effect deduplication.

## Decision 3: One owner per side effect

- Order owns order and return state.
- Inventory owns restock.
- Payment owns refund transactions.
- Notification owns notification persistence.
- Analytics owns reporting projections.

Order publishes intent events. It does not write another service's tables.

## Decision 4: Distributed consistency uses outbox and idempotency

Cross-service workflows do not use database transactions spanning services.

The design uses:

1. Local transaction.
2. Transactional outbox.
3. At-least-once delivery.
4. Consumer-side idempotency.
5. Saga recovery for incomplete workflows.

This gives eventual consistency and safe retries without pretending the workflow is atomic across services.

## Decision 5: Minimal resilience is enough for learning

Each gRPC contract client has:

- A bounded default timeout.
- A circuit breaker for transient dependency failures.
- Caller-provided deadlines when available.

The project does not add a full retry framework or circuit-breaker service. Mutating operations rely on idempotency before any retry is added.

## Decision 6: Health endpoints stay simple

Order exposes:

- `/healthz/live` for process liveness.
- `/healthz/ready` for PostgreSQL and RabbitMQ readiness.

Liveness never checks dependencies. Readiness uses a short timeout and returns `503` when Order cannot access its database or event broker.

Structured logs include `trace_id` for HTTP requests. Event envelopes preserve trace and event identifiers for downstream correlation.

## Rejected alternatives

### Direct cross-service database reads

Rejected because schema changes would break unrelated services and ownership would be unclear.

### Two-phase commit

Rejected because it adds operational complexity and does not fit the learning scope.

### Full production observability stack

Rejected for now. Jaeger, Prometheus, Grafana, Alertmanager, and SLO enforcement belong in a later production-hardening phase.

## Consequences

Positive:

- Clear ownership.
- Independent migrations.
- Safe event retries.
- Failure behavior is testable locally.

Trade-offs:

- Read models are eventually consistent.
- Duplicate delivery remains possible.
- Local Compose still shares infrastructure.
- Manual integration checks require running dependencies.
