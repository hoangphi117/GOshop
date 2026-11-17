# Phase-1 healthz check for all 6 Kafka consumer services.
#   powershell -ExecutionPolicy Bypass -File scripts/check-kafka-health.ps1
# Uses curl.exe, not Invoke-WebRequest: on some Windows setups IWR hangs on
# localhost (proxy/IPv6 resolution) while curl returns instantly.
$services = @(
  @{ Name = "order";        Port = 8088 },
  @{ Name = "inventory";    Port = 8090 },
  @{ Name = "cart";         Port = 8087 },
  @{ Name = "coupon";       Port = 8084 },
  @{ Name = "notification"; Port = 8082 },
  @{ Name = "analytics";    Port = 8092 }
)

$fail = 0
foreach ($s in $services) {
  $url = "http://localhost:$($s.Port)/healthz"
  $raw = $null
  $code = $null
  # Retry ~30s: Docker services may still be starting after make dev-host.
  for ($i = 0; $i -lt 6; $i++) {
    $out = & curl.exe -s -m 5 -w "`n%{http_code}" $url 2>$null
    if ($LASTEXITCODE -eq 0 -and $out) {
      $lines = $out -split "`n"
      $code = $lines[-1].Trim()
      $raw = ($lines[0..($lines.Count - 2)] -join "`n").Trim()
      if ($raw) { break }
    }
    Start-Sleep -Seconds 5
  }

  if (-not $raw) {
    Write-Host "$($s.Name): DOWN ($url unreachable after retries)"
    $fail++
    continue
  }

  $body = $raw | ConvertFrom-Json
  $lag = if ($body.messaging_lag) { ($body.messaging_lag.PSObject.Properties | ForEach-Object { "$($_.Name)=$($_.Value)" }) -join ", " } else { "(no consumers yet)" }
  Write-Host "$($s.Name): $code status=$($body.status) lag=[$lag]"
  if ($body.status -ne "healthy") { $fail++ }
}

if ($fail -gt 0) { Write-Host "$fail service(s) not healthy" -ForegroundColor Red; exit 1 }
Write-Host "All 6 consumer healthz checks healthy" -ForegroundColor Green
