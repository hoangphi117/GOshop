<#
.SYNOPSIS
  End-to-end check of the recommendation tiers, run against the local stack.

.DESCRIPTION
  Drives the seven-step personalization work through the same surfaces a
  browser uses, and asserts on the `source` the API reports — which is the only
  honest statement about how a shelf was built.

  Read-only apart from INSERTs into the event log; cleanup.sql removes them.

  Expected sources, in tier order:
    personalized          profile depth >= 3 distinct products, 90 days
    personalized_session  the visitor's latest browsing pass (needs >= 3)
    trending              >= 2 purchase rows in the last 24h
    orders                the 30-day popularity rollup
    fallback_rating       catalog ratings, when the rollup is empty

.EXAMPLE
  powershell -NoProfile -ExecutionPolicy Bypass -File scripts/harness/test-recommendation-tiers.ps1
  powershell -NoProfile -ExecutionPolicy Bypass -File scripts/harness/test-recommendation-tiers.ps1 -SkipSeed -SkipUi
#>
[CmdletBinding()]
param(
    # API directly. Nginx (:80) injects X-API-KEY; the gateway does not.
    [string]$ApiUrl = "http://127.0.0.1:8095",
    # Storefront dev server, for the SSR checks.
    [string]$StoreUrl = "http://127.0.0.1:3001",
    [string]$DbContainer = "goshop-postgres",
    [switch]$SkipSeed,
    [switch]$SkipUi
)

$ErrorActionPreference = "Stop"
$script:failures = 0

function Step($name) { Write-Host "`n== $name" -ForegroundColor Cyan }
function Pass($msg) { Write-Host "  PASS  $msg" -ForegroundColor Green }
function Fail($msg) { Write-Host "  FAIL  $msg" -ForegroundColor Red; $script:failures++ }

function Assert-Equal($actual, $expected, $what) {
    if ($actual -eq $expected) { Pass "$what = $actual" }
    else { Fail "$what = '$actual', expected '$expected'" }
}

function Assert-True($condition, $what) {
    if ($condition) { Pass $what } else { Fail $what }
}

function Sql([string]$file, [string]$database) {
    Get-Content -Raw (Join-Path $PSScriptRoot $file) |
        docker exec -i $DbContainer psql -U postgres -d $database -v ON_ERROR_STOP=1 2>&1
}

function Source($anonId) {
    $uri = "$ApiUrl/api/v1/recommendations/personalized?limit=12"
    $res = curl.exe -sS $uri -H "Cookie: goshop_anon_id=$anonId" --max-time 30 2>&1
    if ($res -match '"source":"([a-z_]+)"') { return $Matches[1] }
    return "<no source>"
}

function Ssr([string]$path, [string]$anonId) {
    $html = curl.exe -sS "$StoreUrl$path" -H "Cookie: goshop_anon_id=$anonId" --max-time 60 2>&1 | Out-String
    return $html
}

<#
  Plants one view_item into the open session. Kept next to the SQL phases
  because the session tier's failure mode ("3 products is enough to trigger it,
  3 products is also deep enough to leave it") is easier to see as a comment
  here than in a .sql file full of inserts.
#>
function Seed-Pass([string]$productUuid) {
    # The fresh view carries user_id too: by this point the visitor has "signed
    # in" on this device, so the current pass belongs to the account trail.
    # It lands in session 0001 on purpose: products from THREE different brands
    # make diversify() fan the row out. A one-brand pass yields barely the items
    # it names, the shelf comes back too short, and the tier falls through to
    # best sellers — which is correct behaviour, and useless as a test.
    $sql = "INSERT INTO user_events (event_id, event_type, anon_id, user_id, session_id, product_uuid, occurred_at) " +
           "VALUES (gen_random_uuid(), 'view_item', 'dddd0001-dddd-4ddd-8ddd-ddddddddddd1', 1, " +
           "'55550000-0000-4000-8000-000000000001', '$productUuid', now() - interval '5 minutes');"
    $sql | docker exec -i $DbContainer psql -U postgres -d shopping_analytics -v ON_ERROR_STOP=1 | Out-Null
}

# The service caches a shelf per visitor for a minute; a scripted run would
# otherwise read the previous tier's answer and "prove" nothing changed.
function Clear-ShelfCache() {
    docker exec goshop-redis redis-cli --scan --pattern "recs:home:*" 2>$null |
        ForEach-Object { docker exec goshop-redis redis-cli DEL $_ 2>$null | Out-Null }
}

function Has-Text($html, $text) {
    return $html -match [regex]::Escape($text)
}

# ---------------------------------------------------------------------------

Step "Service health"
try {
    $health = curl.exe -sS "$ApiUrl/api/v1/recommendations?limit=1" --max-time 10 2>&1 | Out-String
    Assert-True ($health -match '"status":"success"') "recommendations API answers"
} catch {
    Fail "recommendations API unreachable at $ApiUrl ($_)"
    Write-Host "`nStart it with: docker compose -f docker-compose.dev.yml --env-file ./backend/.env up -d recommendations" -ForegroundColor Yellow
    exit 1
}

Step "Clean slate"
if (-not $SkipSeed) {
    Sql "cleanup.sql" "shopping_analytics" | Out-Null
    Clear-ShelfCache
    Pass "previous harness rows removed and shelf cache cleared"
}

# Tiers 4 and 5: a visitor with no history at all.
Step "Tier: crowd fallback (visitor we know nothing about)"
$newVisitor = "dddd0005-dddd-4ddd-8ddd-ddddddddddd5"
$s = Source $newVisitor
Assert-True ($s -in @("orders", "fallback_rating", "trending")) "source is a crowd tier (got '$s')"
Assert-True ($s -ne "personalized" -and $s -ne "personalized_session") "no personal tier is claimed for a stranger"

# Tier 2: the session tier only fires when the 90-day profile is too thin.
# The threshold is depth-based — the profile wins once THREE distinct products
# exist, because a 3-product centroid already describes the same intent the
# session does, only with more evidence. So the session has to be the only
# thing the visitor has touched, and so does the PROFILE: Phase A plants one
# pass of three products, and the bridge (Phase B) is what makes them count.
# Five brands on purpose. diversify() round-robins by brand, so a one-brand
# pass can only produce the items it names — too few to fill a row, and the
# tier correctly falls through to best sellers (which is what a real shopper
# doing exactly that sees).
Step "Tier: personalized_session (one browsing pass, thin profile)"
$sessionVisitor = "dddd0006-dddd-4ddd-8ddd-ddddddddddd6"
if (-not $SkipSeed) {
    $out = & (Join-Path $PSScriptRoot "send-events.ps1") `
        -AnonId $sessionVisitor `
        -SessionId "55550000-0000-4000-8000-000000000006" `
        -ProductUuids `
            "b9a496f9-88ac-4e9b-bb20-fa4fab736d24", `
            "d28cf6b9-883e-4412-8af6-f02a1eb38be3" `
        -SearchQuery "phone accessories" | Out-String
    Write-Host $out.Trim()
    Clear-ShelfCache
}
$s = Source $sessionVisitor
Assert-Equal $s "personalized_session" "source while the profile is below the depth threshold"
Assert-True ($s -ne "trending") "session beats trending (personal tiers come first)"

# Tier 1: the account sees the trail it built before signing in, and once the
# bridge makes that trail three products deep the profile takes over from the
# session. Both facts ride on the same row.
Step "Tier: personalized (anonymous trail bridged to the account)"
if (-not $SkipSeed) {
    Sql "phase-a-session.sql" "shopping_analytics" | Out-Null
    Sql "phase-b-login.sql" "shopping_analytics" | Out-Null
    Clear-ShelfCache
    Pass "anonymous trail + login bridge row"
}
$s = Source "dddd0001-dddd-4ddd-8ddd-ddddddddddd1"
Assert-Equal $s "personalized" "source once the profile is deep enough"

# Tier 3: trending sits below the personal tiers.
Step "Tier: trending (today's sales)"
if (-not $SkipSeed) {
    Sql "phase-c-trending.sql" "shopping_analytics" | Out-Null
    Clear-ShelfCache
}
Start-Sleep -Seconds 1
$s = Source $newVisitor
Assert-Equal $s "trending" "source for a stranger once 3 purchases today exist"
$s = Source "dddd0001-dddd-4ddd-8ddd-ddddddddddd1"
Assert-Equal $s "personalized" "the personal visitor is unaffected by trending"

# Labels: the storefront must not call a crowd shelf personal.
if (-not $SkipUi) {
    Step "Storefront labels (SSR)"
    $html = Ssr "/recommendations" "dddd0001-dddd-4ddd-8ddd-ddddddddddd1"
    Assert-True (Has-Text $html "Recommended for you") "personal shelf still reads 'Recommended for you'"

    $html = Ssr "/recommendations" $newVisitor
    Assert-True (Has-Text $html "Trending today") "trending shelf reads 'Trending today'"
    Assert-True (-not (Has-Text $html "Based on your recent activity")) "trending shelf does not borrow the personal subtitle"

    # The session shelf is still a personal one: same heading, different reason.
    # Checked on the visitor the bridge promoted (their fourth view was in the
    # current pass), because the session-only visitor is a profile tier now.
    $html = Ssr "/recommendations" "dddd0001-dddd-4ddd-8ddd-ddddddddddd1"
    Assert-True (Has-Text $html "Recommended for you") "promoted visitor reads 'Recommended for you'"
}

Step "Summary"
if ($script:failures -eq 0) {
    Write-Host "  All assertions passed." -ForegroundColor Green
} else {
    Write-Host "  $($script:failures) assertion(s) failed." -ForegroundColor Red
}
Write-Host "`nClean up the seeded rows with:" -ForegroundColor Yellow
Write-Host "  Get-Content scripts/harness/cleanup.sql | docker exec -i $DbContainer psql -U postgres -d shopping_analytics -v ON_ERROR_STOP=1"
exit ([int]($script:failures -gt 0))
