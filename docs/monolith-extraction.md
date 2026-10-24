# Monolith Extraction Plan (7 domains)

Status: `api` container is mid-strangler. Cart, catalog, order, identity, coupon,
inventory, notification, payment already extracted as services. The monolith still
serves 6 real domains + 1 infra concern, and carries 3 dead service files.

## Current monolith modules (`backend/internal/app/app.go:246`)

| Module | Service file (LOC) | Monolith tables | External deps |
|---|---|---|---|
| Review | `review_service.go` (864) | `product_reviews`, `product_review_helpful_votes`, `product_review_reports` | catalog gRPC (values, no cross-DB join), order reader, RabbitMQ |
| Analytics | `analytics_service.go` (515) | `analytics_orders_projection`, `analytics_order_items_projection`, `analytics_order_returns_projection` | fed by worker `order_projection_consumer` |
| Chat | `chat_service.go` (644) | `chat_conversations`, `chat_messages` | wsHub (in-process), Redis bridge, order reader, user repo |
| Banner | `banner_service.go` (261) | `banners` | Redis cache |
| Location | `location_service.go` (143) | none (GHN geo lookup cache) | GHN adapter |
| Upload | `upload_module.go` | none | MinIO |
| WS | `ws_module.go` | none | infrastructure, not a domain |

`address_repository.go` / `user_repository.go` belong to identity concerns already
proxied to identity-service (`/api/v1/addresses` → `identityProxyHandler`).

## Step 0 — delete dead code (no runtime change, ~1.5k LOC)

`router.go` never registers these anymore (products/categories/cart are proxied via
`CATALOG_SERVICE_URL` / `CART_SERVICE_URL` — see `routes.go:236-271`, NoRoute at 345):

- `internal/service/v1/product_service.go` (1003)
- `internal/service/v1/category_service.go` (246)
- `internal/service/v1/cart_service.go` (275)
- their repository slices, if unreferenced after the above

Gate: `go build ./...` + `go test ./internal/...` must pass; if a test file covers a
dead service, delete the test with it.

## Extraction order (cheapest + lowest blast radius first)

Each step repeats the same strangler template, so we only learn it once.

### 1. Banner → `services/banner` (est. 0.5–1 day)

- Own DB `shopping_banner` (add to `scripts/init_databases.sql`), tables: `banners`.
- Pure read model + admin CRUD. Only dep: Redis cache.
- proto `banner/v1/banner.proto`; HTTP under `/api/v1/banners`, `/api/v1/admin/banners`
  (already routed via nginx prefix list — add upstream).

### 2. Location → fold into `services/order` (est. 0.5 day)

- GHN provinces/districts/wards lookup is shipping metadata; `/api/v1/shipping` already
  proxies to order-service. No DB, no consumers besides shipping UX.
- Move `GHNAdapter` + geo cache into order-service; delete monolith LocationModule.
- Alternative (rejected): its own microservice — one 143-LOC client wrapper is not a
  service.

### 3. Review → `services/review` (est. 2–3 days, biggest immediate value)

- Own DB `shopping_review`. The repository already avoided cross-DB joins via catalog
  gRPC (see comment in `review_repository.go:213`) — dependencies are already contract-shaped.
- Needs: `order_reader` replaced by order-service gRPC (purchases verification),
  user sync replaced by identity gRPC, review-approved RabbitMQ events preserved.
- Highest-traffic monolith write path — first real independence win.

### 4. Analytics → `services/analytics` + worker split (est. 2 days)

- Projections `analytics_*_projection` are written by `internal/worker/order_projection_consumer.go`
  (read model — this part is legitimate). Extraction = move projections into
  `shopping_analytics`, point the consumer at it, and move the query API off the monolith.
- The consumer stays a worker; only ownership of the projections moves.

### 5. Chat → `services/chat` (est. 3–4 days, hardest)

- Coupling is `wsHub` (in-process hub) + order reader + user repo.
- Prereq: ws-gateway owns all socket traffic; chat-service talks to it via the existing
  Redis pub/sub bridge instead of the in-process hub. Verify bridge covers every
  `wsHub.Broadcast*` call site before moving.
- DB `shopping_chat`. Real-time domains last — highest integration risk.

### 6. Upload → decision, not extraction

- Two features only: MinIO presigned PUT + serving. Either fold into each consuming
  service (banner needs upload for images — covered by #1) or keep a 2-endpoint
  file service. Decide when #1 lands; do not build a storage platform before then.

### WS — not extracted, consolidated

- ws-gateway container already exists; nginx routes `/api/v1/ws` there. Delete the
  monolith `WSModule` once chat (#5) no longer needs the in-process hub.

## Strangler template per domain (repeat for #1–#5)

1. Add DB to `scripts/init_databases.sql`; service scaffold following `services/cart`
   (it is the smallest complete template: app, contract, gRPC, HTTP, migrations).
2. Proto first under `backend/proto/<domain>/v1/`; `buf lint` + breaking gate in CI.
3. Dual-run: new service deployed; monolith proxies via env (copy the
   `CATALOG_SERVICE_URL` pattern in `routes.go:236`); default env keeps old path.
4. Flip env per environment (dev → prod); watch `/health` + error rates.
5. Delete monolith module, service, repository, routes, DTOs; drop tables from
   `shopping` only after one full release cycle (recoverable via backup).

## Sequencing rule

One domain per PR/branch. Never start #N+1 before #N is fully switched over and the
monolith code deleted — two half-extractions share the same routes.go seams and will
conflict.
