# ADR 0006 — Liveness and readiness are different endpoints

- **Status:** Accepted
- **Date:** 2026-11
- **Scope:** every service
- **Code:** `backend/gomod/services/*/internal/handler/health*`, `nginx/conf.d/goshop-locations.inc`

## Context

One `/health` endpoint that pings the database is the classic mistake: when the database is
briefly slow, the orchestrator restarts every healthy pod, the restart storm keeps the
database slow, and a hiccup becomes an outage. The same endpoint also cannot distinguish "this
process is wedged" from "a dependency is down".

## Decision

Two endpoints per service:

- `GET /healthz/live` — process liveness only. Never touches a dependency; if the goroutine
  scheduler answers, the process is alive.
- `GET /healthz/ready` — dependency readiness with a short (2s) timeout: PostgreSQL pool and
  Kafka. Returns `503` when either is unreachable.

The legacy `GET /health` is kept as a compatibility shim only.

Structured logs carry `trace_id`; HTTP requests propagate it via `X-Trace-Id`, and event
envelopes preserve `trace_id` and `event_id` for downstream correlation.

The edge adds its own probe: `/healthz` answers instantly (no upstream), `/health/deep`
proxies the storefront-critical upstream via `auth_request` and returns `503` naming the
dependency.

## Consequences

**Positive**

- A slow database does not restart pods; it removes them from rotation until they recover.
- `/healthz` on the edge stays green while the API is briefly degraded, so an LB does not
  flap the whole site.
- Correlation across HTTP → gRPC → Kafka is possible without a tracing backend.

**Trade-offs**

- Readiness must not check dependencies it does not need, or it will fence a service that is
  still capable of serving its own traffic.
- nginx OSS can only `auth_request` one upstream per location, so `/health/deep` checks the
  single most critical dependency; more requires njs.

## Rejected alternatives

- **Single `/health` pinging everything** — restart storms, as above.
- **Deep health checks on every probe** — the probe itself becomes a load source, and a
  dependency blip takes down the whole fleet at once.
- **OpenTelemetry + Jaeger now** — the trace plumbing (`trace_id` on requests and envelopes) is
  already there; a backend belongs in a later hardening phase.
