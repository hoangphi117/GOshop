# Deploy on Render + Vercel + Cloudflare: `goshop.dpdns.org`

`docs/DEPLOY-VERCEL-RENDER.md` is the full reference (17 Go services, env matrix,
blockers). This file is the parts that are specific to running **one host with a
real domain**: DNS, which Render service actually works on Render, and the two
places where the current code does not survive a multi-host split.

Read time: 8 minutes. Doing it: 2–4 hours for the first working deploy.

---

## 1. The target shape

| Piece | Hostname | Host | Public? |
| --- | --- | --- | --- |
| Storefront (Next.js) | `goshop.dpdns.org` (apex) | Vercel | yes |
| Dashboard (Vite SPA) | `admin.goshop.dpdns.org` | Vercel | yes |
| Gateway + full backend (compose) | `api.goshop.dpdns.org` | Render **Docker** service | yes |
| Postgres / Redis / Kafka / Meilisearch / MinIO | — | Render / Upstash / R2 | private |

Keeping every browser-facing piece under the **same registrable domain** is the
reason the auth cookies work: `refresh_cookie.go` and `anon_cookie.go` both set
`SameSite=Lax` with no `Domain` attribute, which the browser only sends on
requests to the same site. `docs/DEPLOY-VERCEL-RENDER.md` §4.1 says the same
thing under "[cross-site] refresh cookies" — this file just makes the DNS match.

---

## 2. Why not "one Render service per Go binary"

`docs/DEPLOY-VERCEL-RENDER.md` §3 warning 2 is the blocker, and it is worse than
the doc implies: `catalog`, `identity`, `inventory` and the other gRPC servers
call `net.Listen` on a second port at boot (`catalog/app/app.go:138`,
`identity/app/app.go:214`). A Render private service publishes **one** port, so
those listeners are unreachable and the peers that dial them fail at startup.
Collapsing gRPC to HTTP is a code change across 9 services.

Use **one Docker service** running the compose stack instead. Render runs any
Dockerfile, so the whole `docker-compose`-equivalent topology comes up as a
single private network, exactly as it does locally. This is the shortest path to
a working public site, and it is what the rest of this file assumes.

Trade-off: one service restart restarts everything, and Render's health check
only knows about the gateway. Accept both for a portfolio deploy.

---

## 3. Cloudflare DNS (do this after Render gives you a hostname)

### 3.1 Which mode

| Hostname | Cloudflare proxy | Why |
| --- | --- | --- |
| `goshop.dpdns.org` (Vercel) | **Proxied** (orange) | fine; Vercel accepts it |
| `admin.goshop.dpdns.org` (Vercel) | **Proxied** (orange) | fine |
| `api.goshop.dpdns.org` (Render) | **DNS only** (grey) at first | Render must see the request to issue its Let's Encrypt cert and verify ownership |

Once the Render dashboard shows the certificate as **Verified**, you may flip
`api` to Proxied. Optional — grey is simpler and keeps Render's TLS end to end.

**SSL/TLS mode must be `Full (strict)`, never `Flexible`.** Flexible makes
Cloudflare talk to Render over plain HTTP while Render redirects everything to
HTTPS, which is an infinite redirect loop. Render presents a real Let's Encrypt
cert, so `Full (strict)` validates cleanly.

### 3.2 Records to add (Cloudflare → DNS → Records)

Delete any existing `A`, `AAAA` or `CNAME` record for these names first. Render
is IPv4-only; a stale `AAAA` produces `ERR_SSL_VERSION_OR_CIPHER_MISMATCH`.

| Type | Name | Content | Proxy | TTL |
| --- | --- | --- | --- | --- |
| CNAME | `goshop` | `cname.vercel-dns.com` | Proxied | Auto |
| A | `goshop` | `76.76.21.21` | Proxied | Auto |
| CNAME | `admin` | `cname.vercel-dns.com` | Proxied | Auto |
| A | `admin` | `76.76.21.21` | Proxied | Auto |
| CNAME | `api` | `<service-name>.onrender.com` | **DNS only** | 300 |

The `A` + `CNAME` pair is deliberate: `dpdns.org` is not yours, so Cloudflare
cannot CNAME-flatten at the apex and the `A` record carries the apex itself. Add
both, or use only `A` if Cloudflare warns about the CNAME at the same name.

Verify before touching either dashboard:

```powershell
nslookup goshop.dpdns.org 1.1.1.1
nslookup admin.goshop.dpdns.org 1.1.1.1
nslookup api.goshop.dpdns.org 1.1.1.1
```

---

## 4. Render: the all-in-one Docker service

Render → New → **Web Service** → Deploy from Git repo → Runtime **Docker**.

| Field | Value |
| --- | --- |
| Root Directory | *(empty — repo root)* |
| Dockerfile Path | `backend/deploy/render/Dockerfile.all` |
| Docker Build Context Directory | `backend` |
| Health Check Path | `/healthz` |
| Instance Type | Standard (1 GB will OOM with 17 services) |

### 4.1 Postgres

Use **Supabase** → `docs/DEPLOY-SUPABASE.md`. It covers the 13 databases, the
`vector` + `postgres_fdw` toggle, and the pooler/host choice.

Why not Render Postgres: `recommendations` migration 000001 creates
`postgres_fdw` foreign tables (`000001_schema.up.sql:49-66`), and Render's
instance frequently refuses that extension. `pgvector` is also a plan question
there. Supabase ships both.

### 4.2 Environment variables

Set these in Render → Environment. Everything else defaults from
`backend/.env.prod`-shaped values inside the image, so start from that file's
contents and override the host-facing ones:

```env
APP_ENV=production
APP_ORIGIN=https://goshop.dpdns.org
DB_HOST=<neon host>
DB_PORT=5432
DB_USER=<neon user>
DB_PASSWORD=<neon password>
DB_SSL_MODE=require
REDIS_ADDR=<upstash host>:6379
REDIS_PASSWORD=<upstash password>
KAFKA_BROKERS=<broker>:9092
KAFKA_SASL_USER=
KAFKA_SASL_PASSWORD=
KAFKA_TLS_ENABLED=false
API_KEY=<32+ chars, generate fresh>
INTERNAL_API_SECRET=<fresh>
INTERNAL_API_TOKEN=<fresh>
MEILI_MASTER_KEY=<16+ chars>
MEILISEARCH_HOST=http://127.0.0.1:7700
SEARCH_BASE_URL=http://127.0.0.1:8086
JWT_SIGNING_ENABLED=true
JWT_PRIVATE_KEY=<inline PEM, \n escaped>
JWT_PUBLIC_KEY=<inline PEM, \n escaped>
```

`STORAGE_DRIVER` must be `r2` or `s3` (there is no local-disk driver):
create a Cloudflare R2 bucket, then set `S3_ENDPOINT`, `S3_ACCESS_KEY`,
`S3_SECRET_KEY`, `S3_PUBLIC_BASE_URL`. R2 is free-tier friendly and lives in the
same Cloudflare account as the DNS.

### 4.3 Custom domain on Render

Settings → Custom Domains → Add → `api.goshop.dpdns.org`. Render shows the
`onrender.com` target to put in the Cloudflare CNAME (step 3.2). Click **Verify**
once DNS has propagated.

---

## 5. Vercel: two projects

Both use Root Directory `frontend` (not `apps/...`), so the workspace package
`@goshop/api-client` resolves.

| Setting | Storefront | Dashboard |
| --- | --- | --- |
| Framework Preset | Next.js | Vite |
| Install Command | `pnpm install --frozen-lockfile` | same |
| Build Command | `pnpm --filter storefront build` | `pnpm --filter dashboard build` |
| Output Directory | *(detected)* | `apps/dashboard/dist` |
| Node Version | 22.x | 22.x |

Env (Project → Settings → Environment Variables):

```env
# storefront
NEXT_PUBLIC_API_URL=https://api.goshop.dpdns.org
INTERNAL_API_URL=https://api.goshop.dpdns.org
NEXT_PUBLIC_SITE_URL=https://goshop.dpdns.org
NEXT_PUBLIC_API_KEY=
NEXT_PUBLIC_GOOGLE_CLIENT_ID=<same client id as backend GOOGLE_CLIENT_ID>

# dashboard
VITE_API_BASE_URL=https://api.goshop.dpdns.org
VITE_API_KEY=
```

Both keys stay **empty**: the bundle is public, and the gateway injects
`X-API-KEY` server-side.

Domains: storefront → `goshop.dpdns.org`; dashboard → `admin.goshop.dpdns.org`.
The dashboard lives under `/admin/` unless you also change `base` in
`vite.config.ts` and `basepath` in `src/main.tsx` (reference doc §4).

Google Cloud Console → Credentials → your OAuth client → **Authorized JavaScript
origins** must include `https://goshop.dpdns.org`, or the sign-in button throws
`origin_mismatch`.

---

## 6. Order of operations

1. Cloudflare: nameservers already delegated, nothing to do.
2. Supabase: create the 13 databases and enable `vector` on `shopping_recs` —
   `docs/DEPLOY-SUPABASE.md` §2–§3.
3. Upstash Redis + a Kafka broker. Cheapest: Upstash Kafka free tier, set
   `KAFKA_SASL_USER` / `KAFKA_SASL_PASSWORD` / `KAFKA_TLS_ENABLED=true`.
4. Cloudflare R2 bucket, public read enabled.
5. Render web service (`backend/deploy/render/Dockerfile.all`, context
   `backend`), first deploy.
6. Render → Custom Domains → `api.goshop.dpdns.org` → add the Cloudflare CNAME
   (grey cloud) → Verify.
7. Vercel: two projects, env from §5, add the two domains.
8. Cloudflare: add the `goshop` and `admin` records from §3.2.
9. Seed the database (reference doc §7), then run the smoke test (reference doc
   §6) with `EDGE_URL=https://api.goshop.dpdns.org` and
   `SHOP_URL=https://goshop.dpdns.org`.

---

## 7. Things that will still break, ranked by likelihood

1. **`pgvector`/`postgres_fdw` missing** → `recommendations` never finishes its
   migration. Fix: Supabase, both extensions enabled on `shopping_recs`
   (`docs/DEPLOY-SUPABASE.md` §3).
2. **Kafka unreachable** → the stack boots, but every consumer sits at `lag=0`
   and no email or notification appears. Check the SSR-rendered storefront, not
   the logs: `up` prints "Started" either way.
3. **Cookie 401 after login** → `APP_ORIGIN` not set, or the API is not under
   `goshop.dpdns.org`. `SameSite=Lax` needs a shared site.
4. **Empty sitemap** → deployed the storefront before the API existed. Redeploy
   it after step 6.
5. **Cold start** on the free/standard plan: the gateway answers in 30–60 s
   after idle. Expected, not a bug.

---

## 8. Next action

Create the Render Docker web service with `deploy/render/Dockerfile.all`
(step 4). Everything else in this file depends on its `onrender.com` hostname.

Skipped: `render.yaml` blueprint, CI → GHCR images, one-service-per-binary
topology. Add the blueprint when the env matrix in §4.2 stops changing.
