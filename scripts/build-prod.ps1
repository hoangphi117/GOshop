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
    'analytics', 'order', 'worker', 'ws-gateway'
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
