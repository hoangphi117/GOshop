# Diagnoses a Redis connection URL against what config/redis.go actually reads.
#
# redis.go builds redis.Options{Addr, Username, Password} - it has no
# TLSConfig and no URL parsing. So the URL Render hands out must be split by
# hand, and a "broken pipe" usually means the split was skipped (credentials
# still inside Addr, or TLS expected by the server).
#
#   .\scripts\check-redis-url.ps1 -Url "redis://default:PASS@red-abc:6379"
param([Parameter(Mandatory = $true)][string]$Url)

$ErrorActionPreference = "Stop"

$u = $Url -replace '^rediss?://', ''
$isTLS = $Url -match '^rediss://'

$user = ""
$pass = ""
$hostport = $u

if ($u -match '@') {
  $creds, $hostport = $u -split '@', 2
  if ($creds -match ':') {
    $user, $pass = $creds -split ':', 2
  } else {
    $user = $creds
  }
}

Write-Output "scheme:    $(if ($isTLS) { 'rediss (TLS)' } else { 'redis (plaintext)' })"
Write-Output "host:port: $hostport"
Write-Output "username: $(if ($user) { $user } else { '(none)' })"
Write-Output "password: $(if ($pass) { '<set, ' + $pass.Length + ' chars>' } else { '(none)' })"
Write-Output ""

Write-Output "Set these in Render:"
Write-Output "  REDIS_ADDR=$hostport"
if ($user) { Write-Output "  REDIS_USERNAME=$user" }
if ($pass) { Write-Output "  REDIS_PASSWORD=<the password from the URL>" }
if (-not $pass) { Write-Output "  (no REDIS_PASSWORD - only valid if internal auth is disabled)" }

Write-Output ""
if ($isTLS) {
  Write-Output "BLOCKER: rediss:// needs TLS, but config/redis.go never sets a TLSConfig."
  Write-Output "         Either use the internal (plain redis://) URL, or add TLS support to redis.go."
}
if ($hostport -match ':') {
  $port = ($hostport -split ':')[-1]
  if ($port -ne '6379') {
    Write-Output "NOTE: non-default port $port - confirm this is the internal URL, not external."
  }
}
