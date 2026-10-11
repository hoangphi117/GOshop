# =============================================================================
# GOSHOP FULL E2E SMOKE TEST SUITE (POWERSHELL)
# Luong nghiep vu: Register -> Login -> Products -> Address -> Order (COD) -> Notification
# =============================================================================

param (
    [string]$BaseUrl = "http://localhost",
    [string]$ApiKey  = "abcd-1234-1111"
)

$ErrorActionPreference = "Stop"

Write-Host "========================================================" -ForegroundColor Cyan
Write-Host "  GOSHOP FULL E2E BUSINESS FLOW SMOKE TEST" -ForegroundColor Cyan
Write-Host "  Target: $BaseUrl" -ForegroundColor Cyan
Write-Host "========================================================" -ForegroundColor Cyan

$passed = 0
$failed = 0

function Report-Step {
    param (
        [string]$Name,
        [bool]$Success,
        [string]$Detail = ""
    )
    if ($Success) {
        Write-Host "  [PASS] $Name" -ForegroundColor Green
        if ($Detail) { Write-Host "         $Detail" -ForegroundColor DarkGray }
        $global:passed++
    } else {
        Write-Host "  [FAIL] $Name" -ForegroundColor Red
        if ($Detail) { Write-Host "         Error: $Detail" -ForegroundColor Yellow }
        $global:failed++
    }
}

$timestamp = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$testEmail = "e2e_user_${timestamp}@goshop.com"
$testPassword = "Password123!"
$testName = "Test E2E User $timestamp"

# -----------------------------------------------------------------------------
# STEP 1: DANG KY TAI KHOAN MOI (IDENTITY SERVICE)
# -----------------------------------------------------------------------------
Write-Host "`n[*] Buoc 1: Dang ky user qua Identity Service (/api/v1/auth/register)..." -ForegroundColor Yellow
try {
    $regBody = @{
        name     = $testName
        email    = $testEmail
        password = $testPassword
    } | ConvertTo-Json

    $regRes = Invoke-RestMethod -Uri "$BaseUrl/api/v1/auth/register" -Method Post -Body $regBody -ContentType "application/json"
    Report-Step -Name "Dang ky thanh cong tai khoan $testEmail" -Success $true -Detail "Status: $($regRes.status)"
} catch {
    Report-Step -Name "Dang ky tai khoan" -Success $false -Detail $_.Exception.Message
}

# Cho 1.5s de RabbitMQ Consumer dong bo user sang Core DB (Projection)
Write-Host "    -> Cho 1.5s de RabbitMQ Consumer dong bo User Projection vao Core DB..." -ForegroundColor DarkGray
Start-Sleep -Milliseconds 1500

# -----------------------------------------------------------------------------
# STEP 2: DANG NHAP VA NHAN RS256 TOKEN (IDENTITY SERVICE)
# -----------------------------------------------------------------------------
Write-Host "`n[*] Buoc 2: Dang nhap qua Identity Service (/api/v1/auth/login)..." -ForegroundColor Yellow
$accessToken = ""
try {
    $loginBody = @{
        email    = $testEmail
        password = $testPassword
    } | ConvertTo-Json

    $loginRes = Invoke-RestMethod -Uri "$BaseUrl/api/v1/auth/login" -Method Post -Body $loginBody -ContentType "application/json"
    $accessToken = $loginRes.data.access_token
    $isTokenValid = ($accessToken -ne $null -and $accessToken.Length -gt 20)
    Report-Step -Name "Dang nhap va nhan access token RS256" -Success $isTokenValid -Detail "Token length: $($accessToken.Length)"
} catch {
    Report-Step -Name "Dang nhap tai khoan" -Success $false -Detail $_.Exception.Message
}

$authHeaders = @{
    "Authorization" = "Bearer $accessToken"
    "x-Api-Key"     = $ApiKey
}

# -----------------------------------------------------------------------------
# STEP 3: LAY DANH SACH SAN PHAM CON HANG (CORE BACKEND)
# -----------------------------------------------------------------------------
Write-Host "`n[*] Buoc 3: Lay san pham tu Catalog (/api/v1/products)..." -ForegroundColor Yellow
$targetProduct = $null
try {
    $prodRes = Invoke-RestMethod -Uri "$BaseUrl/api/v1/products" -Method Get
    if ($prodRes.data -and $prodRes.data.Count -gt 0) {
        $targetProduct = $prodRes.data[0]
        Report-Step -Name "Lay danh sach san pham thanh cong" -Success $true -Detail "Chon: $($targetProduct.name) (UUID: $($targetProduct.uuid), Gia: $($targetProduct.price))"
    } else {
        Report-Step -Name "Lay danh sach san pham" -Success $false -Detail "Khong co san pham nao trong database"
    }
} catch {
    Report-Step -Name "Lay danh sach san pham" -Success $false -Detail $_.Exception.Message
}

# -----------------------------------------------------------------------------
# STEP 4: TAO DIA CHI GIAO HANG (IDENTITY SERVICE)
# -----------------------------------------------------------------------------
Write-Host "`n[*] Buoc 4: Tao dia chi giao hang (/api/v1/addresses)..." -ForegroundColor Yellow
try {
    $addrBody = @{
        label         = "Nha rieng"
        name          = $testName
        phone         = "0987654321"
        address       = "123 Xuan Thuy, Dich Vong Hau"
        province_id   = 1
        province_name = "Ha Noi"
        district_id   = 101
        district_name = "Cau Giay"
        ward_code     = "10101"
        ward_name     = "Dich Vong Hau"
        is_default    = $true
    } | ConvertTo-Json

    $addrRes = Invoke-RestMethod -Uri "$BaseUrl/api/v1/addresses" -Method Post -Body $addrBody -ContentType "application/json" -Headers $authHeaders
    Report-Step -Name "Tao dia chi giao hang thanh cong" -Success $true -Detail "Address UUID: $($addrRes.data.uuid)"
} catch {
    $errDetail = $_.Exception.Message
    if ($_.ErrorDetails) { $errDetail += " | " + $_.ErrorDetails.Message }
    Report-Step -Name "Tao dia chi giao hang" -Success $false -Detail $errDetail
}

# -----------------------------------------------------------------------------
# STEP 5: TAO DON HANG COD (CORE BACKEND)
# -----------------------------------------------------------------------------
Write-Host "`n[*] Buoc 5: Dat hang COD (/api/v1/orders)..." -ForegroundColor Yellow
$createdOrderUUID = ""
try {
    $orderBody = @{
        items = @(
            @{
                product_uuid = $targetProduct.uuid
                quantity     = 1
            }
        )
        shipping_name    = $testName
        shipping_phone   = "0987654321"
        shipping_address = "123 Xuan Thuy, Dich Vong Hau, Cau Giay, Ha Noi"
        shipping_fee     = 30000
        payment_method   = 1 # COD
        note             = "Giao gio hanh chinh - E2E Smoke Test"
    } | ConvertTo-Json -Depth 4

    $orderRes = Invoke-RestMethod -Uri "$BaseUrl/api/v1/orders" -Method Post -Body $orderBody -ContentType "application/json" -Headers $authHeaders
    $createdOrderUUID = $orderRes.data.uuid
    Report-Step -Name "Tao don hang thanh cong (Fix order_user_uuid verified)" -Success ($createdOrderUUID -ne $null) -Detail "Order Number: $($orderRes.data.order_number), Total: $($orderRes.data.total_amount)"
} catch {
    $errDetail = $_.Exception.Message
    if ($_.ErrorDetails) {
        $errDetail += " | Body: " + $_.ErrorDetails.Message
    }
    Report-Step -Name "Tao don hang COD" -Success $false -Detail $errDetail
}

# -----------------------------------------------------------------------------
# STEP 6: KIEM TRA CHI TIET DON HANG (CORE BACKEND)
# -----------------------------------------------------------------------------
Write-Host "`n[*] Buoc 6: Kiem tra chi tiet don hang (/api/v1/orders/$createdOrderUUID)..." -ForegroundColor Yellow
if ($createdOrderUUID) {
    try {
        $getRes = Invoke-RestMethod -Uri "$BaseUrl/api/v1/orders/$createdOrderUUID" -Method Get -Headers $authHeaders
        $isMatch = ($getRes.data.uuid -eq $createdOrderUUID)
        Report-Step -Name "Doc chi tiet don hang da tao" -Success $isMatch -Detail "Status: $($getRes.data.status), Payment: $($getRes.data.payment_method)"
    } catch {
        Report-Step -Name "Doc chi tiet don hang" -Success $false -Detail $_.Exception.Message
    }
} else {
    Report-Step -Name "Doc chi tiet don hang" -Success $false -Detail "Bo qua vi buoc 5 that bai"
}

# -----------------------------------------------------------------------------
# STEP 7: KIEM TRA THONG BAO CUA USER (CORE BACKEND)
# -----------------------------------------------------------------------------
Write-Host "`n[*] Buoc 7: Kiem tra danh sach thong bao (/api/v1/notifications/me)..." -ForegroundColor Yellow
try {
    # Cho 1.5s de OrderConsumer gui thong bao qua RabbitMQ
    Start-Sleep -Milliseconds 1500
    $notifRes = Invoke-RestMethod -Uri "$BaseUrl/api/v1/notifications/me" -Method Get -Headers $authHeaders
    Report-Step -Name "Doc danh sach thong bao cua user" -Success $true -Detail "Ket qua: $($notifRes.status)"
} catch {
    Report-Step -Name "Doc danh sach thong bao" -Success $false -Detail $_.Exception.Message
}

# -----------------------------------------------------------------------------
# TONG KET
# -----------------------------------------------------------------------------
Write-Host "`n========================================================" -ForegroundColor Cyan
Write-Host "  KET QUA SMOKE TEST E2E: $passed Passed, $failed Failed" -ForegroundColor $(if ($failed -eq 0) { "Green" } else { "Red" })
Write-Host "========================================================" -ForegroundColor Cyan

if ($failed -gt 0) {
    exit 1
}
exit 0
