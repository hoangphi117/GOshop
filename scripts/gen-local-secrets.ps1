# Rotates the edge API key in one pass:
#  1. new 32-byte random key
#  2. writes nginx/secrets/api_key (mounted read-only into nginx)
#  3. rewrites API_KEY in backend/.env (gateway reads it at startup)
#  4. replays the secret into the running api containers
# Then reload the edge:  make reload-nginx   (or `make prod-fast`)
#
# Also ensures INTERNAL_API_TOKEN exists and matches in every env file that
# reads it. That token is a different secret with a different job: it guards the
# recommendations write route (/internal/embeddings/:uuid). Compose interpolates
# it from the environment (${INTERNAL_API_TOKEN:?...}), NOT from env_file, so a
# missing value stops `make prod` at config time. It is kept stable across runs
# on purpose: rotating it means catalog's publish hook starts sending a token
# the recommender rejects, and new products silently stop being embedded.
# Force a rotation with -RotateInternalToken.
#
# The values never appear in a bundle, image layer, or CI log.
param(
    [switch]$RotateInternalToken
)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$secretFile = Join-Path $root 'nginx\secrets\api_key'
$envFile = Join-Path $root 'backend\.env'
# prod secrets live beside .env and are read by `make prod` via ENV_FILE, but a
# deployment that copies one over the other needs both to agree.
$envFiles = @(
    (Join-Path $root 'backend\.env'),
    (Join-Path $root 'backend\.env.prod')
)

if (-not (Test-Path (Split-Path -Parent $secretFile))) {
    New-Item -ItemType Directory -Path (Split-Path -Parent $secretFile) | Out-Null
}

function New-HexSecret([int]$bytes) {
    $buf = [byte[]]::new($bytes)
    [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($buf)
    return (-join ($buf | ForEach-Object { $_.ToString('x2') }))
}

$utf8 = New-Object System.Text.UTF8Encoding($false)

$key = New-HexSecret 32
[System.IO.File]::WriteAllText($secretFile, "$key`n", $utf8)

# --- INTERNAL_API_TOKEN: reuse the existing value unless asked to rotate -----
# Read the current value from whichever env file already has one, so a second
# run does not invalidate a running catalog -> recommendations handshake.
$internalToken = $null
if (-not $RotateInternalToken) {
    foreach ($f in $envFiles) {
        if (-not (Test-Path $f)) { continue }
        $m = [regex]::Match([System.IO.File]::ReadAllText($f), '(?m)^INTERNAL_API_TOKEN=(\S+)\s*$')
        if ($m.Success) { $internalToken = $m.Groups[1].Value; break }
    }
}
if (-not $internalToken) { $internalToken = New-HexSecret 24 }

foreach ($file in $envFiles) {
    if (-not (Test-Path $file)) {
        Write-Host "  $file not found; skipped"
        continue
    }
    $text = [System.IO.File]::ReadAllText($file)
    if ($text -match '(?m)^INTERNAL_API_TOKEN=') {
        $text = $text -replace '(?m)^INTERNAL_API_TOKEN=.*$', "INTERNAL_API_TOKEN=$internalToken"
    } else {
        $text = $text.TrimEnd() + "`nINTERNAL_API_TOKEN=$internalToken`n"
    }
    if ($file -eq $envFile) {
        if ($text -match '(?m)^API_KEY=.*$') {
            $text = $text -replace '(?m)^API_KEY=.*$', "API_KEY=$key"
        } else {
            $text = $text.TrimEnd() + "`nAPI_KEY=$key`n"
        }
    }
    [System.IO.File]::WriteAllText($file, $text, $utf8)
}

Write-Host "Rotated API key (32 bytes hex)."
Write-Host "  nginx/secrets/api_key  <- updated"
Write-Host "  backend/.env           <- API_KEY updated"
if ($RotateInternalToken) {
    Write-Host "  INTERNAL_API_TOKEN     <- ROTATED in every env file"
    Write-Host "Next: recreate catalog AND recommendations together, or the publish"
    Write-Host "      hook will 401 until both read the new value:"
    Write-Host "      docker compose -f docker-compose.prod.yml --env-file ./backend/.env up -d --force-recreate catalog recommendations"
} else {
    Write-Host "  INTERNAL_API_TOKEN     <- ensured present, value unchanged"
}
Write-Host "Next: make prod-fast   (recreates api1/api2 + reloads nginx)"
