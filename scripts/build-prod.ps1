#!/usr/bin/env pwsh
# Build prod images one at a time.
#
# Why sequential: running every `go build` stage in parallel (the default when
# passing many services to `docker compose build`) OOMs the Docker Desktop VM
# and kills the engine mid-build. Each image reuses the shared golang layer
# cache, so serial builds cost little after the first.

$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

# Compose requires these to interpolate the prod file; the values never reach
# an image (the Dockerfiles no longer copy .env).
$env:INTERNAL_API_SECRET = 'build-dummy'
$env:JWT_PRIVATE_KEY = 'build-dummy'

$services = @(
    'api1', 'api2', 'identity', 'notification', 'payment', 'catalog',
    'inventory', 'coupon', 'cart', 'banner', 'review', 'chat', 'media',
    'analytics', 'order', 'worker', 'ws-gateway'
)

$failed = @()
foreach ($svc in $services) {
    Write-Host ">>> building $svc" -ForegroundColor Cyan
    docker compose -f docker-compose.prod.yml build --progress quiet $svc
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
