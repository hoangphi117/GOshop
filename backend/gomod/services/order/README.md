# Order Service

Order Service owns order lifecycle and return workflow.

## Ownership

### Owns

- Orders and order items.
- Order status and payment-status projection fields.
- Return requests, return status, and return timeline.
- Order-owned outbox events.
- Checkout orchestration and saga recovery.

### Does not own

- Product catalog or product prices.
- Inventory quantities or stock movements.
- Payment transactions or gateway state.
- Cart contents.
- User identity and credentials.
- Analytics read models.

Other services use gRPC contracts or versioned Kafka events. They must not read or write Order tables.

## Interfaces

### HTTP

- HTTP port: `8088`.
- Main API prefix: `/api/v1`.
- Liveness: `GET /healthz/live`.
- Readiness: `GET /healthz/ready`.
- Compatibility health endpoint: `GET /health`.

Readiness checks the Order PostgreSQL pool and Kafka with a two-second timeout. Liveness does not call dependencies.

### gRPC

- gRPC port: `50057`.
- Contract: `proto/order/v1/order.proto`.
- Consumers include Payment, Review, and other service adapters.

Important RPCs:

- `GetOrderSummary`
- `GetOrderForPayment`
- `VerifyPurchase`

### Events

Order writes events to its transactional outbox before publication.

Important routing keys:

- `order.created`
- `order.status_changed`
- `order.return.requested`
- `order.return.updated`
- `order.return.completed`

Events include stable metadata such as `event_id`, `event_version`, `event_type`, `aggregate_id`, `trace_id`, and `occurred_at`.

Side-effect owners:

| Effect | Owner |
| --- | --- |
| Return restock | Inventory |
| Payment refund | Payment |
| Return notification | Notification |
| Analytics projection | Analytics consumer |

## Dependencies

| Dependency | Purpose | Address |
| --- | --- | --- |
| PostgreSQL | Order database | `DB_HOST`, `DB_PORT` |
| Redis | Cache and saga support | `REDIS_ADDR` |
| Kafka | Outbox publication and consumers | `KAFKA_BROKERS` |
| Catalog | Checkout product snapshot | `CATALOG_GRPC_ADDR` |
| Inventory | Reserve and restore stock | `INVENTORY_GRPC_ADDR` |
| Coupon | Validate and apply discounts | `COUPON_GRPC_ADDR` |
| Cart | Remove purchased items | `CART_GRPC_ADDR` |
| Payment | Create and refund payments | `PAYMENT_GRPC_ADDR` |
| Identity | Resolve customer data | `IDENTITY_GRPC_ADDR` |

All gRPC clients use bounded request deadlines and a per-connection circuit breaker.

## Environment

Required values are documented in [`backend/.env.example`](../../.env.example).

Order-specific values:

```text
ORDER_DB_NAME=shopping_order
ORDER_SERVER_PORT=8088
ORDER_GRPC_PORT=:50057
PAYMENT_GRPC_ADDR=localhost:50052
CATALOG_GRPC_ADDR=localhost:50055
INVENTORY_GRPC_ADDR=localhost:50054
COUPON_GRPC_ADDR=localhost:50053
CART_GRPC_ADDR=localhost:50056
IDENTITY_GRPC_ADDR=localhost:50051
```

Use service DNS names in Compose, for example `payment-service:50052`.

## Local development

From `backend`:

```powershell
go run ./cmd/order
```

The repository's hybrid development script starts Order on the host and supporting services in Docker:

```powershell
..\scripts\dev.ps1
```

For the full Compose topology:

```powershell
docker compose -f ..\docker-compose.dev.yml up -d
```

## Tests

Run Order unit, worker, and gRPC contract tests:

```powershell
go test ./services/order/...
```

Run focused resilience and return tests:

```powershell
go test ./pkg/grpcclient ./services/order/... ./services/payment/... ./internal/worker
```

Run static checks:

```powershell
go vet ./services/order/... ./pkg/grpcclient ./internal/worker
```

Run local infrastructure smoke tests:

```powershell
docker compose -f docker-compose.integration.yml up -d
go test -tags=integration ./integration
docker compose -f docker-compose.integration.yml down -v
```

CI runs the same PostgreSQL and Kafka smoke test on Ubuntu.

The gRPC contract tests use `bufconn`. Outbox, saga, return, redelivery, and failure paths use deterministic mocks. Postgres/Kafka integration checks require the Compose stack and are manual by design for this learning project.

## Failure modes

- PostgreSQL unavailable: readiness returns `503`; requests needing Order data fail explicitly.
- Kafka unavailable: outbox rows remain pending or failed and are retried.
- Payment or Inventory unavailable: bounded gRPC timeout and circuit breaker prevent indefinite request blocking.
- Duplicate return event: Inventory business keys prevent duplicate restock.
- Duplicate refund request: Payment idempotency key prevents a second refund.
- Concurrent return update: expected-status update returns a conflict.
- Consumer redelivery: consumers must use event or business keys before side effects.

## Operational checks

```powershell
curl http://localhost:8088/healthz/live
curl http://localhost:8088/healthz/ready
```

Pass `X-Trace-Id` on manual requests to correlate HTTP logs and downstream events.
