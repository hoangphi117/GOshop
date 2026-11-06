# Adds every Kafka secret to backend/.env: one SCRAM password per client plus
# the broker TLS settings. Merged from gen-compose-env.ps1 + gen-kafka-tls-env.ps1.
#
# backend/.env is the single source of truth. Compose reads it both as a
# container env_file and, via `--env-file backend/.env`, for ${VAR}
# interpolation, so there is no second copy to drift.
#
#   make kafka-env                 # add any missing value, keep existing
#   make kafka-env ROTATE=1        # regenerate every Kafka password
#
# Idempotent. Passwords are never printed; read backend/.env when you need one.
#
# Run it again after `make env-prod`/`make env-dev`, because those replace
# backend/.env wholesale from a committed variant that has no Kafka entries.
#
# The TLS block needs scripts/gen-kafka-tls-certs.ps1 to have run first; it is
# skipped (with a warning) when the JKS files are absent so a plaintext
# deployment still gets its SASL passwords.

param([switch]$Rotate, [string]$KeystorePassword = "changeit")

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$target = Join-Path $root "backend\.env"
$tls = Join-Path $root "backend\kafka\tls"

if (-not (Test-Path $target)) {
  throw "backend/.env is missing; run `make env-dev` or `make env-prod` first"
}

# One SCRAM user per Kafka client. No ws-gateway: it does not touch Kafka.
$kafkaUsers = @("API", "IDENTITY", "NOTIFICATION", "PAYMENT", "ORDER", "INVENTORY",
  "CATALOG", "COUPON", "CART", "BANNER", "REVIEW", "CHAT", "MEDIA", "ANALYTICS", "WORKER")

function New-Secret {
  # 32 alphanumerics: no $, no space, no quotes, safe in .env and in a SCRAM
  # password without escaping.
  $chars = ((48..57) + (65..90) + (97..122) | ForEach-Object { [char]$_ })
  -join ($chars | Get-Random -Count 32)
}

$lines = [System.Collections.Generic.List[string]](Get-Content $target)

# Index of the last line per key, so an existing entry is replaced in place
# rather than duplicated.
function Find-Key([string]$key) {
  for ($i = $lines.Count - 1; $i -ge 0; $i--) {
    if ($lines[$i] -match "^\s*$([regex]::Escape($key))\s*=") { return $i }
  }
  return -1
}

$added = 0
function Set-Key([string]$key, [string]$value) {
  $i = Find-Key $key
  if ($i -ge 0) {
    # A key copied from the example template still holds its placeholder, and
    # an empty value means "unset" to compose's ${VAR:?} guard, so neither
    # counts as configured. Replace it; leave a real value alone.
    $current = ($lines[$i] -split "=", 2)[1]
    if ($Rotate -or [string]::IsNullOrWhiteSpace($current) -or $current -eq "CHANGE_ME") {
      $lines[$i] = "$key=$value"
    }
    return
  }
  $script:added++
  $script:lines.Add("$key=$value")
}

# ---- SASL ----------------------------------------------------------------
if ((Find-Key "KAFKA_SASL_ADMIN_USER") -lt 0 -and (Find-Key "KAFKA_SASL_API_PASSWORD") -lt 0) {
  $lines.Add("")
  $lines.Add("# Kafka SASL (phase 6). Created by scripts/gen-kafka-env.ps1.")
}
Set-Key "KAFKA_SASL_ADMIN_USER" "admin"

foreach ($user in @("ADMIN") + $kafkaUsers) {
  Set-Key "KAFKA_SASL_${user}_PASSWORD" (New-Secret)
}

# ---- TLS -----------------------------------------------------------------
# Paths are container paths. docker-compose.prod.yml mounts backend/kafka/tls
# read-only at /app/kafka/tls in every service and at /etc/kafka/tls in the
# broker. Only the keystore password travels through the environment; the JKS
# files come from that mount.
$tlsReady = $true
foreach ($f in @("ca.crt", "kafka.keystore.jks", "kafka.truststore.jks")) {
  if (-not (Test-Path (Join-Path $tls $f))) { $tlsReady = $false; break }
}

if ($tlsReady) {
  Set-Key "KAFKA_SSL_KEYSTORE_PASSWORD" $KeystorePassword
  Set-Key "KAFKA_SSL_TRUSTSTORE_PASSWORD" $KeystorePassword
  Set-Key "KAFKA_TLS_ENABLED" "true"
  Set-Key "KAFKA_TLS_CA_FILE" "/app/kafka/tls/ca.crt"
} else {
  Write-Warning "backend/kafka/tls is incomplete; run scripts/gen-kafka-tls-certs.ps1 first. TLS keys skipped."
}

# Preserve the trailing newline: some tools treat a missing one as corruption.
if ($lines.Count -gt 0 -and $lines[$lines.Count - 1] -eq "") { $lines.RemoveAt($lines.Count - 1) }
[System.IO.File]::WriteAllLines($target, $lines, [System.Text.UTF8Encoding]::new($false))

Write-Host "updated backend/.env: $added new key(s), $($kafkaUsers.Count + 1) Kafka passwords $(if ($Rotate) { 'rotated' } else { 'in place' })"
if ($tlsReady) {
  Write-Host "  TLS: keystore password + KAFKA_TLS_ENABLED"
  Write-Host "  compose mounts $tls at /etc/kafka/tls (broker) and /app/kafka/tls (services)"
}
