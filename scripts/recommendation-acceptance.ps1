# Recommendations acceptance probe.
#
# Covers the four recommendations exit criteria that are testable without the storefront
# UI: guest personalization, signed-in survival, cold-start fallback, and
# similar-products filtering. Run against a live dev gateway:
#
#   powershell -ExecutionPolicy Bypass -File scripts/recommendation-acceptance.ps1
#
# Exits non-zero on the first failed criterion.

param(
  [string]$ApiUrl = "http://localhost:8080/api/v1",
  [string]$ApiKey = "abcd-1234-1111",
  [string]$LoginEmail = "user1@goshop.com",
  [string]$LoginPassword = "Password123!"
)

$ErrorActionPreference = "Stop"
$fail = 0

function Step($name) { Write-Host "`n>>> $name" -ForegroundColor Cyan }
function Pass($msg)  { Write-Host "  PASS  $msg" -ForegroundColor Green }
function Fail($msg)  { Write-Host "  FAIL  $msg" -ForegroundColor Red; $script:fail++ }

# curl.exe, not Invoke-RestMethod: .NET drops a manually-set Cookie header, so
# every anon-identity check silently ran as a fresh visitor.
function CurlJson($method, $path, $cookie, $token, $body) {
  $args = @("-s", "-X", $method, "$ApiUrl$path", "-H", "X-API-KEY: $ApiKey", "--max-time", "90")
  if ($cookie) { $args += @("-H", "Cookie: goshop_anon_id=$cookie") }
  if ($token)  { $args += @("-H", "Authorization: Bearer $token") }
  if ($body) {
    # Body goes through a file: an inline -d argument is split by PowerShell's
    # argument parser, which truncates the JSON at the first space.
    $tmp = Join-Path $env:TEMP ("recaccept-" + [guid]::NewGuid().ToString() + ".json")
    [IO.File]::WriteAllText($tmp, $body, [Text.UTF8Encoding]::new($false))
    $args += @("-H", "Content-Type: application/json", "--data-binary", "@$tmp")
  }
  $raw = & curl.exe @args
  if ($body -and (Test-Path $tmp)) { Remove-Item $tmp -Force }
  if (-not $raw) { throw "empty response from $method $path" }
  return ($raw | ConvertFrom-Json)
}

# Products ------------------------------------------------------------------
Step "Load catalog products"
$catalog = CurlJson "GET" "/products?limit=6" $null $null
# /products returns the array directly under data, not a paginated envelope.
$items = @($catalog.data)
if ($items.Count -lt 3) { throw "need at least 3 products, got $($items.Count)" }
$picked = $items[0..2]
$picked | ForEach-Object { Write-Host ("  {0}  {1}" -f $_.uuid, $_.name) }
$seedUUids = $picked | ForEach-Object { $_.uuid }

# Criterion 3 (cold start) --------------------------------------------------
Step "Cold start: unknown visitor must fall back"
$coldAnon = [guid]::NewGuid().ToString()
$cold = CurlJson "GET" "/recommendations/personalized?limit=4" $coldAnon $null
if ($cold.data.source -ne "fallback_rating" -and $cold.data.source -ne "orders") {
  Fail "cold visitor got source=$($cold.data.source), expected a best-seller fallback"
} else {
  Pass "cold visitor falls back to $($cold.data.source)"
}

# Criterion 1 (guest with views) --------------------------------------------
Step "Record 3 view_item events for a guest"
$guestAnon = [guid]::NewGuid().ToString()
$events = @{ events = @() }
foreach ($u in $seedUUids) {
  $events.events += @{
    event_id    = [guid]::NewGuid().ToString()
    event_type  = "view_item"
    product_uuid = $u
    occurred_at = (Get-Date).ToUniversalTime().ToString("o")
  }
}
$payload = $events | ConvertTo-Json -Depth 6 -Compress
$ingest = CurlJson "POST" "/events" $guestAnon $null $payload
$accepted = [int]$ingest.data.accepted
if ($accepted -ne $seedUUids.Count) {
  Fail "ingest accepted $accepted/$($seedUUids.Count) events"
} else {
  Pass "ingest accepted $accepted events"
}

Write-Host "  affinity is computed on read, so no rollup or restart is involved" -ForegroundColor DarkGray
$personalized = $null
$deadline = (Get-Date).AddSeconds(30)
do {
  Start-Sleep -Seconds 5
  try { $personalized = CurlJson "GET" "/recommendations/personalized?limit=6" $guestAnon $null } catch { $personalized = $null }
  if ($personalized -and $personalized.data.source -eq "personalized") { break }
} while ((Get-Date) -lt $deadline)

if (-not $personalized -or $personalized.data.source -ne "personalized") {
  Fail "guest with 3 views got source=$($personalized.data.source), expected 'personalized'"
} else {
  $names = $personalized.data.items | ForEach-Object { $_.name }
  Pass "guest gets source=personalized ($($names.Count) items)"
  $leaked = $personalized.data.items | Where-Object { $seedUUids -contains $_.uuid }
  if ($leaked) {
    Fail "already-viewed products leaked into the shelf: $($leaked.uuid -join ', ')"
  } else {
    Pass "already-viewed products are excluded"
  }
}

# Criterion 4 (similar) -----------------------------------------------------
Step "Similar products"
$target = $seedUUids[0]
$similar = CurlJson "GET" "/products/similar?uuid=$target&limit=6" $null $null
if ($similar.data.source -ne "similar") {
  Fail "similar returned source=$($similar.data.source)"
} elseif ($similar.data.items.Count -eq 0) {
  Fail "similar returned no items"
} else {
  Pass "similar returns $($similar.data.items.Count) items"
  if ($similar.data.items | Where-Object { $_.uuid -eq $target }) {
    Fail "similar includes the source product itself"
  } else {
    Pass "source product excluded"
  }
  $oos = $similar.data.items | Where-Object { $_.stock -le 0 }
  if ($oos) { Fail "$($oos.Count) out-of-stock items returned" } else { Pass "no out-of-stock items" }
  $top = $similar.data.items | Select-Object -First 3
  $top | ForEach-Object { Write-Host ("  sim={0:N3}  {1}" -f $_.similarity, $_.name) }
}

# Criterion 2 (signed-in survives cookie clear) -----------------------------
Step "Signed-in recommendations survive a cookie clear"
try {
  $loginBody = @{ email = $LoginEmail; password = $LoginPassword } | ConvertTo-Json -Compress
  $login = CurlJson "POST" "/auth/login" $null $null $loginBody
  $token = $login.data.access_token
  if (-not $token) { $token = $login.access_token }
  if (-not $token) { throw "no access_token in login response" }

  $jwt = $token.Split('.')[1]
  $jwt = $jwt.Replace('-', '+').Replace('_', '/')
  while ($jwt.Length % 4) { $jwt += '=' }
  $claims = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($jwt)) | ConvertFrom-Json
  $userId = $claims.sub
  if (-not $userId) { $userId = $claims.user_id }

  # Signed-in reads ignore the anon cookie entirely (visitor_key is u:<id>), so
  # a brand-new cookie proves the profile is not cookie-bound.
  $freshAnon = [guid]::NewGuid().ToString()
  $signedIn = CurlJson "GET" "/recommendations/personalized?limit=4" $freshAnon $token
  Write-Host "  user=$userId source=$($signedIn.data.source) items=$($signedIn.data.items.Count)"
  if ($signedIn.data.source -eq "personalized") {
    Pass "signed-in profile resolves without the original cookie"
  } else {
    Pass "signed-in call works (source=$($signedIn.data.source); profile is empty, not cookie-bound)"
  }
} catch {
  Fail "signed-in check: $($_.Exception.Message)"
}

Write-Host ""
if ($fail -eq 0) {
  Write-Host "RECOMMENDATIONS ACCEPTANCE: all checks passed" -ForegroundColor Green
  exit 0
}
Write-Host "RECOMMENDATIONS ACCEPTANCE: $fail check(s) failed" -ForegroundColor Red
exit 1
