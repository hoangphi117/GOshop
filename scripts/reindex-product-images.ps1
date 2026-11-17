#!/usr/bin/env pwsh
# Rewrites every product image URL in the catalog DB to its edge-relative form,
# then republishes all products so Meilisearch stores the same shape.
#
# Run once whenever the stored media origin changes (S3_PUBLIC_BASE_URL moved
# from http://localhost:9000 to the edge, a new domain, a CDN base...). The API
# already normalizes on read, so this is about cleaning the data, not fixing a
# rendering bug; skip it and serving still works.
#
#   ./scripts/reindex-product-images.ps1 -AdminToken <jwt>
#
# The reindex goes through the catalog admin update route (a no-op PUT per
# product): that is the documented publish path, so it re-emits the outbox
# event, re-syncs the search document and re-embeds the vector. Bulk UPDATEs
# would leave the index stale.
param(
    [Parameter(Mandatory = $true)][string]$AdminToken,
    [string]$CatalogUrl = 'http://localhost:8086',
    [string]$ApiKey = 'abcd-1234-1111',
    [string]$DbDsn = 'postgres://postgres:super%40password@localhost:5432/shopping_catalog',
    [int]$Throttle = 8
)

$ErrorActionPreference = 'Stop'

# 1. Data: let the shared helper decide the canonical form.
$env:CATALOG_DB_DSN = $DbDsn
if (-not $env:S3_PUBLIC_BASE_URL) { $env:S3_PUBLIC_BASE_URL = 'http://localhost/goshop-media' }
& "$PSScriptRoot\..\backend\bin\reindex-images.exe"
if ($LASTEXITCODE -ne 0) { throw 'image URL rewrite failed' }

# 2. Index: republish through the admin update route.
$ids = @(docker exec goshop-postgres psql -U postgres -d shopping_catalog -t -A `
        -c 'SELECT product_uuid FROM products WHERE product_deleted_at IS NULL' |
        Where-Object { $_ })
Write-Host "republishing $($ids.Count) products (throttle $Throttle)"

# Windows PowerShell 5.1 has no ForEach-Object -Parallel, and spawning one
# curl.exe per product took ~40 minutes. One HttpClient with a bounded number
# of in-flight requests does the same work in a few minutes.
Add-Type -AssemblyName System.Net.Http
$client = New-Object System.Net.Http.HttpClient
$client.Timeout = [TimeSpan]::FromSeconds(30)
$client.DefaultRequestHeaders.Authorization =
    New-Object System.Net.Http.Headers.AuthenticationHeaderValue('Bearer', $AdminToken)
$null = $client.DefaultRequestHeaders.TryAddWithoutValidation('X-API-KEY', $ApiKey)

$pending = New-Object System.Collections.ArrayList
$failed = 0
$done = 0

foreach ($id in $ids) {
    while ($pending.Count -ge $Throttle) {
        $finished = [System.Threading.Tasks.Task]::WaitAny($pending.ToArray())
        $task = $pending[$finished]
        $pending.RemoveAt($finished)
        if ($task.Result.IsSuccessStatusCode) { $done++ } else {
            $failed++
            Write-Warning "HTTP $([int]$task.Result.StatusCode)"
        }
        $task.Result.Dispose()
    }

    $body = '{' + '"uuid":"' + $id + '"' + '}'
    $content = New-Object System.Net.Http.StringContent($body, [Text.Encoding]::UTF8, 'application/json')
    $pending.Add($client.PutAsync("$CatalogUrl/api/v1/admin/products/$id", $content)) | Out-Null
}

foreach ($task in $pending) {
    if ($task.Result.IsSuccessStatusCode) { $done++ } else { $failed++ }
    $task.Result.Dispose()
}
$client.Dispose()

Write-Host "done: $done ok, $failed failed"
if ($failed -gt 0) { exit 1 }
