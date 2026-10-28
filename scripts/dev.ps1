# =============================================================================
# GOshop Single-Command Dev Orchestrator (Lightweight Hybrid Mode)
# =============================================================================
# 1. Docker Compose starts infrastructure (Postgres, Redis, RabbitMQ, MinIO,
#    Meilisearch) plus the storage-heavy domain services (inventory, catalog,
#    coupon, cart, banner).
# 2. The gateway and the remaining services build and run on the host for fast
#    rebuild loops: api, identity, notification, payment, worker, order,
#    review, analytics, chat, media, ws-gateway.
# =============================================================================

Write-Host ">>> [1/3] Starting Infrastructure and Domain Services (Docker)..." -ForegroundColor Cyan

# Services that must run on the host: remove any container holding their name or
# their host ports, regardless of which compose project started it. `compose stop`
# only works for services of the current project and silently skips strays.
$hostOwnedContainers = @(
    "goshop-api", "goshop-identity", "goshop-notification", "goshop-order", "goshop-payment",
    "goshop-worker", "goshop-ws-gateway", "goshop-review", "goshop-analytics", "goshop-chat", "goshop-nginx"
)
foreach ($container in $hostOwnedContainers) {
    docker rm -f $container 2>$null | Out-Null
}

docker compose -f docker-compose.dev.yml --env-file ./backend/.env up -d minio jaeger inventory catalog coupon cart banner

if ($LASTEXITCODE -ne 0) {
    Write-Host "Failed to start infrastructure containers!" -ForegroundColor Red
    exit 1
}

Write-Host ">>> [2/3] Building Go Binaries (Sequential)..." -ForegroundColor Yellow
$binDir = Join-Path $PSScriptRoot "..\backend\bin"
if (!(Test-Path $binDir)) { New-Item -ItemType Directory -Path $binDir -Force | Out-Null }

$services = @("api", "identity", "notification", "payment", "worker", "coupon", "inventory", "catalog", "cart", "order", "review", "analytics", "chat", "media", "ws-gateway")
$hostServices = @("api", "identity", "notification", "payment", "worker", "order", "review", "analytics", "chat", "media", "ws-gateway")
$backendDir = Join-Path $PSScriptRoot "..\backend"
$serviceBinaryPaths = $services | ForEach-Object {
    [System.IO.Path]::GetFullPath((Join-Path $backendDir "bin\$_.exe"))
}
$runningServiceProcesses = Get-Process -ErrorAction SilentlyContinue | Where-Object {
    $_.Path -and $serviceBinaryPaths -contains ([System.IO.Path]::GetFullPath($_.Path))
}
foreach ($process in $runningServiceProcesses) {
    Stop-Process -Id $process.Id -Force
}

$requiredPorts = @(8080, 8081, 8082, 8083, 8088, 8091, 8092, 8093, 8094, 8085, 50057, 50058)
$deadline = (Get-Date).AddSeconds(10)
do {
    $occupiedPorts = @(
        Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue |
            Where-Object { $requiredPorts -contains $_.LocalPort }
    )
    if ($occupiedPorts.Count -eq 0) {
        break
    }
    Start-Sleep -Milliseconds 250
} while ((Get-Date) -lt $deadline)

if ($occupiedPorts.Count -gt 0) {
    $portList = ($occupiedPorts | Select-Object -ExpandProperty LocalPort -Unique) -join ", "
    Write-Host "Required host ports are still in use: $portList" -ForegroundColor Red
    Write-Host "Stop the owning process, then run make dev-host again." -ForegroundColor Red
    exit 1
}

Push-Location $backendDir
try {
    foreach ($svc in $hostServices) {
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
# Host services reach the dev collector on the published OTLP port. Without
# this they still run; tracing just stays a no-op.
if (-not $env:OTEL_EXPORTER_OTLP_ENDPOINT) {
    $env:OTEL_EXPORTER_OTLP_ENDPOINT = "http://localhost:4318"
}
$exe = if ($IsWindows -or $env:OS -match "Windows") { ".exe" } else { "" }
$binPrefix = if ($IsWindows -or $env:OS -match "Windows") { "bin\" } else { "./bin/" }
npx -y concurrently `
  -n "api,identity,notif,payment,worker,order,review,analytics,chat,media,wsgw" `
  -c "cyan,green,magenta,blue,yellow,orange,white,gray,red,cyan,darkyellow" `
  --kill-others-on-fail `
  "cd backend && ${binPrefix}api$exe" `
  "cd backend && ${binPrefix}identity$exe" `
  "cd backend && ${binPrefix}notification$exe" `
  "cd backend && ${binPrefix}payment$exe" `
  "cd backend && ${binPrefix}worker$exe" `
  "cd backend && ${binPrefix}order$exe" `
  "cd backend && ${binPrefix}review$exe" `
  "cd backend && ${binPrefix}analytics$exe" `
  "cd backend && ${binPrefix}chat$exe" `
  "cd backend && ${binPrefix}media$exe" `
  "cd backend && ${binPrefix}ws-gateway$exe"
