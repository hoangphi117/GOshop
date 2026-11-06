# Query Grafana Cloud Loki logs. Needs GRAFANA_LOKI_* keys in backend/.env
# (read token with logs:read scope).
#
# Usage:
#   .\scripts/logq.ps1 '{container="goshop-api1"}'                 # last 2h
#   .\scripts/logq.ps1 '{service="nginx"} |= "POST"' -Hours 24 -Limit 20
#   .\scripts/logq.ps1 '{service=~"api1|api2"} | json | level="warn"'
#
# Quoting through cmd/make strips bare double quotes, which LogQL requires, so
# a value written without them is quoted here: {container=goshop-order} means
# the same as {container="goshop-order"}.
param(
    [Parameter(Mandatory = $true)][string]$Query,
    [int]$Hours = 2,
    [int]$Limit = 10,
    [ValidateSet("backward", "forward")][string]$Direction = "backward"
)

# Quote unquoted stream-selector values so cmd/make can pass LogQL without
# double quotes: {container=goshop-order} == {container="goshop-order"}.
# Only the leading {...} block is touched; pipelines after it are left as-is.
$selectorEnd = $Query.IndexOf("}")
if ($Query.StartsWith("{") -and $selectorEnd -gt 0) {
    $head = $Query.Substring(0, $selectorEnd + 1)
    $tail = $Query.Substring($selectorEnd + 1)
    $head = [regex]::Replace($head, '([=!]=?~?)\s*([A-Za-z0-9_.:-]+)', {
        param($m) $op = $m.Groups[1].Value; "$op`"$($m.Groups[2].Value)`""
    })
    $Query = $head + $tail
}
Write-Verbose "logql: $Query"

$ErrorActionPreference = "Stop"
$envFile = Join-Path $PSScriptRoot "..\backend\.env"
$vars = @{}
foreach ($line in Get-Content $envFile) {
    if ($line -match '^(GRAFANA_[A-Z_]+)=(.*)$') { $vars[$Matches[1]] = $Matches[2] }
}
foreach ($k in @("GRAFANA_LOKI_URL", "GRAFANA_LOKI_USER", "GRAFANA_LOKI_READ_TOKEN")) {
    if (!$vars[$k]) { Write-Host "Missing $k in backend/.env" -ForegroundColor Red; exit 1 }
}

$base = $vars["GRAFANA_LOKI_URL"].Replace("/loki/api/v1/push", "")
$pair = "$($vars['GRAFANA_LOKI_USER']):$($vars['GRAFANA_LOKI_READ_TOKEN'])"
$now = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$from = $now - ($Hours * 3600)
$url = "$base/loki/api/v1/query_range?query=" + [uri]::EscapeDataString($Query) +
    "&start=$from`000000000&end=$now`000000000&limit=$Limit&direction=$Direction"

$out = Join-Path $env:TEMP "loki_q.json"
& curl.exe -s -o $out -w "HTTP %{http_code}`n" -u $pair $url --max-time 30
$j = (Get-Content $out -Raw) | ConvertFrom-Json
if ($j.status -ne "success") { Write-Output $out; exit 1 }

$total = ($j.data.result | ForEach-Object { $_.values.Count } | Measure-Object -Sum).Sum
Write-Output "streams: $($j.data.result.Count), entries: $total (window: last $Hours h)"
foreach ($stream in $j.data.result) {
    Write-Output "--- $($stream.stream.container) [$($stream.stream.level)] ---"
    foreach ($v in $stream.values) {
        $t = [DateTimeOffset]::FromUnixTimeMilliseconds([long]([double]$v[0] / 1e6)).LocalDateTime.ToString("HH:mm:ss")
        $line = $v[1]
        if ($line.Length -gt 200) { $line = $line.Substring(0, 200) + " ..." }
        Write-Output "  $t  $line"
    }
}
