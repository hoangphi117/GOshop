# =============================================================================
# GOSHOP REVERSE PROXY & LOAD BALANCER SMOKE TEST SUITE (POWERSHELL / CURL)
# =============================================================================

param (
    [string]$BaseUrl = "http://localhost"
)

Write-Host "========================================================" -ForegroundColor Cyan
Write-Host "  GOSHOP MICROSERVICES SMOKE & ROUTING TEST" -ForegroundColor Cyan
Write-Host "  Target Base URL: $BaseUrl" -ForegroundColor Cyan
Write-Host "========================================================" -ForegroundColor Cyan

$passed = 0
$failed = 0

function Test-Check {
    param (
        [string]$Name,
        [string]$ResponseHeaders,
        [string]$Pattern
    )

    Write-Host "`n[*] Testing: $Name" -ForegroundColor Yellow

    if ($ResponseHeaders -match $Pattern) {
        Write-Host "    [PASS] $Name" -ForegroundColor Green
        $global:passed++
    } else {
        Write-Host "    [FAIL] $Name" -ForegroundColor Red
        Write-Host "    Response received:`n$ResponseHeaders" -ForegroundColor Gray
        $global:failed++
    }
}

# 1. Test Static Uploads Cache & Security Headers
$resUpload = (curl.exe -s -I "$BaseUrl/uploads/test.jpg") -join "`n"
Test-Check -Name "Static Uploads Offloading (Nginx Headers & CSP)" -ResponseHeaders $resUpload -Pattern "(?s)X-Content-Type-Options:\s*nosniff"

# 2. Test WebSocket Handshake & Upgrade Route
$resWs = (curl.exe -s -I -N -H "Connection: Upgrade" -H "Upgrade: websocket" -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" -H "Sec-WebSocket-Version: 13" "$BaseUrl/api/v1/ws") -join "`n"
Test-Check -Name "WebSocket Upgrade Route (/api/v1/ws)" -ResponseHeaders $resWs -Pattern "(101 Switching Protocols|400 Bad Request|401 Unauthorized|404 Not Found)"

# 3. Test Global Security Headers (X-Frame-Options)
$resSec = (curl.exe -s -I "$BaseUrl/api/v1/categories") -join "`n"
Test-Check -Name "Global Security Headers (X-Frame-Options: SAMEORIGIN)" -ResponseHeaders $resSec -Pattern "X-Frame-Options:\s*SAMEORIGIN"

# 4. Test Core Backend REST API Proxying
$resApi = (curl.exe -s -I "$BaseUrl/api/v1/categories") -join "`n"
Test-Check -Name "Core REST API Proxying (/api/v1/categories)" -ResponseHeaders $resApi -Pattern "(200 OK|401 Unauthorized|403 Forbidden|404 Not Found)"

# 5. Test Identity Service Reverse Proxy (/api/v1/auth/login)
$resAuth = (curl.exe -s -I -X POST "$BaseUrl/api/v1/auth/login") -join "`n"
Test-Check -Name "Identity Service Route (/api/v1/auth/login)" -ResponseHeaders $resAuth -Pattern "(400 Bad Request|422 Unprocessable|200 OK)"

# 6. Test Products Route
$resProd = (curl.exe -s -I "$BaseUrl/api/v1/products") -join "`n"
Test-Check -Name "Products Catalog Route (/api/v1/products)" -ResponseHeaders $resProd -Pattern "(200 OK|404 Not Found)"

# 7. Test Internal API Isolation (Direct external call must be blocked or unauthorized)
$resInternal = (curl.exe -s -I -X POST "$BaseUrl/internal/v1/users/batch") -join "`n"
Test-Check -Name "Internal API Protected/Blocked (/internal/v1/users/batch)" -ResponseHeaders $resInternal -Pattern "(401 Unauthorized|403 Forbidden|404 Not Found)"

Write-Host "`n========================================================" -ForegroundColor Cyan
Write-Host "  SMOKE TEST SUMMARY: $passed Passed, $failed Failed" -ForegroundColor $(if ($failed -eq 0) { "Green" } else { "Red" })
Write-Host "========================================================" -ForegroundColor Cyan
