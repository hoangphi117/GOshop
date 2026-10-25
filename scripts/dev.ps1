# =============================================================================
# GOshop Single-Command Dev Orchestrator (Lightweight Hybrid Mode)
# =============================================================================
# 1. Starts Docker Infrastructure (Postgres, Redis, RabbitMQ, MinIO, Meilisearch)
# 2. Runs core Go services on Host; domain services run in Docker Compose
# =============================================================================

Write-Host ">>> [1/2] Starting Infrastructure and Domain Services (Docker)..." -ForegroundColor Cyan
docker compose -f ./docker-compose.dev.yml stop api identity notification order payment worker ws-gateway 2>$null | Out-Null
docker compose -f docker-compose.dev.yml --env-file ./backend/.env up -d minio inventory catalog coupon cart banner

if ($LASTEXITCODE -ne 0) {
    Write-Host "Failed to start infrastructure containers!" -ForegroundColor Red
    exit 1
}

Write-Host ">>> [2/3] Building Go Binaries (Sequential)..." -ForegroundColor Yellow
$binDir = Join-Path $PSScriptRoot "..\backend\bin"
if (!(Test-Path $binDir)) { New-Item -ItemType Directory -Path $binDir -Force | Out-Null }

$services = @("api", "identity", "notification", "payment", "worker", "coupon", "inventory", "catalog", "cart", "order")
$hostServices = @("api", "identity", "notification", "payment", "worker", "order")
$backendDir = Join-Path $PSScriptRoot "..\backend"
$serviceBinaryPaths = $services | ForEach-Object {
    [System.IO.Path]::GetFullPath((Join-Path $backendDir "bin\$_.exe"))
}
$runningServiceProcesses = Get-Process -ErrorAction SilentlyContinue | Where-Object {
    $_.Path -and $serviceBinaryPaths -contains ([System.IO.Path]::GetFullPath($_.Path))
}
foreach ($process in $runningServiceProcesses) {
    Stop-Process -Id $process.Id -Force
}

$requiredPorts = @(8080, 8081, 8082, 8083, 8088, 50057)
$deadline = (Get-Date).AddSeconds(10)
do {
    $occupiedPorts = @(
        Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue |
            Where-Object { $requiredPorts -contains $_.LocalPort }
    )
    if ($occupiedPorts.Count -eq 0) {
        break
    }
    Start-Sleep -Milliseconds 250
} while ((Get-Date) -lt $deadline)

if ($occupiedPorts.Count -gt 0) {
    $portList = ($occupiedPorts | Select-Object -ExpandProperty LocalPort -Unique) -join ", "
    Write-Host "Required host ports are still in use: $portList" -ForegroundColor Red
    Write-Host "Stop the owning process, then run make dev-host again." -ForegroundColor Red
    exit 1
}

Push-Location $backendDir
try {
    foreach ($svc in $hostServices) {
        Write-Host "Compiling $svc..." -ForegroundColor DarkGray
        $out = if ($IsWindows -or $env:OS -match "Windows") { "bin/$svc.exe" } else { "bin/$svc" }
        & go build -o $out "./cmd/$svc"
        if ($LASTEXITCODE -ne 0) {
            Write-Host "Build failed for $svc!" -ForegroundColor Red
            exit 1
        }
    }
} finally {
    Pop-Location
}

Write-Host ">>> [3/3] Starting Go Services on Host" -ForegroundColor Green
$exe = if ($IsWindows -or $env:OS -match "Windows") { ".exe" } else { "" }
$binPrefix = if ($IsWindows -or $env:OS -match "Windows") { "bin\" } else { "./bin/" }
npx -y concurrently `
  -n "api,identity,notif,payment,worker,order" `
  -c "cyan,green,magenta,blue,yellow,orange" `
  --kill-others-on-fail `
  "cd backend && ${binPrefix}api$exe" `
  "cd backend && ${binPrefix}identity$exe" `
  "cd backend && ${binPrefix}notification$exe" `
  "cd backend && ${binPrefix}payment$exe" `
  "cd backend && ${binPrefix}worker$exe" `
  "cd backend && ${binPrefix}order$exe"
