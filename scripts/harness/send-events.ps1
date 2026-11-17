<#
  Sends one honest event batch through the real ingest endpoint, so the rows go
  through the same validation (event_id, occurred_at range, session_id parse)
  the storefront's tracker hits.

  Identity is still supplied out-of-band: the handler reads anon_id from the
  cookie and user_id from the JWT. For scripted testing the anon cookie is an
  explicit parameter; the JWT case needs a login and is not covered here.

  Goes through the edge (nginx) by default: it is the only hop that injects
  X-API-KEY, and curl cannot set the header without the secret.
#>
param(
    [Parameter(Mandatory)][string]$AnonId,
    [Parameter(Mandatory)][string]$SessionId,
    [Parameter(Mandatory)][string[]]$ProductUuids,
    [string]$ApiUrl = "http://127.0.0.1",
    [string]$SearchQuery = ""
)

$ErrorActionPreference = "Stop"

$events = foreach ($p in $ProductUuids) {
    @{
        event_id     = [guid]::NewGuid().ToString()
        event_type   = "view_item"
        session_id   = $SessionId
        product_uuid = $p
        occurred_at  = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
    }
}
if ($SearchQuery) {
    $events += @{
        event_id    = [guid]::NewGuid().ToString()
        event_type  = "search"
        session_id  = $SessionId
        query       = $SearchQuery
        occurred_at = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
    }
}

$body = @{ events = @($events) } | ConvertTo-Json -Depth 5 -Compress
$tmp = Join-Path $env:TEMP ("goshop-events-" + [guid]::NewGuid().ToString("N") + ".json")
# ConvertTo-Json via a file, not a command-line argument: PowerShell strips the
# inner double quotes when it passes JSON as an argument, and PostgreSQL then
# reports `Token "quantity" is invalid` on the far side.
#
# -Encoding ascii, not utf8: Windows PowerShell's "utf8" is UTF-8 *with* a BOM,
# and the ingest service rejects the leading \ufeff as invalid JSON. The payload
# is ASCII anyway (uuids, timestamps, event names).
Set-Content -Path $tmp -Value $body -Encoding ascii -NoNewline

try {
    $status = curl.exe -sS -o NUL -w "%{http_code}" `
        -X POST "$ApiUrl/api/v1/events" `
        -H "Content-Type: application/json" `
        -H "X-Client-App: storefront" `
        -H "Cookie: goshop_anon_id=$AnonId" `
        --data-binary "@$tmp" --max-time 30
} finally {
    Remove-Item $tmp -ErrorAction SilentlyContinue
}

if ($status -notin @("200", "202")) {
    throw "event ingest failed with HTTP $status (is the edge up at $ApiUrl?)"
}
Write-Host "  ingested $($events.Count) event(s) for anon $AnonId"
