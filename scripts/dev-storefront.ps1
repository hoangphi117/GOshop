#!/usr/bin/env pwsh
# Start the storefront dev server on one origin with the API.
#
# Why a wrapper and not plain `next dev`: the session cookie is HttpOnly with
# SameSite=Lax on Path=/api/v1/auth (backend/.../refresh_cookie.go). The browser
# only sends it back when the page and the API share an origin, so the page must
# be served from the nginx edge (:3001 -> next dev on :3002), not from next dev
# directly. Calling next dev on :3001 puts the API on another port again and
# every restore silently 400s with "Refresh token is required".
#
#   pnpm dev:storefront          -> edge on :3001, next dev on :3002
#
# Ctrl+C stops both.
param(
    [int]$EdgePort = 3001,
    [int]$DevPort = 3002
)

$ErrorActionPreference = 'Stop'
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$storefront = Join-Path $here '..\frontend\apps\storefront'
$repo = Join-Path $here '..'

$edgeUp = $false
try {
    # Only the TCP hop matters: a 4xx/5xx from the api still proves the edge is
    # listening. Testing a JSON endpoint for 200 flagged a healthy edge as down
    # whenever the catalog answered 500 (Supabase connection slots), which is the
    # very case this script is meant to unblock.
    $client = [System.Net.Sockets.TcpClient]::new()
    $edgeUp = $client.ConnectAsync('localhost', $EdgePort).Wait(2000)
    $client.Dispose()
} catch {
    $edgeUp = $false
}

if (-not $edgeUp) {
    Write-Host "Edge on :$EdgePort is not answering. Start it once with:" -ForegroundColor Yellow
    Write-Host "  docker compose -f docker-compose.dev.yml --env-file backend/.env up -d nginx" -ForegroundColor Yellow
    Write-Host "Then run this again." -ForegroundColor Yellow
    exit 1
}

Write-Host "Storefront -> http://localhost:$EdgePort  (next dev on :$DevPort, behind the edge)" -ForegroundColor Green

Push-Location $storefront
try {
    pnpm exec next dev --port $DevPort
} finally {
    Pop-Location
}
