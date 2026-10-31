# Rotates the edge API key in one pass:
#  1. new 32-byte random key
#  2. writes nginx/secrets/api_key (mounted read-only into nginx)
#  3. rewrites API_KEY in backend/.env (gateway reads it at startup)
#  4. replays the secret into the running api containers
# Then reload the edge:  make reload-nginx   (or `make prod-fast`)
#
# The value never appears in a bundle, image layer, or CI log.
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$secretFile = Join-Path $root 'nginx\secrets\api_key'
$envFile = Join-Path $root 'backend\.env'

if (-not (Test-Path (Split-Path -Parent $secretFile))) {
    New-Item -ItemType Directory -Path (Split-Path -Parent $secretFile) | Out-Null
}

$bytes = [byte[]]::new(32)
[System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
$key = -join ($bytes | ForEach-Object { $_.ToString('x2') })

[System.IO.File]::WriteAllText($secretFile, "$key`n", (New-Object System.Text.UTF8Encoding($false)))

$envText = [System.IO.File]::ReadAllText($envFile)
if ($envText -match '(?m)^API_KEY=.*$') {
    $envText = $envText -replace '(?m)^API_KEY=.*$', "API_KEY=$key"
} else {
    $envText = $envText.TrimEnd() + "`nAPI_KEY=$key`n"
}
[System.IO.File]::WriteAllText($envFile, $envText, (New-Object System.Text.UTF8Encoding($false)))

Write-Host "Rotated API key (32 bytes hex)."
Write-Host "  nginx/secrets/api_key  <- updated"
Write-Host "  backend/.env           <- API_KEY updated"
Write-Host "Next: make prod-fast   (recreates api1/api2 + reloads nginx)"
