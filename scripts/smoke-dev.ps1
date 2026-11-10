# Smoke test for the dev edge: every gateway-proxied route must be reachable.
#
#   powershell -ExecutionPolicy Bypass -File scripts/smoke-dev.ps1
#
# Why this exists: the api gateway fans out to one microservice per *_SERVICE_URL
# env var, but a service present in docker-compose.prod.yml and missing from
# docker-compose.dev.yml produces no error until a request arrives. The symptom
# is a bare 502 from nginx (DNS: lookup analytics: no such host). Nothing in the
# build catches it, so this hits one path per upstream and fails on 502 only.
#
# 502 is the only failure status checked on purpose: 401/403/404/405 all mean an
# upstream answered, which is what this script verifies. Routes are called
# through the edge (default http://localhost) because nginx injects X-API-KEY
# and answers preflight itself.
param(
  [string]$BaseUrl = "http://localhost",
  [int]$TimeoutSeconds = 8,
  [int]$Retries = 3
)

# One representative path per proxied upstream. Keep in sync with
# backend/gomod/cmd/api/routes/routes.go: each pair there is a health probe for
# one *_SERVICE_URL.
$routes = @(
  @{ Path = "/api/v1/auth/login";                   Upstream = "identity" },
  @{ Path = "/api/v1/users/me";                     Upstream = "identity" },
  @{ Path = "/api/v1/addresses";                    Upstream = "identity" },
  @{ Path = "/api/v1/admin/users";                  Upstream = "identity" },
  @{ Path = "/api/v1/notifications";                Upstream = "notification" },
  @{ Path = "/api/v1/admin/notifications/unread";   Upstream = "notification" },
  @{ Path = "/api/v1/payments";                     Upstream = "payment" },
  @{ Path = "/api/v1/admin/payments";               Upstream = "payment" },
  @{ Path = "/api/v1/coupons";                      Upstream = "coupon" },
  @{ Path = "/api/v1/admin/coupons";                Upstream = "coupon" },
  @{ Path = "/api/v1/inventory";                    Upstream = "inventory" },
  @{ Path = "/api/v1/admin/inventory";              Upstream = "inventory" },
  @{ Path = "/api/v1/categories";                   Upstream = "catalog" },
  @{ Path = "/api/v1/products";                     Upstream = "catalog" },
  @{ Path = "/api/v1/admin/products";               Upstream = "catalog" },
  @{ Path = "/api/v1/admin/categories";             Upstream = "catalog" },
  @{ Path = "/api/v1/cart";                         Upstream = "cart" },
  @{ Path = "/api/v1/banners";                      Upstream = "banner" },
  @{ Path = "/api/v1/admin/banners";                Upstream = "banner" },
  @{ Path = "/api/v1/orders";                       Upstream = "order" },
  @{ Path = "/api/v1/admin/orders";                 Upstream = "order" },
  @{ Path = "/api/v1/admin/returns";                Upstream = "order" },
  @{ Path = "/api/v1/reviews";                      Upstream = "review" },
  @{ Path = "/api/v1/admin/reviews";                Upstream = "review" },
  @{ Path = "/api/v1/admin/analytics/overview";     Upstream = "analytics" },
  @{ Path = "/api/v1/admin/analytics/revenue-chart"; Upstream = "analytics" },
  @{ Path = "/api/v1/chat/shop-status";             Upstream = "chat" },
  @{ Path = "/api/v1/upload/presign";               Upstream = "media" },
  @{ Path = "/api/v1/ws?token=smoke";               Upstream = "ws-gateway" }
)

# curl.exe, not Invoke-WebRequest: IWR hangs on some Windows localhost setups
# (proxy/IPv6 resolution) while curl returns instantly. See check-kafka-health.ps1.
$failures = @()
foreach ($r in $routes) {
  $url = "$BaseUrl$($r.Path)"
  $code = "000"
  # Retry: a freshly started container answers 502 for a few seconds while the
  # DNS entry propagates and the process binds.
  for ($i = 0; $i -lt $Retries; $i++) {
    $out = & curl.exe -s -o NUL -m $TimeoutSeconds -w "%{http_code}" $url 2>$null
    $code = if ($out) { $out.Trim() } else { "000" }
    if ($code -ne "502" -and $code -ne "000") { break }
    if ($i -lt $Retries - 1) { Start-Sleep -Seconds 3 }
  }

  # 000 means no TCP connection at all: the edge itself is down, report it once.
  if ($code -eq "000") {
    Write-Host ("{0,-6} {1,-38} {2}" -f "DEAD", $r.Path, $r.Upstream) -ForegroundColor Red
    $failures += "$($r.Path) ($($r.Upstream)): edge unreachable at $BaseUrl"
    continue
  }

  if ($code -eq "502") {
    Write-Host ("{0,-6} {1,-38} {2}" -f "502", $r.Path, $r.Upstream) -ForegroundColor Red
    $failures += "$($r.Path) -> $($r.Upstream): 502, service missing from docker-compose.dev.yml or not started"
  } else {
    Write-Host ("{0,-6} {1,-38} {2}" -f $code, $r.Path, $r.Upstream) -ForegroundColor DarkGray
  }
}

if ($failures.Count -gt 0) {
  Write-Host ""
  Write-Host "$($failures.Count) route(s) unreachable:" -ForegroundColor Red
  $failures | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
  exit 1
}

Write-Host ""
Write-Host "All $($routes.Count) gateway routes reachable" -ForegroundColor Green
