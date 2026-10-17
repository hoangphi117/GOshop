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

Write-Host ">>> [2/3] Building Go Binaries (Sequential)..." -ForegroundColor Yellow
$binDir = Join-Path $PSScriptRoot "..\backend\bin"
if (!(Test-Path $binDir)) { New-Item -ItemType Directory -Path $binDir -Force | Out-Null }

$services = @("api", "identity", "notification", "payment", "worker", "coupon", "inventory", "catalog", "cart")
$backendDir = Join-Path $PSScriptRoot "..\backend"
Push-Location $backendDir
try {
    foreach ($svc in $services) {
        Write-Host "Compiling $svc..." -ForegroundColor DarkGray
        $out = if ($IsWindows -or $env:OS -match "Windows") { "bin/$svc.exe" } else { "bin/$svc" }
        & go build -o $out "./cmd/$svc"
        if ($LASTEXITCODE -ne 0) {
            Write-Host "Build failed for $svc!" -ForegroundColor Red
            exit 1
        }
    }
} finally {
    Pop-Location
}

Write-Host ">>> [3/3] Starting Go Services on Host" -ForegroundColor Green
$exe = if ($IsWindows -or $env:OS -match "Windows") { ".exe" } else { "" }
$binPrefix = if ($IsWindows -or $env:OS -match "Windows") { "bin\" } else { "./bin/" }
npx -y concurrently `
  -n "api,identity,notif,payment,worker,coupon,inventory,catalog,cart" `
  -c "cyan,green,magenta,blue,yellow,pink,red,gray,white" `
  --kill-others-on-fail `
  "cd backend && ${binPrefix}api$exe" `
  "cd backend && ${binPrefix}identity$exe" `
  "cd backend && ${binPrefix}notification$exe" `
  "cd backend && ${binPrefix}payment$exe" `
  "cd backend && ${binPrefix}worker$exe" `
  "cd backend && ${binPrefix}coupon$exe" `
  "cd backend && ${binPrefix}inventory$exe" `
  "cd backend && ${binPrefix}catalog$exe" `
  "cd backend && ${binPrefix}cart$exe"

