#!/usr/bin/env bash
# Post-deploy smoke check for the GOshop prod stack.
#
# Purpose: prove the edge is wired to the right service after `docker compose up`.
# The monolith extraction left several nginx paths pointing at a container that
# no longer exists (core-backend), and four services (chat, review, analytics,
# media) were missing from the prod compose file entirely. This checks the edges
# those bugs would break, without needing credentials or a seeded cart.
#
# Usage:  scripts/smoke-prod.sh [BASE_URL]        (default http://localhost)
# Exit:   0 when every check passes, 1 otherwise. Safe to run repeatedly.

set -uo pipefail

BASE_URL="${1:-http://localhost}"
failures=0

check() {
    local name="$1" url="$2" expect="${3:-200}"
    local code
    code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$url" || echo 000)"
    # $expect accepts alternatives: "401|400" passes on either code.
    if [[ "$code" =~ ^($expect)$ ]]; then
        printf '  ok    %-42s %s (%s)\n' "$name" "$code" "$url"
    else
        printf '  FAIL  %-42s got %s want %s (%s)\n' "$name" "$code" "$expect" "$url"
        failures=$((failures + 1))
    fi
}

echo "GOshop prod smoke against $BASE_URL"

# Edge itself must answer before any upstream claim means anything.
check "edge healthz" "$BASE_URL/healthz" 200

# Deep probe reports which dependency is down instead of a bare 503.
check "deep health (catalog)" "$BASE_URL/health/deep" 200

# Gateway-served paths: these 404'd when REVIEW/ANALYTICS/MEDIA env was unset.
check "catalog products via gateway" "$BASE_URL/api/v1/products?page=1&limit=1" 200
check "review route reaches gateway" "$BASE_URL/api/v1/products/00000000-0000-0000-0000-000000000000/reviews" 404

# Service-owned paths nginx proxies directly.
check "auth rejects anonymous" "$BASE_URL/api/v1/auth/does-not-exist" 404
# These routes used to answer 400 (missing X-API-KEY) since the edge injected
# it server-side; anonymous callers now clear the key gate and stop at JWT
# (401). Either code proves auth forwards correctly through the stack.
check "cart rejects anonymous" "$BASE_URL/api/v1/cart" "401|400"
check "orders rejects anonymous" "$BASE_URL/api/v1/orders" "401|400"

# Recommendations is what the storefront homepage shelf calls. It answers 200
# with a shelf (personalized or its best-seller fallback); a 502 here means the
# container is missing from the compose file or died at boot.
check "personalized shelf via gateway" "$BASE_URL/api/v1/recommendations/personalized?limit=1" 200

# Internal IPC must never be reachable from outside.
check "internal endpoints blocked" "$BASE_URL/internal/health" 403

# Every service container the prod stack claims to run must be up.
echo "container health"
for name in api1 api2 identity notification payment catalog inventory coupon cart banner review chat media analytics order worker ws-gateway storefront nginx recommendations embedder; do
    status="$(docker inspect -f '{{if .State.Running}}running{{else}}{{.State.Status}}{{end}}' "goshop-$name" 2>/dev/null || echo missing)"
    if [[ "$status" == "running" ]]; then
        printf '  ok    %-42s %s\n' "$name" "$status"
    else
        printf '  FAIL  %-42s %s\n' "$name" "$status"
        failures=$((failures + 1))
    fi
done

if (( failures > 0 )); then
    echo "SMOKE FAILED: $failures check(s)" >&2
    exit 1
fi
echo "SMOKE OK"
