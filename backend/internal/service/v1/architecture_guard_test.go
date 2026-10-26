package v1service_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestArchitectureGuard_GolangciLinterConfig verifies that .golangci.yml enforces depguard rules.
//
// The AST-based guards that lived here scanned ./order*.go and ./chat*.go inside
// internal/service/v1. Both file families now live in their own services
// (services/order, services/chat), so depguard in .golangci.yml is the single
// enforcement point for cross-domain isolation.
func TestArchitectureGuard_GolangciLinterConfig(t *testing.T) {
	configBytes, err := os.ReadFile("../../../.golangci.yml")
	require.NoError(t, err)
	content := string(configBytes)

	assert.Contains(t, content, "order_domain_isolation")
	assert.Contains(t, content, "goshop/internal/repository/product_repository")
	assert.Contains(t, content, "goshop/internal/repository/coupons_repository")
	assert.Contains(t, content, "goshop/internal/repository/stock_movement_repository")
	assert.Contains(t, content, "goshop/internal/repository/cart_repository")
	assert.Contains(t, content, "goshop/internal/repository/user_repository")

	assert.Contains(t, content, "inventory_domain_isolation")
	assert.Contains(t, content, "coupon_domain_isolation")
	assert.Contains(t, content, "monolith_core_isolation")
	assert.Contains(t, content, "pkg: \"goshop/internal/repository\"")
	assert.Contains(t, content, "pkg: \"goshop/internal/db/sqlc\"")
}
