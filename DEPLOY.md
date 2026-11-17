# DEPLOYMENT

How to run GOshop in two modes, and the reasoning behind each choice at the current project scale.

## Two Modes

|                     | DEV                                    | PROD (Local)                                       |
| ------------------- | -------------------------------------- | -------------------------------------------------- |
| **Command**         | `make env-dev` → `make dev`            | `make env-prod` → `make prod`                      |
| **Environment**     | `backend/.env.dev`                     | `backend/.env.prod`                                |
| **Object Storage**  | MinIO container                        | **Cloudflare R2** (no MinIO container)             |
| **Backend Tracing** | Local Jaeger (`jaeger:4318`)           | Grafana Cloud Tempo                                |
| **Logging**         | stdout → Promtail → Grafana Cloud Loki | Same as DEV                                        |
| **Metrics/Alerts**  | Local Prometheus + Alertmanager → Discord | Local Prometheus + Alertmanager → Discord       |
| **Email**           | Gmail SMTP (App Password)              | Gmail SMTP (App Password)                          |
| **Storefront**      | Dev server on `:3001`                  | Next.js standalone image behind the edge           |
| **Dashboard**       | Dev server                             | Static build, with `dist/` bind-mounted into Nginx |

The single source of truth for configuration is `backend/.env`. Switch environments using `make env-dev` / `make env-prod` (which copies the selected variant to the active `.env` file). Compose files do not hardcode service endpoints beyond the container topology.

### DEV — Build and Run

```powershell
make env-dev   # Copies backend/.env.dev -> backend/.env (STORAGE_DRIVER=minio ...)
make dev       # Runs compose down + up --build; MinIO always starts
```

MinIO is configured automatically through `backend/.env.dev`:

```env
STORAGE_DRIVER=minio
S3_ENDPOINT=http://minio:9000
S3_USE_PATH_STYLE=true
S3_USE_SSL=false
```

`docker-compose.dev.yml` keeps `minio` outside any profile, so it always starts.

### PROD — Build and Run

```powershell
make env-prod  # Copies backend/.env.prod -> backend/.env (STORAGE_DRIVER=r2 ...)
make prod      # Runs build-prod.ps1 (sequential Go builds) + builds storefront + starts services with up -d + reloads Nginx
make smoke-prod
```

R2 is configured automatically through `backend/.env.prod`:

```env
STORAGE_DRIVER=r2
S3_ENDPOINT=https://<CF_ACCOUNT_ID>.r2.cloudflarestorage.com
S3_ACCESS_KEY=CHANGE_ME            # R2 API token; revoke it if exposed
S3_SECRET_KEY=CHANGE_ME
S3_USE_PATH_STYLE=false
S3_USE_SSL=true
S3_PUBLIC_BASE_URL=https://pub-<random>.r2.dev
```

**How MinIO is excluded from the production build:**

- `docker-compose.prod.yml` assigns the `profiles: ["minio"]` profile to the `minio` service. Therefore, a default `docker compose up -d` skips it, saving approximately 150 MB of RAM and one volume.
- No service in the production Compose file has `depends_on: minio`.
- `S3_ENDPOINT` is not hardcoded in Compose; the `.env` configuration with `STORAGE_DRIVER=r2` takes precedence.

To run MinIO offline in production:

```powershell
docker compose -f docker-compose.prod.yml --profile minio up -d minio
```

### PROD — Restart Without Rebuilding

```powershell
make prod-fast
```

**Note:** This command does not rebuild the storefront. After changing any `NEXT_PUBLIC_*` variables, run:

```powershell
docker compose -f docker-compose.prod.yml build storefront
```

## Considered Alternatives (and Why They Were Not Chosen)

### CI Build (GitHub Actions → GHCR) Instead of Building on the Deployment Machine

A common concern is that "building production images on a 4 GB VPS will cause an OOM." This is theoretically valid. That is why `scripts/build-prod.ps1` already performs **sequential builds**, with the reasoning documented at the beginning of the script.

At the current project scale (no VPS, no domain, and no real users), setting up a CI pipeline would introduce additional code, a PAT secret, and GHCR quota management for a problem that does not yet exist.

When deploying to a real VPS, there are two options:

1. **Add 4 GB of swap to the VPS** — This requires only a single OS configuration change and takes approximately 15 minutes to build. It is sufficient for a demo project, so this is the first approach to try.
2. **CI → GHCR** — Consider this only when deployment frequency increases or the VPS has less than 2 GB of RAM. Add `.github/workflows/build.yml` and replace `build:` with `image: ghcr.io/<user>/goshop-<svc>:tag` in the Compose file. Initial setup takes approximately one hour.

### TLS

`nginx/prod/goshop.conf` **already listens on port 443**, mounts a self-signed certificate (`scripts/gen-cert.ps1` generates `nginx/certs/local.crt` for localhost), and includes an ACME webroot mount and a `certbot` service for automatic certificate renewal.

When a real domain becomes available, only the following steps are required:

1. Point the DNS A record to the VPS.
2. Set `APP_ORIGIN=https://<domain>` in `backend/.env.prod` (all other URLs are derived from this value).
3. Run `scripts/init_letsencrypt.sh` (update `DOMAINS` and `EMAIL` first) to replace the dummy certificate with a valid Let's Encrypt certificate.

There is no need to change `VITE_API_BASE_URL`. The dashboard is served from the same origin through the edge, so leaving this variable empty enables same-origin requests. Switching from HTTP to HTTPS happens automatically based on the page's origin.

This is the main reason for adopting a **single edge origin** architecture: the frontend does not need to hardcode the origin.

## Event Contracts

The broker payloads are documented as protobuf under `backend/gomod/shared/proto/events/`. They are **schema only**: no code is generated from them and the services still marshal plain JSON structs. `buf breaking` (run in CI against `origin/develop`) fails a pull request that renames or removes a field, which is the only thing stopping a producer from drifting away from its consumers.

| Topic | Proto package | Producer |
| --- | --- | --- |
| `order.created` | `events.order.v1` | order outbox |
| `order.cancelled` | `events.order.v1` | order lifecycle |
| `order.status_changed` | `events.order.v1` | order lifecycle / shipping |
| `order_return.requested` / `.updated` / `.completed` | `events.order.v1` | order returns |
| `payment.succeeded` / `.failed` / `.expired` / `.refunded` | `events.payment.v1` | payment |
| `review.rating_updated` | `events.review.v1` | review |
| `user.created` / `.updated` / `.deleted` | `events.identity.v1` | identity |
| `media.uploaded` | `events.media.v1` | media |
| `inventory.stock_low`, `notification_events` | no schema yet | inventory / any |

Rules when editing an event proto:

1. Add fields, never rename or reuse a number. Renaming requires a new field plus `reserved` on the old one.
2. Every field carries `[json_name = "..."]`. The wire name is the snake_case JSON key, not protojson's camelCase guess.
3. Publish through `buf lint` and `buf breaking` from the repository root — running either from `backend/` fails because buf resolves `.git` by walking up from the working directory.
4. The proto describes what the **producer** sends. Where a consumer reads something else, that divergence is written in the field comment, not smoothed over.

## Observability

Prometheus scrapes every Go service, holds the alert rules, and hands firing alerts to Alertmanager, which posts them to a Discord channel. There is no separate bridge container: Alertmanager's native `discord_configs` receiver does the work.

| Piece | Where | Exposed on |
| --- | --- | --- |
| `prometheus` | `system/prometheus/prometheus.yml` + `system/prometheus/rules/goshop.yml` | `:9090` (dev only; NOT published in prod) |
| `alertmanager` | `system/alertmanager/alertmanager.yml` | `:9093` (dev only) |
| `kafka-exporter` | compose service | `:9308` |
| `goshop-*` services | `/metrics` on the service's own HTTP port (8080 in-container) | internal only |
| Grafana dashboard | `system/grafana/dashboards/goshop-overview.json` | import into Grafana Cloud |
| Logs | `system/promtail/promtail-config.yml` → Grafana Cloud Loki | — |

**What is scraped.** Docker service discovery picks up every container whose name is `goshop-<service>`, on the container's own HTTP port (8080 for every service; the host port differs per published mapping). Adding a service means adding its name to the `keep` regex in `system/prometheus/prometheus.yml` — otherwise it is silently unscraped. `worker` and `ws-gateway` are excluded because neither mounts `/metrics` on an HTTP router; `worker` instead serves `METRICS_PORT=9464` via `shared/pkg/metrics.ListenAndServe`.

**`/metrics` is internal-only.** The endpoint is unauthenticated. It listens on the compose network; it is not published to the host in prod, and nginx does not proxy it. Do not add a proxy rule for `/metrics`.

**Alerts.** Rules live in `system/prometheus/rules/goshop.yml`. Every rule carries `severity` and `service` labels; Alertmanager groups by `[alertname, service]` and repeats every 4h. `scripts/check_alert_rules.py` is a CI gate: it fails if a rule loses a required label, if `ConsumerLagHigh`'s threshold drifts from `kafka.LagAlertThreshold`, or if a rule references a metric that is not exposed (a silent alert).

**Where alerts go.** `ALERTMANAGER_DISCORD_WEBHOOK_URL` is the Discord channel webhook (Channel → Edit → Integrations → Webhooks). It is separate from `DISCORD_WEBHOOK_URL`, which feeds in-app alerts from `shared/pkg/discord`. The URL is substituted into the config at container start because Alertmanager does not expand environment variables in its own config file.

**Silencing an alert.** While an alert is active, open Alertmanager (`:9093` in dev), pick the alert, and create a silence with a matcher such as `alertname="ConsumerLagHigh"` and an expiry. Silences are stored in `alertmanager-data` and survive restarts.

**Retention.** Prometheus keeps 7 days (`--storage.tsdb.retention.time=7d`) in the `prometheus-data` volume. Alertmanager's silences and notification log live in `alertmanager-data`.

**Verification.** `scripts/verify_lag_alert.py` starts a throwaway Prometheus against the real rule file, feeds it a synthetic lag of 5000, and asserts that `ConsumerLagHigh` reaches `firing`. Run it from the repository root.

## Secrets and Key Rotation

All `CHANGE_ME` values in `backend/.env.*.example` represent your own secrets. Example files should always contain empty values or placeholders.

If a secret is accidentally committed:

1. Removing it from the current file is **not enough** — the secret remains in Git history.
2. Revoke or rotate the secret with the corresponding provider (Cloudflare, GHN, Discord, Gmail App Password).
3. Optionally, use `git filter-repo` to permanently remove the secret from Git history. Be careful: rewriting history requires a force push, and all contributors will need to re-clone the repository.
