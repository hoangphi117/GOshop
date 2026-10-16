# =============================================================================
# GOshop Single-Command Dev Orchestrator (Lightweight Hybrid Mode)
# =============================================================================
# 1. Starts Docker Infrastructure (Postgres, Redis, RabbitMQ, Meilisearch)
# 2. Runs all 4 Go services concurrently on Host with colored prefix logs
# =============================================================================

Write-Host ">>> [1/2] Starting Infrastructure (Docker)..." -ForegroundColor Cyan
docker compose -f docker-compose.noapp.yml --env-file ./backend/.env up -d

if ($LASTEXITCODE -ne 0) {
    Write-Host "Failed to start infrastructure containers!" -ForegroundColor Red
    exit 1
}

Write-Host ">>> [2/2] Starting Go Services on Host (api, identity, notification, payment, worker)..." -ForegroundColor Green
npx -y concurrently `
  -n "api,identity,notif,payment,worker" `
  -c "cyan,green,magenta,blue,yellow" `
  --kill-others-on-fail `
  "cd backend && go run ./cmd/api" `
  "cd backend && go run ./cmd/identity" `
  "cd backend && go run ./cmd/notification" `
  "cd backend && go run ./cmd/payment" `
  "cd backend && go run ./cmd/worker"
