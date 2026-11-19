# GOshop on a VPS (GOLD 1: 1 vCPU, 2 GB, 30 GB SSD)

One container runs the whole Go backend. Supabase, Render Key Value, Redpanda
Cloud and Meilisearch Cloud stay managed: this host runs no database, no broker
and no search engine.

RAM budget: backend ~700–1000 MB, Caddy ~30 MB, OS + Docker ~250 MB. Fits 2 GB
with room for a spike.

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
| Redis | `REDIS_ADDR`, `REDIS_PASSWORD` (Render Key Value: `host:port`, TLS on) |
| Kafka | `KAFKA_BROKERS`, `KAFKA_SASL_*`, `KAFKA_SASL_MECHANISM=SCRAM-SHA-256`, `KAFKA_TLS_ENABLED=true` |
| Search | `MEILISEARCH_HOST`, `MEILISEARCH_MASTER_KEY` |
| Auth | `JWT_PRIVATE_KEY`, `JWT_PUBLIC_KEY`, `INTERNAL_API_SECRET`, `INTERNAL_API_TOKEN`, `API_KEY` |
| App | `APP_ENV=production`, `APP_ORIGIN`, `APP_URL`, `BASE_URL` |

Two rules that have bitten this stack before:

- `KAFKA_TLS_CA_FILE` must stay **unset** against Redpanda Cloud.
- `KAFKA_SASL_MECHANISM` must be `SCRAM-SHA-256`, not 512.

The JWT keypair is bind-mounted by the image build from
`backend/deploy/render/jwt_public.pem`, which is tracked. The **private** key is
gitignored and arrives only through `JWT_PRIVATE_KEY`. If it is missing, every
authenticated request 401s because `pkg/auth` mints an ephemeral key.

## 4. Build and start (~10 min on 1 vCPU)

```bash
cd ~/goshop/backend
docker compose -f deploy/vps/docker-compose.vps.yml up -d --build
docker compose -f deploy/vps/docker-compose.vps.yml logs -f backend
```

The first build compiles 17 binaries. On one vCPU that takes a while; later
builds reuse the Go build cache inside BuildKit only if `--build` keeps the
cache, so expect ~15 min clean and ~2 min after a code change.

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

The contract source of truth is Supabase; this VM holds only code and Caddy
state. Nothing here needs a backup except the Caddy ACME account
(`caddy-data` volume), which re-issues itself if lost.

What *does* need a backup is `backend/.env`. Copy it somewhere safe now:

```bash
scp deploy@<VM_IP>:/home/deploy/goshop/backend/.env ./goshop-vps.env
```

## 7. Redeploy after a code change (~2 min)

```bash
cd ~/goshop && git pull
cd backend
docker compose -f deploy/vps/docker-compose.vps.yml up -d --build backend
```

`--build backend` rebuilds only the app; Caddy stays up, so there is no TLS gap.

## What this layout does not have

- **The embedder.** `recommendations` falls back to best-sellers and rating
  order. Restoring semantic shelves needs a 1.5 GB torch container, i.e. a
  bigger VM.
- **A second gateway replica.** One `api` process. A crash costs ~10 s of
  downtime while Docker restarts it.
- **Prometheus / alertmanager / promtail.** The prod compose stack has them;
  here, use `docker compose logs` or point Render's log stream away.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `remaining connection slots are reserved` | migrations ran on all 17 services at once | set `RUN_MIGRATIONS=false` after the first successful boot |
| `tls error: server refused TLS connection` | `DB_SSL_MODE` not `require` | set it; Supabase refuses plaintext |
| every authenticated request 401s | `JWT_PRIVATE_KEY` missing | paste the PEM with literal `\n` escapes |
| `Unknown Topic Or Partition` on publish | Kafka topics missing | `docker exec goshop-backend /app/bin/kafkatopics` |
| container restarts every ~60 s | OOM | `docker stats`, then raise the VM or set `GOMEMLIMIT` lower |
