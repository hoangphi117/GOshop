# Kill the Go services `make dev-host` started on the host.
#
# Safe by construction:
#   - only the exact binary names dev.ps1 launches are matched
#   - only processes running from this repo's backend\bin are killed, so an
#     unrelated `api.exe` elsewhere on the machine survives
#   - nothing is deleted: containers and volumes stay as `docker compose down`
#     left them (use `down -v` yourself if you really want the data gone)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$binDir = [System.IO.Path]::GetFullPath((Join-Path $repoRoot "backend\bin"))

$names = @(
  "api", "identity", "notification", "payment", "worker",
  "order", "review", "analytics", "chat", "media", "ws-gateway"
)

$killed = 0
foreach ($name in $names) {
  Get-Process -Name $name -ErrorAction SilentlyContinue | ForEach-Object {
    $path = $null
    try { $path = $_.Path } catch { }
    if ($path -and [System.IO.Path]::GetFullPath($path).StartsWith($binDir, [StringComparison]::OrdinalIgnoreCase)) {
      Stop-Process -Id $_.Id -Force
      Write-Host "stopped $name (pid $($_.Id))"
      $killed++
    }
  }
}

if ($killed -eq 0) {
  Write-Host "no dev-host services were running"
} else {
  Write-Host "stopped $killed service(s)"
}
