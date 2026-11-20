# Deploy: storefront + dashboard on Vercel, backend on Render

Read this whole file before creating anything. Step 4 has a decision that
changes the dashboard build, and step 7 is not optional — the data migration
has one real blocker (`pgvector` + `postgres_fdw` on managed Postgres).

Everything below assumes three unrelated origins:

| Piece | Origin | What it means |
| --- | --- | --- |
| Storefront | `https://<shop>.vercel.app` | Next.js 16 app router |
| Dashboard | `https://<admin>.vercel.app` | Vite SPA, `base: '/admin/'` |
| Edge API | `https://<api>.onrender.com` | the `api` gateway binary (`cmd/api`) |

Because the three are different *sites*, the browser sees them as
cross-site. That drives several requirements further down; they are marked
**[cross-site]**.

## 0. What this repo already does for you

- `backend/build/Dockerfile.service` builds any Go service with
  `--build-arg SERVICE=<cmd name>`. Render uses it as-is.
- `api` on `:8080` already speaks `/api/v1/*` for **every** domain: it proxies
  `/api/v1/auth`, `/api/v1/users`, `/api/v1/cart`, … to their services by URL
  env var, and serves public `/api/v1/products` without an API key.
  Internal services therefore do **not** need public ingress.
- `CORSMiddleware` reflects whatever `Origin` arrives and allows credentials
  (`shared/pkg/middleware/cors_middleware.go`), so no CORS config is needed.
- `storage.NewStorageService` only supports `s3`, `minio`, `r2` — there is no
  local-disk driver. R2 (or S3) is mandatory.
- Identity runs its own migrations at boot. They run `CREATE EXTENSION`, so
  `DB_USER` must own the database.

---

## 1. Prerequisites (fill these in before touching either dashboard)

Copy this block to a scratch file and fill it as you go. Every value is
referenced by name later.

```env
# --- origins (decide first, everything else derives from them)
EDGE_URL=https://goshop-api.onrender.com
SHOP_URL=https://goshop-shop.vercel.app
ADMIN_URL=https://goshop-admin.vercel.app

# --- Render-managed resources (copy the internal connection URLs Render shows)
PG_INTERNAL_URL=      # postgres://user:pass@dpg-xxxx-a/db  (internal, not the public one)
REDIS_INTERNAL_URL=   # redis://red-xxxxx:6379
KAFKA_BROKER=         # host:port of your Kafka (see step 2)
MEILI_HOST=           # https://<meili>.onrender.com
EMBEDDER_HOST=        # http://goshop-embedder:8096 (internal only)

# --- secrets you generate (never reuse the dev values from backend/.env)
API_KEY=
INTERNAL_API_SECRET=
INTERNAL_API_TOKEN=
JWT_PRIVATE_KEY_PEM=      # single line, \n escaped
JWT_PUBLIC_KEY_PEM=
JWT_KEY_ID=goshop-key-1
JWT_SECRET=
JWT_ENCRYPT_KEY=

# --- object storage (Cloudflare R2)
S3_ENDPOINT=              # https://<account>.r2.cloudflarestorage.com
S3_ACCESS_KEY=
S3_SECRET_KEY=
S3_PUBLIC_BASE_URL=       # https://pub-xxxx.r2.dev
S3_PUBLIC_ORIGIN=         # https://goshop-media.<account>.r2.cloudflarestorage.com

# --- credentials you already own
GOOGLE_CLIENT_ID=
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=
SMTP_PASSWORD=
GHN_TOKEN=
GHN_SHOP_ID=
VNPAY_TMN_CODE=
VNPAY_HASH_SECRET=
```

Timezone: 1–2 hours. Do step 2 before anything else; it is the only step that
can block the whole plan.

---

## 2. Infrastructure on Render (do this first)

Order matters — each service's connection URL only exists after the previous
one is created.

1. **Postgres** → New → Postgres.
   - Version 16 (matches the local `pgvector/pgvector:pg16`).
   - Plan: at least Starter. The free plan has no backups.
   - After it is live, open the **PSQL command** and create the databases.
     They are separate databases on one instance, not separate servers:
     ```sql
     CREATE DATABASE shopping_identity;
     CREATE DATABASE shopping_catalog;
     CREATE DATABASE shopping_cart;
     CREATE DATABASE shopping_coupon;
     CREATE DATABASE shopping_inventory;
     CREATE DATABASE shopping_order;
     CREATE DATABASE shopping_payment;
     CREATE DATABASE shopping_review;
     CREATE DATABASE shopping_chat;
     CREATE DATABASE shopping_analytics;
     CREATE DATABASE shopping_recs;
     CREATE DATABASE shopping_banner;
     CREATE DATABASE shopping_notification;
     CREATE DATABASE shopping_assistant;
     ```
   - Then verify the two extensions that are *not* available everywhere:
     ```sql
     \c shopping_recs
     CREATE EXTENSION IF NOT EXISTS vector;
     CREATE EXTENSION IF NOT EXISTS postgres_fdw;
     \c shopping_inventory
     CREATE EXTENSION IF NOT EXISTS dblink;
     ```
     **If `vector` fails**, stop and move to step 7 — the recommendation
     service cannot run against this instance as configured.

2. **Redis** → New → Key Value. Take the internal URL.

3. **Kafka**. Render has no managed Kafka. Pick one and write the broker
   address down as `KAFKA_BROKER`:
   - **Upstash Kafka** (fastest, free tier): SASL/SSL. Then for every Go
     service set `KAFKA_SASL_USER`, `KAFKA_SASL_PASSWORD`,
     `KAFKA_TLS_ENABLED=true`, and leave `KAFKA_TLS_CA_FILE` **empty** so the
     client uses the system root CAs.
   - **A second Render service** running `apache/kafka:3.9.0` with a
     plaintext internal listener. Same port everywhere, no SASL. Cheapest
     path, but the compose file's SASL_SSL listener wiring does not carry
     over — you would set `KAFKA_BROKERS=<service>:9092` and leave all
     `KAFKA_SASL_*` empty.

   Either way, run the topic creation from
   `docker-compose.prod.yml` → `kafka-init` once, by hand, against the new
   broker. `KAFKA_AUTO_CREATE_TOPICS_ENABLE` is off in the reference setup and
   a mistyped topic fails at consume time, not at boot.

4. **Meilisearch** → New → Web Service (Docker, any image) or the official
   image `getmeili/meilisearch:v1.13`.
   - `MEILI_ENV=production`, `MEILI_MASTER_KEY=<≥16 chars>`,
     `MEILI_NO_ANALYTICS=true`.
   - It must stay publicly reachable if the storefront's client-side
     autocomplete is meant to keep working (step 5, item 6).

---

## 3. Go services on Render

Create **one private service per binary**. All of them use:

- Dockerfile: `backend/build/Dockerfile.service`
- Build context: `backend`
- Build arg: `SERVICE=<name>` (the directory under `backend/gomod/cmd/`)
- **Do not** connect them to Git with auto-deploy for now; use Manual Deploy
  so a push does not roll 15 services at once.

Public ingress: only `api`. Everything else is private and is reached by URL
from `api` (`IDENTITY_SERVICE_URL`, `CATALOG_SERVICE_URL`, …).

### 3.1 Shared environment (every service)

```env
APP_ENV=production
SERVER_PORT=8080
DB_HOST=<host from PG_INTERNAL_URL>
DB_PORT=5432
DB_USER=<user from PG_INTERNAL_URL>
DB_PASSWORD=<password from PG_INTERNAL_URL>
DB_SSL_MODE=require
REDIS_ADDR=<host:port from REDIS_INTERNAL_URL>
REDIS_PASSWORD=<from REDIS_INTERNAL_URL>
KAFKA_BROKERS=<KAFKA_BROKER>
API_KEY=<API_KEY>
INTERNAL_API_SECRET=<INTERNAL_API_SECRET>
INTERNAL_API_TOKEN=<INTERNAL_API_TOKEN>
CORS_ALLOWED_ORIGINS=<SHOP_URL>,<ADMIN_URL>
ANON_COOKIE_SECURE=true
```

Then, per service, the DB name and its peers:

| SERVICE | DB name | extra env |
| --- | --- | --- |
| `api` | (none — it only proxies) | all `*_SERVICE_URL` in 3.2, `KAFKA_SASL_*` for its own producer |
| `identity` | `shopping_identity` | `JWT_SIGNING_ENABLED=true`, `JWT_PRIVATE_KEY=<pem>`, `JWT_PUBLIC_KEY=<pem>`, `JWT_KEY_ID`, `JWT_SECRET`, `JWT_ENCRYPT_KEY`, `GOOGLE_CLIENT_ID`, `SMTP_*`, `MAIL_PROVIDER_TYPE=smtp`, `MEILISEARCH_HOST` |
| `catalog` | `shopping_catalog` | `JWT_SIGNING_ENABLED=false`, `JWT_PUBLIC_KEY=<pem>`, `IDENTITY_SERVICE_URL`, `INVENTORY_GRPC_ADDR=goshop-inventory:50054`, `RECOMMENDATIONS_SERVICE_URL=http://goshop-recommendations:8080`, `MEILISEARCH_HOST` |
| `cart` | `shopping_cart` | `JWT_PUBLIC_KEY`, `CATALOG_GRPC_ADDR=goshop-catalog:50055` |
| `coupon` | `shopping_coupon` | `JWT_PUBLIC_KEY` |
| `inventory` | `shopping_inventory` | `JWT_PUBLIC_KEY`, `CATALOG_DB_NAME=shopping_catalog` |
| `order` | `shopping_order` | `JWT_PUBLIC_KEY`, `CATALOG_GRPC_ADDR`, `INVENTORY_GRPC_ADDR`, `COUPON_GRPC_ADDR`, `CART_GRPC_ADDR`, `IDENTITY_GRPC_ADDR`, `PAYMENT_GRPC_ADDR` |
| `payment` | `shopping_payment` | `JWT_PUBLIC_KEY`, `ORDER_GRPC_ADDR`, `VNPAY_*`, `MOMO_*` |
| `review` | `shopping_review` | `JWT_PUBLIC_KEY`, `IDENTITY_GRPC_ADDR`, `CATALOG_GRPC_TARGET`, `ORDER_GRPC_TARGET` |
| `chat` | `shopping_chat` | `JWT_PUBLIC_KEY`, `IDENTITY_GRPC_ADDR`, `ORDER_GRPC_TARGET` |
| `banner` | `shopping_banner` | `JWT_PUBLIC_KEY` |
| `notification` | `shopping_notification` | `JWT_PUBLIC_KEY`, `IDENTITY_GRPC_ADDR`, `DISCORD_WEBHOOK_URL` (optional) |
| `analytics` | `shopping_analytics` | `JWT_PUBLIC_KEY`, `IDENTITY_GRPC_ADDR`, `ORDER_GRPC_TARGET`, `CATALOG_GRPC_TARGET`, `INVENTORY_GRPC_ADDR` |
| `recommendations` | `shopping_recs` | `CATALOG_DB_NAME=shopping_catalog`, `ANALYTICS_DB_NAME=shopping_analytics`, `CATALOG_SERVICE_URL`, `CATALOG_GRPC_TARGET`, `CATALOG_API_KEY=<API_KEY>`, `EMBEDDER_URL=http://goshop-embedder:8080` |
| `media` | (none) | `SERVICE_NAME=media`, `STORAGE_DRIVER=r2`, `S3_*`; no `JWT_*` needed beyond the verifier pair |
| `assistant` | (none) | `JWT_PUBLIC_KEY`, `CATALOG_GRPC_TARGET`, `CATALOG_BASE_URL`, `ASSISTANT_API_KEY` |
| `worker` | (none) | `JWT_PUBLIC_KEY`, `MEILISEARCH_HOST`, `METRICS_PORT=9464` |
| `ws-gateway` | (none) | `JWT_PUBLIC_KEY`, `REDIS_ADDR`, `IDENTITY_GRPC_ADDR`, `CHAT_GRPC_TARGET` |

Notes that cost an hour each if missed:

- **Port.** The Dockerfile `EXPOSE`s 8080–8095 but each service binds
  `:*_SERVER_PORT` (identity `IDENTITY_SERVER_PORT=8081`, catalog 8086, …).
  Set the container's server port to that number, **or** set the
  `*_SERVER_PORT` var to 8080. Do not mix: `api`'s `IDENTITY_SERVICE_URL`
  must name the port identity actually listens on.
- **gRPC.** `grpc.NewServer` listeners (`IDENTITY_GRPC_PORT=50051`, …) are
  not reachable through Render's HTTP port mapping. `api` is the only thing
  that calls peers over HTTP; but `order`, `review`, `chat`, `cart`,
  `inventory`, `catalog`, `payment`, `identity` and `ws-gateway` all dial
  peers over gRPC at boot. On Render those dials must point at
  `<internal host>:<grpc port>` — which means the private service exposes a
  second port. Render private services expose exactly one port. Two ways out,
  both acceptable for a demo:
  1. Keep the gRPC dials pointing at services you run on the same
     `goshop-network` (i.e. host the compose stack somewhere else and only
     front `api` from Render), **or**
  2. Collapse the gRPC-only paths to HTTP for the demo (cart/order/coupon
     lookups) and accept the missing features listed in the plan.
  This is the main reason the "one service per binary" topology costs
  several deploy cycles. Budget for it.

### 3.2 `api` — the only public service

Run it from `backend/build/Dockerfile.service --build-arg SERVICE=api`,
port 8080, health check `/health`.

```env
IDENTITY_SERVICE_URL=http://goshop-identity:8081
CATALOG_SERVICE_URL=http://goshop-catalog:8086
CART_SERVICE_URL=http://goshop-cart:8087
ORDER_SERVICE_URL=http://goshop-order:8088
COUPON_SERVICE_URL=http://goshop-coupon:8084
PAYMENT_SERVICE_URL=http://goshop-payment:8083
REVIEW_SERVICE_URL=http://goshop-review:8091
CHAT_SERVICE_URL=http://goshop-chat:8093
BANNER_SERVICE_URL=http://goshop-banner:8089
INVENTORY_SERVICE_URL=http://goshop-inventory:8090
ANALYTICS_SERVICE_URL=http://goshop-analytics:8092
MEDIA_SERVICE_URL=http://goshop-media:8094
RECOMMENDATIONS_SERVICE_URL=http://goshop-recommendations:8095
NOTIFICATION_SERVICE_URL=http://goshop-notification:8082
ASSISTANT_SERVICE_URL=http://goshop-assistant:8097
WS_GATEWAY_URL=http://goshop-ws-gateway:8085
STORAGE_DRIVER=r2
S3_ENDPOINT=<S3_ENDPOINT>
S3_ACCESS_KEY=<...>
S3_SECRET_KEY=<...>
S3_PUBLIC_BASE_URL=<S3_PUBLIC_BASE_URL>
S3_USE_PATH_STYLE=false
S3_USE_SSL=true
GHN_TOKEN=<...>
GHN_SHOP_ID=<...>
```

`api` starts a storage client and **fatals** if `STORAGE_DRIVER` is not one
of `s3|minio|r2`, so the `S3_*` block is not optional even though `api` only
redirects `/uploads/*`.

### 3.3 Signing keys

`backend/certs/` is gitignored, so Render has no key files. Generate a fresh
pair for production and paste both PEMs inline, newlines escaped:

```powershell
$priv = Get-Content -Raw backend\certs\jwt_private.pem
$pub  = Get-Content -Raw backend\certs\jwt_public.pem
($priv -replace "`r?`n", '\n')
($pub  -replace "`r?`n", '\n')
```

- `identity` gets `JWT_SIGNING_ENABLED=true` and **both**.
- Every verifier gets `JWT_SIGNING_ENABLED=false` and **only the public key**.
  A verifier without it mints an *ephemeral* key at boot and every token
  401s — the comment at the top of `docker-compose.prod.yml` documents this.
- `loadKeyData` treats the value as a path first and falls back to PEM, so a
  single-line escaped value works, and so does a stray path string (harmless).

---

## 4. Frontend on Vercel — decide the dashboard base first

The dashboard is built with `base: '/admin/'` and a TanStack Router
`basepath: '/admin'`. On Vercel the project gets its **own** origin, so
`https://<admin>.vercel.app/` returns 404 and everything lives under
`https://<admin>.vercel.app/admin/`. That works, but every deep link users
share must carry the prefix.

1. **Keep `/admin/`** — no code change. Vercel's SPA behaviour already
   serves `index.html` for unknown paths, so no `vercel.json` is needed;
   `https://<admin>.vercel.app/admin/` loads and deep links work.
2. **Drop the prefix** — change `base` in `vite.config.ts` to `'/'` **and**
   `basepath` in `src/main.tsx` to `'/'`, then fix `src/lib/auth-redirect.ts`
   and `auth-redirect.test.ts` (they strip `/admin`). Cleaner URLs; ~10
   minutes of work; also touches the local edge setup.

Pick one now. Both Vercel projects need the same monorepo settings:

| Setting | Storefront | Dashboard |
| --- | --- | --- |
| Root Directory | `frontend` | `frontend` |
| Framework | Next.js | Vite |
| Install Command | `pnpm install --frozen-lockfile` | same |
| Build Command | `pnpm --filter storefront build` | `pnpm --filter dashboard build` |
| Output Directory | `.next` (Vercel detects) | `apps/dashboard/dist` |
| Node | 22 | 22 |

Building from `frontend/` (not the app dir) is what lets the workspace
packages resolve; Vercel's "Root Directory = app" would break
`@goshop/api-client`.

### 4.1 Storefront env (Vercel → Project → Settings → Environment Variables)

```env
NEXT_PUBLIC_API_URL=<EDGE_URL>
NEXT_PUBLIC_API_KEY=
INTERNAL_API_URL=<EDGE_URL>
NEXT_PUBLIC_SITE_URL=<SHOP_URL>
GOOGLE_CLIENT_ID=<...>
NEXT_PUBLIC_GOOGLE_CLIENT_ID=<...>
NEXT_PUBLIC_MEILISEARCH_URL=<MEILI_HOST>
NEXT_PUBLIC_MEILISEARCH_SEARCH_KEY=<MEILI_MASTER_KEY>
INTERNAL_MEILISEARCH_URL=<MEILI_HOST>
MEILISEARCH_SEARCH_KEY=<MEILI_MASTER_KEY>
```

`NEXT_PUBLIC_API_KEY` stays empty in production: the edge injects
`X-API-KEY` server-side, and baking it in would ship the key to every
browser.

**[cross-site] Three things must change in `next.config.ts`** because Vercel
copies that file into its own build:

1. `output: "standalone"` is for the Docker image only. It is harmless on
   Vercel but confusing; leaving it is fine.
2. The `/uploads/:path*` rewrite proxies to `INTERNAL_API_URL`. On Vercel
   that is a public Render URL, so every upload read becomes a Vercel→Render
   hop. Either keep it (works, slower) or drop the rewrite and let
   `S3_PUBLIC_BASE_URL` serve media directly.
3. `images.unoptimized: true` stays: media URLs point at R2/MinIO, not at
   hosts Vercel's optimizer can reach.

**[cross-site] Google sign-in.** The GSI button is rendered with
`NEXT_PUBLIC_GOOGLE_CLIENT_ID`, and identity checks
`tokenInfo.Aud == GOOGLE_CLIENT_ID`. In Google Cloud Console → Credentials →
your OAuth client → **Authorized JavaScript origins**, add `<SHOP_URL>`.
Without it the button renders but the popup fails with `origin_mismatch`.
`<ADMIN_URL>` needs no entry unless you add a Google button there.

**[cross-site] refresh cookies.** `Set-Cookie` uses
`SameSite=Lax`, a fixed `Path=/api/v1/auth`, and no `Domain`. A cookie set by
Render is therefore *same-site only* with `<EDGE_URL>`, and the browser will
not send it on a cross-site `fetch` from `<SHOP_URL>`/`<ADMIN_URL>` with
`credentials: 'include'` the way `Lax` behaves for top-level navigations.
Login can appear to succeed and then every reload is logged out.
Two options:

- **Recommended:** put the frontends behind the same *site* as the API —
  e.g. a Render custom domain `<api>.<yourdomain>` plus Vercel custom
  domains `<shop>.<yourdomain>` / `<admin>.<yourdomain>`. Same registrable
  domain = same site, `SameSite=Lax` works, no backend change.
- Otherwise you need `SameSite=None; Secure` (a backend change in
  `refresh_cookie.go`) **and** the anon cookie to match. Do not ship the
  login flow before deciding this.

**[cross-site] on-demand rendering.** `app/(shop)/page.tsx` is
`revalidate = 0` and `app/(shop)/products/page.tsx` is `force-dynamic`, so no
build-time fetch is needed for them. `products/[slug]/page.tsx` is
`revalidate = 60` and has no `generateStaticParams`, so it is also rendered
on demand. What *does* run at build time is `app/sitemap.ts`, which fetches
`/api/v1/products?limit=50` and returns `[]` if the API is unreachable — a
Vercel build against a sleeping or not-yet-deployed `api` produces an empty
sitemap rather than a failed build. Deploy `api` first, then redeploy the
storefront.

### 4.2 Dashboard env

```env
VITE_API_BASE_URL=<EDGE_URL>
VITE_API_KEY=
```

`VITE_API_KEY` must stay empty; the dashboard bundle is public.

---

## 5. Behaviours that will look like bugs on the new topology

Check each one after the first deploy. Every item is verified against the
current code, not guessed.

1. **Public `/api/v1/products` needs no API key.** Confirmed against the
   catalog binary: 200 with and without `X-API-KEY`. So a storefront with an
   empty `NEXT_PUBLIC_API_KEY` still browses products.
2. **Rate limiting keys on the client IP.** `api` trusts
   `127.0.0.1/::1/10.0.0.0/8/172.16.0.0/12/192.168.0.0/16` as proxies. Render
   terminates TLS on its edge and forwards from a private address, so this
   generally lands right; if login rate limits fire across unrelated users,
   the trusted-proxy list is why.
3. **`api` must not be reachable in a way that skips the edge.** Public
   `/api/v1/admin/*` routes on catalog were reachable with a bare
   `X-API-KEY: x` during verification — the gateway `admin` group is what
   enforces the JWT. Keep `/api/v1/admin/*` on the gateway (`<EDGE_URL>`),
   never directly on `goshop-catalog`'s URL, even though the latter is a
   private service.
4. **`uploads` redirect.** `api` redirects `/uploads/*` to
   `S3_PUBLIC_BASE_URL`. With R2 that is the `pub-xxxx.r2.dev` bucket host, so
   `next.config.ts`'s rewrite only ever matters for rows written before the
   switch.
5. **Analytics.** The empty-dashboard problem from the local run was
   "no orders", not code. A fresh Render database has zero orders, so every
   analytics screen will be empty until data exists. Do not debug it before
   seeding.
6. **Client-side autocomplete** fetches `${NEXT_PUBLIC_MEILISEARCH_URL}
   /indexes/products/search` straight from the browser with
   `Authorization: Bearer <NEXT_PUBLIC_MEILISEARCH_SEARCH_KEY>`. Whatever key
   you put there is **public**. It must be a Meilisearch *search-only* key
   with read access to `products`, not the master key. The master key only
   goes in `MEILISEARCH_MASTER_KEY` / `INTERNAL_MEILISEARCH_URL` side.
7. **`/api/v1/ws`** is used by the storefront chat
   (`API_BASE_URL.replace(/^http/, 'ws')` → `<EDGE_URL>/api/v1/ws`). Render
   supports WebSockets on web services; no config needed, but expect the
   first connection after idle to be slow (cold start).
8. **Discord alerts are optional.** Every notifier is a no-op when
   `DISCORD_WEBHOOK_URL` is empty; nothing fatals.
9. **`media` has no database** (no `shopping_media` DB exists locally
   either). Do not point it at one.

---

## 6. Smoke test, in this order

Run each line and stop at the first failure.

```powershell
# 1. the gateway is up and answers public reads
curl "$env:EDGE_URL/healthz"
curl "$env:EDGE_URL/api/v1/products?limit=1"

# 2. identity is reachable through the gateway (401 is the expected answer)
curl -i "$env:EDGE_URL/api/v1/users/me"

# 3. login works and sets a cookie (expect 200 + Set-Cookie)
curl -i -X POST "$env:EDGE_URL/api/v1/auth/login" `
  -H "Content-Type: application/json" -H "X-Client-App: dashboard" `
  -d '{"email":"<seed admin>","password":"<password>"}'

# 4. the storefront renders and points at the right API
curl -s "$env:SHOP_URL" | Select-String "api"

# 5. the dashboard loads under its base path
curl -o NUL -w "%{http_code}`n" "$env:ADMIN_URL/admin/"
```

Expected: `1` → `/healthz` is an nginx-only location, so on Render expect
`{"status":"ok"}` from `api`; `2` → 200 with a product list; `3` → 401
`UNAUTHORIZED`; `4` → 200 + `Set-Cookie`; `5` → 200 containing the edge URL;
`6` → 200.

---

## 7. Data: seed and the two extension blockers

Empty databases, empty analytics, no admin user. Seed deliberately:

1. Migrations run automatically on each service's first boot. Verify with
   `SELECT * FROM schema_migrations_<service>;` in the service's DB.
2. Seeding: `scripts/seed.sql` uses `\connect` and `dblink` across
   databases. `psql` can do that on Render's PSQL prompt if the extensions
   exist; the Go seeder (`go run ./gomod/cmd/seed`) parses the same file but
   needs a reachable DSN for every database.
3. The seed creates the admin login used by the smoke test. Do it before
   step 6.

Extension blockers to resolve first:

- **`shopping_recs` needs `vector` and `postgres_fdw`.** Many managed
  Postgres plans allow `vector` (install it from the dashboard's extensions
  list) but not `postgres_fdw`. Without `vector` the recommender's
  migrations fail; without the FDW the affinity joins fail. If Render's
  instance refuses, the fallback is a dedicated Postgres for `shopping_recs`
  on a provider that supports both (Neon, Supabase), with `DB_HOST` pointed
  at it only for that service.
- **`dblink`** is used by `shopping_inventory`, `shopping_order`,
  `shopping_payment`, `shopping_analytics` — all created by the seed, not by
  migrations, so a missing `dblink` surfaces as "function dblink does not
  exist" mid-seed.

---

## What to add later, not now

- A `render.yaml` blueprint (13 private services + 1 public + 4 resources).
  Worth writing once the env matrix in step 3 is proven by hand; writing it
  first means debugging YAML and the topology at the same time.
- CI → GHCR images, so Render stops rebuilding Go from source on every
  deploy.
- `SameSite=None` only if you refuse custom domains. Prefer the shared
  registrable domain.
