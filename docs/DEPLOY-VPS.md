# GOshop on a VPS (GOLD 1: 1 vCPU, 2 GB, 30 GB SSD)

One container runs the whole Go backend. Supabase, Render Key Value, Redpanda
Cloud and Meilisearch Cloud stay managed: this host runs no database, no broker
and no search engine.

RAM budget: backend ~700–1000 MB, embedder ~300 MB, Caddy ~30 MB, OS + Docker
~250 MB. Fits 2 GB with a small spike margin.

## 1. Provision (Ubuntu 24.04, ~20 min)

```bash
# As the provider's default user (ubuntu on cloud images), not root.
sudo adduser --gecos "" deploy      # prompts for a password: sudo needs one
sudo usermod -aG sudo deploy

# SSH key for the deploy user, then disable password login.
sudo mkdir -p /home/deploy/.ssh && sudo chmod 700 /home/deploy/.ssh
# Paste the PUBLIC key:
sudo vi /home/deploy/.ssh/authorized_keys
sudo chmod 600 /home/deploy/.ssh/authorized_keys
sudo chown -R deploy:deploy /home/deploy/.ssh

# Verify BEFORE locking the password out: open a second session as deploy.
# If that session fails, this is the last chance to fix authorized_keys.
sudo sed -i 's/^#\?PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config
sudo sed -i 's/^#\?PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config
sudo systemctl restart ssh

# Firewall: only SSH, HTTP, HTTPS.
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw --force enable

# Swap. 2 GB of RAM with 17 Go processes needs the headroom; without it the
# kernel OOM-kills the container instead of slowing down. swappiness=10 keeps
# RAM first so the API does not thrash.
sudo fallocate -l 2G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
echo 'vm.swappiness=10' | sudo tee /etc/sysctl.d/99-swap.conf
sudo sysctl -p /etc/sysctl.d/99-swap.conf

# Docker + compose plugin.
sudo apt-get update && sudo apt-get install -y ca-certificates curl git
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] \
https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo $VERSION_CODENAME) stable" \
  | sudo tee /etc/apt/sources.list.d/docker.list
sudo apt-get update && sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
sudo usermod -aG docker deploy

# Unattended security updates.
sudo apt-get install -y unattended-upgrades && sudo dpkg-reconfigure -plow unattended-upgrades
```

The `docker` group is read at login, so `docker run` still fails inside the
current session. Close the VS Code remote connection entirely
(`Remote: Close Remote Connection`) and reconnect; `groups` must list `docker`.
If it still does not, `pkill -u deploy -f vscode-server` kills the stale server,
then reconnect. `sg docker -c "..."` is not a substitute: it prompts for the
group's password, which is not how group membership works.

## 2. Checkout (~2 min)

```bash
git clone https://github.com/hoangphi117/GOshop.git ~/goshop
cd ~/goshop
git checkout main
```

## 3. Environment (~10 min)

`backend/.env` is gitignored and must be copied from Render by hand.

```bash
cp ~/goshop/backend/.env.prod.example ~/goshop/backend/.env
chmod 600 ~/goshop/backend/.env
vi ~/goshop/backend/.env
```

Copy the values from the Render dashboard (Environment tab). Required:

| Group | Keys |
|---|---|
| Database | `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_SSL_MODE=require`, all `*_DB_NAME` |
| Redis | `REDIS_PASSWORD` only. `REDIS_ADDR` is overridden to `redis:6379` by the compose file |
| Kafka | `KAFKA_BROKERS`, `KAFKA_SASL_*`, `KAFKA_SASL_MECHANISM=SCRAM-SHA-256`, `KAFKA_TLS_ENABLED=true` |
| Search | `MEILISEARCH_HOST`, `MEILISEARCH_MASTER_KEY` |
| Auth | `JWT_PRIVATE_KEY`, `JWT_PUBLIC_KEY`, `INTERNAL_API_SECRET`, `INTERNAL_API_TOKEN`, `API_KEY` |
| App | `APP_ENV=production`, `APP_ORIGIN`, `APP_URL`, `BASE_URL` |

Two rules that have bitten this stack before:

- `KAFKA_TLS_CA_FILE` must stay **unset** against Redpanda Cloud.
- `KAFKA_SASL_MECHANISM` must be `SCRAM-SHA-256`, not 512.

Redis runs on this VM instead of Render Key Value. Render's free Key Value
refuses external traffic, and its internal hostname only resolves inside
Render's network, so a container here cannot reach it. Set a fresh password:

```bash
sed -i "s|^REDIS_PASSWORD=.*|REDIS_PASSWORD=$(openssl rand -hex 32)|" ~/goshop/backend/.env
grep '^REDIS_PASSWORD=' ~/goshop/backend/.env
```

`REDIS_TLS_ENABLED` must be **unset**: the connection is plain TCP inside the
compose network. The compose file replaces `REDIS_ADDR` with `redis:6379`, so
whatever the copied `.env` still says from Render does not matter.

The JWT keypair is bind-mounted by the image build from
`backend/deploy/render/jwt_public.pem`, which is tracked. The **private** key is
gitignored and arrives only through `JWT_PRIVATE_KEY`. If it is missing, every
authenticated request 401s because `pkg/auth` mints an ephemeral key.

## 4. First start (image from CI, ~1 min)

CI builds the image on every push to `main` and pushes it to
`ghcr.io/hoangphi117/goshop/backend:<sha>` (plus the embedder). The host only
pulls; nothing is compiled here. One-time setup:

```bash
# PAT with read:packages (packages are private even though the repo is public)
echo "$GHCR_PAT" | docker login ghcr.io -u <github-user> --password-stdin

# First boot uses the default local tag, so build once OR pull the current main:
cd ~/goshop/backend
docker login ghcr.io -u <github-user> --password-stdin < /dev/null   # idempotent
GOSHOP_BACKEND_IMAGE=ghcr.io/hoangphi117/goshop/backend:main \
GOSHOP_EMBEDDER_IMAGE=ghcr.io/hoangphi117/goshop/embedder:main \
  docker compose -f deploy/vps/docker-compose.vps.yml up -d --no-build
```

In GitHub → Settings → Secrets and variables → Actions, add the three secrets
the deploy job reads: `VPS_HOST`, `VPS_USER` (the `deploy` user), `VPS_SSH_KEY`
(its private key, `-----BEGIN OPENSSH PRIVATE KEY-----` line included).

From then on a merge to `main` deploys itself: `.github/deploy/vps.sh` records
the new sha in `backend/.env` (`GOSHOP_BACKEND_IMAGE` / `GOSHOP_EMBEDDER_IMAGE`),
pulls both images, restarts, and waits for `goshop-backend` to report healthy —
it re-points the tag at the previous image if that never happens.

```bash
# rollback by hand (any previous sha, see the Actions run or `docker images`)
sed -i 's|^GOSHOP_BACKEND_IMAGE=.*|GOSHOP_BACKEND_IMAGE=ghcr.io/hoangphi117/goshop/backend:<sha>|' ~/goshop/backend/.env
cd ~/goshop/backend && docker compose -f deploy/vps/docker-compose.vps.yml up -d --no-build backend
```

Building on the host is still possible (`up -d --build`, ~15 min on one vCPU);
it is just not part of a deploy anymore.

Look for, in order:

```
start identity on :8081
  identity listening
...
bootstrap kafka topics
kafka topics ready total=22
start api on :8080
```

Then confirm the whole stack is healthy:

```bash
curl -s localhost:8080/healthz
# {"database":"up","messaging":"up","redis":"up","storage":"skipped"}
```

`storage":"skipped"` is correct on this host: there is no local object store,
uploads go to R2.

Check Redis separately, since a wrong password there shows up as a health
failure rather than a boot failure:

```bash
docker exec goshop-redis sh -c 'redis-cli --no-auth-warning -a "$REDIS_PASSWORD" ping'
# PONG
docker exec goshop-redis redis-cli --no-auth-warning -a "$(grep -oP "(?<=^REDIS_PASSWORD=).*" ~/goshop/backend/.env)" dbsize
```

If the gateway logs `failed to dial ... lookup red-...: server misbehaving`, the
backend is still reading Render's hostname: `REDIS_ADDR` in the compose
`environment` block was lost, or the container was started from an older file.

## 5. DNS (~5 min, in Cloudflare)

`api.goshop.dpdns.org` currently points at Render. Switch it to the VM's IPv4:

| Type | Name | Content | Proxy |
|---|---|---|---|
| A | `api` | `<VM_IPV4>` | Proxied (orange) |
| AAAA | `api` | — | delete any existing record |

While proxied, Cloudflare terminates TLS and Caddy sees HTTP; that is why the
`Caddyfile` keeps a `:80` block. Once the record is DNS-only, Caddy issues its
own certificate for `api.goshop.dpdns.org`.

Verify:

```bash
curl -s https://api.goshop.dpdns.org/healthz
curl -s -o /dev/null -w '%{http_code}\n' https://api.goshop.dpdns.org/api/v1/products
```

## 6. Backups (~5 min)

The contract source of truth is Supabase; this VM holds only code, Caddy state
and the Redis append-only file. Nothing here needs a backup except the Caddy
ACME account (`caddy-data`, which re-issues itself if lost) and Redis, whose
`redis-data` volume is a cache that refills from Postgres on demand — losing it
logs users out and empties carts, nothing more.

What *does* need a backup is `backend/.env`. Copy it somewhere safe now:

```bash
scp deploy@<VM_IP>:/home/deploy/goshop/backend/.env ./goshop-vps.env
```

To snapshot Redis anyway:

```bash
docker exec goshop-redis sh -c 'redis-cli --no-auth-warning -a "$REDIS_PASSWORD" BGSAVE'
docker cp goshop-redis:/data/appendonlydir ./redis-snapshot
```

## 7. Redeploy after a code change (~2 min)

```bash
cd ~/goshop && git pull
cd backend
docker compose -f deploy/vps/docker-compose.vps.yml up -d --build backend embedder
```

`--build backend embedder` rebuilds the two app images; Caddy and Redis stay up,
so there is no TLS gap and no cache flush. Drop `embedder` when only the Go code
changed.

## What this layout does not have

- **A GPU for embeddings.** The embedder runs as the CPU ONNX sidecar
  (docker/embedder, ~300 MB), so semantic and personalized shelves work, but with
  a single thread's throughput. Moving to a GPU is a hosting change, not a code one.
- **A second gateway replica.** One `api` process. A crash costs ~10 s of
  downtime while Docker restarts it.
- **Prometheus / alertmanager / promtail.** The prod compose stack has them;
  here, use `docker compose logs` or point Render's log stream away.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `remaining connection slots are reserved` | 13 services opened a pool each, and Supabase `max_connections` is 60 | set `DB_MAX_CONNS=3` + `DB_MIN_CONNS=0`, and `SKIP_MIGRATIONS=true` after the first successful boot |
| `Dirty database version N. Fix and force version.` | a migration committed its DDL but the version row stayed dirty | verify the schema, then `UPDATE public.schema_migrations_<svc> SET version=N, dirty=false;` |
| `tls error: server refused TLS connection` | `DB_SSL_MODE` not `require` | set it; Supabase refuses plaintext |
| every authenticated request 401s | `JWT_PRIVATE_KEY` missing | paste the PEM with literal `\n` escapes |
| `Unknown Topic Or Partition` on publish | Kafka topics missing | `docker exec goshop-backend /app/bin/kafkatopics` |
| `lookup red-...: server misbehaving` | backend still reading Render Key Value's internal hostname | `REDIS_ADDR: redis:6379` must be in the compose `environment` block |
| `NOAUTH` / `WRONGPASS` in redis logs | compose interpolated `REDIS_PASSWORD` from the shell, not `.env` | run compose from `backend/` so `../../.env` resolves, or export it first |
| container restarts every ~60 s | OOM | `docker stats`, then raise the VM or set `GOMEMLIMIT` lower |
