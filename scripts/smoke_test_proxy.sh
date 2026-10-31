#!/bin/bash
# =============================================================================
# GOSHOP REVERSE PROXY & LOAD BALANCER SMOKE TEST SUITE (BASH / CURL)
# =============================================================================

set -e

BASE_URL="${1:-http://localhost}"

echo "========================================================"
echo "  GOSHOP MICROSERVICES SMOKE & ROUTING TEST"
echo "  Target Base URL: $BASE_URL"
echo "========================================================"

PASSED=0
FAILED=0

check() {
  local name="$1"
  local status="$2"
  local expected="$3"

  if [[ "$status" =~ $expected ]]; then
    echo "  [PASS] $name (HTTP $status)"
    ((PASSED++))
  else
    echo "  [FAIL] $name (Got HTTP $status, Expected $expected)"
    ((FAILED++))
  fi
}

echo ""
echo "[*] 1. Testing Static File Offloading & Security Headers..."
UPLOAD_RES=$(curl -s -I "$BASE_URL/uploads/test.jpg" || true)
UPLOAD_STATUS=$(echo "$UPLOAD_RES" | head -n 1 | cut -d$' ' -f2)
check "Static Upload Header Check" "$UPLOAD_STATUS" "200|404"

echo ""
echo "[*] 2. Testing WebSocket Upgrade Handshake..."
WS_RES=$(curl -s -I -N \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
  -H "Sec-WebSocket-Version: 13" \
  "$BASE_URL/api/v1/ws" || true)
WS_STATUS=$(echo "$WS_RES" | head -n 1 | cut -d$' ' -f2)
check "WebSocket Upgrade Route" "$WS_STATUS" "101|400|401"

echo ""
echo "[*] 3. Testing Core Backend REST API Proxying..."
API_RES=$(curl -s -I "$BASE_URL/api/v1/categories" || true)
API_STATUS=$(echo "$API_RES" | head -n 1 | cut -d$' ' -f2)
check "REST API Proxy Route" "$API_STATUS" "200|401|403"

echo ""
echo "[*] 4. Testing Identity Service Reverse Proxy..."
AUTH_RES=$(curl -s -I -X POST "$BASE_URL/api/v1/auth/login" || true)
AUTH_STATUS=$(echo "$AUTH_RES" | head -n 1 | cut -d$' ' -f2)
check "Identity Service Route (/api/v1/auth/login)" "$AUTH_STATUS" "400|422|200"

echo ""
echo "[*] 5. Testing Products Catalog Route..."
PROD_RES=$(curl -s -I "$BASE_URL/api/v1/products" || true)
PROD_STATUS=$(echo "$PROD_RES" | head -n 1 | cut -d$' ' -f2)
check "Products Catalog Route (/api/v1/products)" "$PROD_STATUS" "200|404"

echo ""
echo "[*] 6. Testing Internal API Isolation..."
INT_RES=$(curl -s -I -X POST "$BASE_URL/internal/v1/users/batch" || true)
INT_STATUS=$(echo "$INT_RES" | head -n 1 | cut -d$' ' -f2)
check "Internal API Protected/Blocked" "$INT_STATUS" "401|403|404"

echo ""
echo "========================================================"
echo "  SUMMARY: $PASSED Passed, $FAILED Failed"
echo "========================================================"

if [ "$FAILED" -gt 0 ]; then
  exit 1
fi
