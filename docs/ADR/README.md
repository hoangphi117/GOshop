# Architecture Decision Records

Decisions that shaped GOshop, one file each, in the format of the original
[`services/order/ADR.md`](../../backend/gomod/services/order/ADR.md): **Status → Context →
Decision → Consequences → Rejected alternatives**.

Read them in order; each assumes the previous.

| # | Decision | Area |
|---|---|---|
| [0001](0001-order-owns-its-data.md) | Order owns its data (one database, one schema) | boundaries |
| [0002](0002-contracts-cross-the-boundary.md) | Contracts (gRPC + Kafka), not tables, cross the boundary | boundaries |
| [0003](0003-one-owner-per-side-effect.md) | One owner per side effect | boundaries |
| [0004](0004-outbox-and-idempotency.md) | Transactional outbox + consumer idempotency | consistency |
| [0005](0005-minimal-resilience.md) | Deadlines and circuit breakers, no retry framework | resilience |
| [0006](0006-health-endpoints.md) | Liveness and readiness are different endpoints | operations |
| [0007](0007-openapi-spec-merge.md) | The OpenAPI spec is merged, not generated in one pass | API |
| [0008](0008-per-client-refresh-cookies.md) | Refresh cookies scoped per frontend | security |
| [0009](0009-r2-in-prod-minio-in-dev.md) | Cloudflare R2 in production, MinIO in development | infrastructure |
| [0010](0010-analytics-read-model.md) | Analytics is a read model, not a query over order tables | analytics |

## Why the original file still exists

[`backend/gomod/services/order/ADR.md`](../../backend/gomod/services/order/ADR.md) is kept
untouched because it is the one document a reviewer of that service opens first. Its six
decisions are the source for 0001–0006 here, expanded with code references, consequences that
actually bit during development, and the alternatives that were tried and dropped.

When a new non-obvious decision is made, add `00NN-<slug>.md` here and a row to the table
above. Do not edit an accepted ADR — supersede it with a new one that says so.
