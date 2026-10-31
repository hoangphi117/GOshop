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
| **Email**           | Mailtrap sandbox                       | Mailtrap sandbox                                   |
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

## Secrets and Key Rotation

All `CHANGE_ME` values in `backend/.env.*.example` represent your own secrets. Example files should always contain empty values or placeholders.

If a secret is accidentally committed:

1. Removing it from the current file is **not enough** — the secret remains in Git history.
2. Revoke or rotate the secret with the corresponding provider (Cloudflare, GHN, Discord, Mailtrap).
3. Optionally, use `git filter-repo` to permanently remove the secret from Git history. Be careful: rewriting history requires a force push, and all contributors will need to re-clone the repository.
