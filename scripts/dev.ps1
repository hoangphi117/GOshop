# =============================================================================
# GOshop Single-Command Dev Orchestrator (Lightweight Hybrid Mode)
# =============================================================================
# 1. Docker Compose starts infrastructure (Postgres, Redis, Kafka, MinIO,
#    Meilisearch) plus the storage-heavy domain services (inventory, catalog,
#    coupon, cart, banner).
# 2. The gateway and the remaining services build and run on the host for fast
#    rebuild loops: api, identity, notification, payment, worker, order,
#    review, analytics, chat, media, ws-gateway.
# =============================================================================

Write-Host ">>> [1/3] Starting Infrastructure and Domain Services (Docker)..." -ForegroundColor Cyan

# Static check first: it needs no running container, and it catches the class of
# bug where the gateway proxies to a service the dev compose never starts
# (502 at request time, no build-time symptom).
& python "$PSScriptRoot\check-compose-parity.py"
if ($LASTEXITCODE -ne 0) {
    Write-Host "Compose parity check failed (see above). Fix before starting." -ForegroundColor Red
    exit 1
}

# Services that must run on the host: remove any container holding their name or
# their host ports, regardless of which compose project started it. `compose stop`
# only works for services of the current project and silently skips strays.
$hostOwnedContainers = @(
    "goshop-api", "goshop-identity", "goshop-notification", "goshop-order", "goshop-payment",
    "goshop-worker", "goshop-ws-gateway", "goshop-review", "goshop-analytics", "goshop-chat", "goshop-nginx",
    "goshop-media"
)
foreach ($container in $hostOwnedContainers) {
    docker rm -f $container 2>$null | Out-Null
}

# Rebuild domain images so container services pick up fresh host code.
# Sequential (not parallel) to avoid OOM during go build on low-RAM hosts.
foreach ($svc in @("inventory", "catalog", "coupon", "cart", "banner")) {
    Write-Host "Building image $svc..." -ForegroundColor DarkGray
    docker compose -f docker-compose.dev.yml --env-file ./backend/.env build $svc
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Build failed for image $svc!" -ForegroundColor Red
        exit 1
    }
}

docker compose -f docker-compose.dev.yml --env-file ./backend/.env up -d --force-recreate minio jaeger postgres redis kafka kafka-init inventory catalog coupon cart banner

if ($LASTEXITCODE -ne 0) {
    Write-Host "Failed to start infrastructure containers!" -ForegroundColor Red
    exit 1
}

# Preflight: wait for infra to be healthy before building/running host
# services. Without this, host services boot while Kafka/Postgres is still
# starting and exit immediately, triggering kill-others-on-fail.
$infraDeadline = (Get-Date).AddSeconds(90)
do {
    $states = docker inspect goshop-postgres goshop-redis goshop-kafka --format "{{.Name}} {{.State.Health.Status}}" 2>$null
    $unhealthy = @($states | Where-Object { $_ -notmatch "healthy" })
    if ($unhealthy.Count -eq 0) { break }
    Start-Sleep -Seconds 3
} while ((Get-Date) -lt $infraDeadline)

if ($unhealthy.Count -gt 0) {
    Write-Host "Infra not healthy after 90s:" -ForegroundColor Red
    $unhealthy | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
    Write-Host "Check: docker logs goshop-kafka --tail 20" -ForegroundColor Red
    exit 1
}
Write-Host "Infra healthy: postgres, redis, kafka." -ForegroundColor Green

Write-Host ">>> [2/3] Building Go Binaries (Sequential)..." -ForegroundColor Yellow
$binDir = Join-Path $PSScriptRoot "..\backend\bin"
if (!(Test-Path $binDir)) { New-Item -ItemType Directory -Path $binDir -Force | Out-Null }

$services = @("api", "identity", "notification", "payment", "worker", "coupon", "inventory", "catalog", "cart", "order", "review", "analytics", "chat", "media", "ws-gateway", "assistant")
$hostServices = @("api", "identity", "notification", "payment", "worker", "order", "review", "analytics", "chat", "media", "ws-gateway", "assistant")
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

$requiredPorts = @(8080, 8081, 8082, 8083, 8088, 8091, 8092, 8093, 8094, 8097, 8085, 50057, 50058)
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

Push-Location "$backendDir/gomod/cmd"
try {
    foreach ($svc in $hostServices) {
        Write-Host "Compiling $svc..." -ForegroundColor DarkGray
        $out = if ($IsWindows -or $env:OS -match "Windows") { "bin/$svc.exe" } else { "bin/$svc" }
        # Binaries land in backend/bin; gomod/cmd is two levels below it.
        & go build -o "../../$out" "./$svc"
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
# Review runs in Docker (see step 1); the host-mode API gateway reaches it on
# the published port. Unset, the gateway would fall back to 404 on reviews.
if (-not $env:REVIEW_SERVICE_URL) {
    $env:REVIEW_SERVICE_URL = "http://localhost:8091"
}

Write-Host ">>> Smoke testing gateway routes..." -ForegroundColor Cyan
# dev-host has no edge nginx (it is in hostOwnedContainers above), so every
# gateway-proxied route is probed directly on :8080 rather than through :80.
# Bounded wait: the binaries are still booting, and a 502 on the first probe
# would be a false alarm. 60s covers a cold Kafka consumer group join.
$apiPort = 8080
$smokeDeadline = (Get-Date).AddSeconds(60)
do {
    $probe = & curl.exe -s -o NUL -m 3 -w "%{http_code}" "http://localhost:$apiPort/api/v1/categories" 2>$null
    if ($probe -and $probe.Trim() -ne "000" -and $probe.Trim() -ne "502") { break }
    Start-Sleep -Seconds 3
} while ((Get-Date) -lt $smokeDeadline)

if (-not $probe -or $probe.Trim() -eq "000") {
    Write-Host "Gateway never answered on :$apiPort; skipping smoke test." -ForegroundColor Yellow
} else {
    $smokeArgs = @("-ExecutionPolicy", "Bypass", "-File", (Join-Path $PSScriptRoot "smoke-dev.ps1"),
                   "-BaseUrl", "http://localhost:$apiPort")
    # Advisory, not fatal: services keep restarting under concurrently, and a
    # missing upstream is worth printing but must not kill the dev loop.
    & powershell -NoProfile @smokeArgs
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Smoke test reported failing routes (see above). Dev stack keeps running." -ForegroundColor Yellow
    }
}

$exe = if ($IsWindows -or $env:OS -match "Windows") { ".exe" } else { "" }
$binPrefix = if ($IsWindows -or $env:OS -match "Windows") { "bin\" } else { "./bin/" }
# --kill-others would mask which service failed, so run with restart-on-fail:
# a single crash no longer kills the other 10. Max 15 concurrent OS
# processes (11 host + 4 infra-adjacent) keeps host RAM under ~6GB.
npx -y concurrently `
  -k `
  --restart-tries 3 `
  --restart-after 3000 `
  -n "api,identity,notif,payment,worker,order,review,analytics,chat,media,assistant,wsgw" `
  -c "cyan,green,magenta,blue,yellow,orange,white,gray,red,cyan,magenta,darkyellow" `
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
  "cd backend && ${binPrefix}assistant$exe" `
  "cd backend && ${binPrefix}ws-gateway$exe"
