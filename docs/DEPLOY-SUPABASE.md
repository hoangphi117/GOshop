# Supabase as the Postgres for the Render deploy

Short answer: **yes, and it is the better choice** than Render Postgres — but the
shape of the Supabase project matters, and one migration will fail unless you
change a setting first.

Reference: `docs/DEPLOY-DOMAIN-RENDER-VERCEL.md` §4.1 (this replaces its
"Neon or Supabase" line with Supabase-specific instructions). To run the **local
stack** against Supabase first, see `docs/DEPLOY-SUPABASE-LOCAL.md`.

---

## 1. Why Supabase works where Render Postgres is risky

| Extension | Used by | Supabase | Render Postgres |
| --- | --- | --- | --- |
| `pgcrypto` | `identity` migration 000001 | yes | yes |
| `unaccent` | `identity`, `order`, `review` | yes | yes |
| `vector` | `recommendations` migration 000001:81 | yes, **needs a dashboard toggle** | plan-dependent |
| `postgres_fdw` | `recommendations` migration 000001:49 | yes | usually refused |

`postgres_fdw` is the decisive one: `recommendations` creates foreign tables at
migration time (`000001_schema.up.sql:55-66`) and only *repoints* the server at
boot (`app/app.go:335`). If the extension is missing, the migration fails and the
service never starts. Supabase ships it.

---

## 2. Create the project

1. Supabase → New project. Pick the region closest to your Render region
   (Oregon/Frankfurt), because every query crosses the public internet.
2. Project Settings → Database → **Connection string** → URI. Note two hosts:
   - `db.<ref>.supabase.co:5432` — direct, **IPv4 is not available on the free
     plan**. Render cannot reach it.
   - `<region>.pooler.supabase.com:6543` — Supavisor, **transaction mode**.
     Use this one. Port `5432` on the same pooler host is session mode and
     exhausts connections at 17 services.
3. Project Settings → Database → Connection pooling → **Transaction mode**,
   and read the note: transaction mode does **not** support server-side
   prepared statements unless the client disables them.

> `pgx` (used by every service) defaults to the extended protocol with prepared
> statements. Against the transaction pooler that produces
> `prepared statement "..." already exists` under concurrency. `pgxpool` handles
> this if the DSN sets `default_query_exec_mode=simple_protocol`. Options that
> need no code change are listed in §4.

---

## 3. The 13 databases

`backend/.env` and every service assume **one database per domain**, all on the
same server (each compose container hardcodes its own `DB_NAME`,
`docker-compose.dev.yml:125`). Supabase gives you one database named `postgres`.

Do not create them by hand from a guess: `docs/DEPLOY-SUPABASE-LOCAL.md` §2 has
three working methods. One caveat matters here — the Supabase SQL editor wraps a
script in a single transaction, so a batched `CREATE DATABASE ... \gexec` fails
there. Use psql with `scripts/init_supabase.psql.sql` (creates all 13 **and**
enables the extensions), or one statement per editor run.

The extension list is in `scripts/init_supabase.sql`, still worth having as the
per-database reference:

```sql
-- identity:  pgcrypto, unaccent
-- order / review: unaccent
-- recs:      vector, postgres_fdw   <- the only pair that blocks a boot
```

### Credentials

Do **not** use the `postgres` superuser. The migrations run `CREATE EXTENSION`,
which on Supabase requires a role that can; the `postgres` role already exists in
every database you created. Set `DB_USER=postgres` and the database password —
but know that this is the highest-privilege role, so treat `DB_PASSWORD` as a
production secret (and never reuse the dev value).

---

## 4. Pooler compatibility (pick one)

Option A — **keep the pooler, fix the client** (no code change, one DSN flag):

```
DB_HOST=<region>.pooler.supabase.com
DB_PORT=6543
DB_SSL_MODE=require
```

Then in Render's env, per service that opens a pool, add:

```env
PGX_DEFAULT_QUERY_EXEC_MODE=simple_protocol
```

`pgx` reads that env var. Simple protocol trades prepared-statement caching for
pooler compatibility; at this scale that is free.

Option B — **use session mode** (`:5432` on the pooler host). Prepared
statements work unchanged, but Supabase's free tier caps connections, and 17 Go
services with `pgxpool` defaults (`MaxConns` per service) will exhaust it on a
cold start. Only use this if you also set a small `MaxConns`.

Option C — **Supabase's direct host with the IPv4 add-on** (paid). Cleanest
semantics, costs money.

Recommendation: **A**.

---

## 5. Render env, Supabase edition

Replaces §4.2 of the deploy doc:

```env
APP_ENV=production
APP_ORIGIN=https://goshop.dpdns.org

DB_HOST=aws-0-<region>.pooler.supabase.com
DB_PORT=6543
# Pooler user is postgres.<project-ref>, NOT plain "postgres" — Supavisor uses
# the suffix to pick the tenant project. A bare "postgres" authenticates
# against nothing and fails at connect time.
DB_USER=postgres.<project-ref>
DB_PASSWORD=<supabase db password>
DB_SSL_MODE=require

# No PGX_DEFAULT_QUERY_EXEC_MODE needed. shared/pkg/db/pool.go forces
# QueryExecModeCacheDescribe on the pool config, which is what the transaction
# pooler needs. Do NOT set simple_protocol/exec: QueryExecModeExec cannot encode
# []uuid.UUID ("cannot find encode plan"), which breaks cart and order queries.

# Migrations run through a separate DSN that needs the SESSION pooler (:5432),
# because transaction mode resets search_path to pg_catalog between statements
# and every migration runs `SET search_path`. The seeder uses these too: its
# dblink() calls connect with this host/port pair.
DB_DIRECT_HOST=aws-0-<region>.pooler.supabase.com
DB_DIRECT_PORT=5432

# recommendations points its postgres_fdw server at shopping_order, on the
# same host. Both must be set or best-sellers silently fall back to ratings
# (app.go:269 logs a warning, it does not fail).
ORDER_DB_HOST=aws-0-<region>.pooler.supabase.com
ORDER_DB_PORT=6543
ORDER_DB_USER=postgres.<project-ref>
ORDER_DB_NAME=shopping_order

REDIS_ADDR=<upstash host>:6379
REDIS_PASSWORD=<upstash password>
KAFKA_BROKERS=<broker>:9092
API_KEY=<fresh 32+ chars>
INTERNAL_API_SECRET=<fresh>
INTERNAL_API_TOKEN=<fresh>
MEILISEARCH_HOST=http://127.0.0.1:7700
JWT_SIGNING_ENABLED=true
JWT_PRIVATE_KEY=<inline PEM>
JWT_PUBLIC_KEY=<inline PEM>
STORAGE_DRIVER=r2
S3_ENDPOINT=...
S3_ACCESS_KEY=...
S3_SECRET_KEY=...
S3_PUBLIC_BASE_URL=...
```

> `postgres_fdw` over a **pooled** connection is the one combination to watch:
> the FDW opens its own libpq connection, outside `pgx`, so it does not get
> `simple_protocol`. If best-sellers stay empty while the logs show
> `Order database link not configured`, point `ORDER_DB_HOST` /
> `ORDER_DB_PORT` at the pooler's **session** port (`5432`) instead — the FDW
> then behaves normally, and it only holds one connection.

---

## 6. Seed

`scripts/seed.sql` is sectioned by `-- DB: <name>` markers, and
`cmd/seed/main.go:208` connects to each database separately, so one DSN suffices:

```powershell
cd backend
go run ./gomod/cmd/seed
# or from the repo root: make seed
```

It reads `DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_SSL_MODE` straight from the
environment, so run it with the same values as Render (locally, from
`backend/.env.prod`). It creates the admin login the smoke test uses.

---

## 7. What still does not work on Supabase

1. **`shopping_recs` vector index build** is slow on a small instance; the first
   `recommendations` boot may take a minute while it writes embeddings. Not an
   error, just a slow first start.
2. **Supabase pauses free projects after 7 days of inactivity.** A paused
   database means the Render service boots and every query fails. Keep it warm
   or pick the paid tier.
3. **`dblink` is not used anywhere in the codebase** (`grep` over
   `backend/**/*.sql` and `*.go` finds zero callers) despite what the earlier
   deploy doc said. No action needed.4. **Point-in-time recovery** is a paid feature on both Render and Supabase.
   Whatever you pick, take a manual dump before running the seed against
   production.
