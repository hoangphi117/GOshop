param([string[]]$Only = @())

# Build prod images one at a time.
#
# Why sequential: building several Go services in one `docker compose build`
# runs their `go build` stages in parallel and OOMs the Docker Desktop VM
# (single instance, low RAM). Serial builds cost little after the first
# because the BuildKit cache mounts in Dockerfile.service persist the module
# and compiler caches across services.

$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

# Compose requires these to interpolate the prod file; the values never reach
# an image (the Dockerfiles do not copy .env).
if (-not $env:INTERNAL_API_SECRET) { $env:INTERNAL_API_SECRET = 'build-dummy' }
if (-not $env:JWT_PRIVATE_KEY) { $env:JWT_PRIVATE_KEY = 'build-dummy' }

$services = @(
    'api1', 'api2', 'identity', 'notification', 'payment', 'catalog',
    'inventory', 'coupon', 'cart', 'banner', 'review', 'chat', 'media',
    'analytics', 'order', 'recommendations', 'worker', 'ws-gateway',
    # Not Go services, but they are compose-managed images: leaving them out
    # makes `up --no-build` fail on "No such image".
    # embedder is the recommendations sidecar; without it the shelf silently
    # degrades to best sellers, so it is built like any other service.
    'assistant', 'storefront', 'embedder'
)

# Opt-in subset: ./scripts/build-prod.ps1 -Only api1,order
if ($Only.Count -gt 0) { $services = $services | Where-Object { $Only -contains $_ } }

$failed = @()
foreach ($svc in $services) {
    Write-Host ">>> building $svc" -ForegroundColor Cyan
    docker compose --progress plain -f docker-compose.prod.yml --env-file ./backend/.env build $svc
    if ($LASTEXITCODE -ne 0) {
        Write-Host "    FAILED: $svc" -ForegroundColor Red
        $failed += $svc
    }
}

if ($failed.Count -gt 0) {
    Write-Host "FAILED: $($failed -join ', ')" -ForegroundColor Red
    exit 1
}
Write-Host 'all prod images built' -ForegroundColor Green

# Kafka's image runs as uid 1000 but Docker creates named volumes owned by
# root, so a fresh volume crash-loops with
#   AccessDeniedException: /tmp/kafka-logs/bootstrap.checkpoint.tmp
# Chown only a volume that does not exist yet: an existing one already has the
# right owner and its data must not be touched.
if (-not (docker volume inspect shopping_kafka-data 2>$null)) {
    Write-Host '>>> preparing kafka volume ownership' -ForegroundColor Cyan
    docker volume create shopping_kafka-data | Out-Null
    docker run --rm -v shopping_kafka-data:/tmp/kafka-logs alpine `
        sh -c 'chown -R 1000:1000 /tmp/kafka-logs && chmod -R 755 /tmp/kafka-logs'
    if ($LASTEXITCODE -ne 0) {
        Write-Host 'FAILED: kafka volume ownership' -ForegroundColor Red
        exit 1
    }
}
