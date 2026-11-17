# ADR 0005 — Minimal resilience: deadlines, circuit breakers, no retry framework

- **Status:** Accepted
- **Date:** 2026-11
- **Scope:** every gRPC client
- **Code:** `backend/gomod/shared/pkg/grpcclient`, `backend/gomod/services/*/internal/service/*circuit*`

## Context

A service that calls four dependencies on the request path can hang four ways. The tempting
fix is a retry framework with exponential backoff everywhere, which multiplies load exactly
when a dependency is already struggling.

## Decision

Each gRPC client gets three things and nothing more:

1. A bounded default timeout.
2. A per-connection circuit breaker (`gobreaker`).
3. The caller's deadline, when one is set.

Mutating calls are **not** retried by the framework. They are retried only after the operation
is idempotent at the target ([ADR 0004](0004-outbox-and-idempotency.md)) — normally by the saga,
which knows which step to redo.

## Consequences

**Positive**

- A slow dependency fails fast instead of holding a goroutine and a DB transaction.
- No retry storms: a tripped breaker stops the traffic instead of multiplying it.
- Behaviour is testable locally with a mock that returns `Unavailable`.

**Trade-offs**

- A transient blip surfaces as an error rather than being papered over; the saga or the user
  retries.
- Breaker thresholds are per-connection defaults, not tuned per dependency.

## Rejected alternatives

- **Universal retry middleware** — hides non-idempotent calls behind a retry and turns one
  failure into duplicated side effects.
- **Service mesh with built-in retries/outlier detection** — Kubernetes, Istio and linkerd are
  out of scope for this project.
- **Retry queue per client** — adds a store and a worker per dependency for a problem the
  circuit breaker already bounds.
