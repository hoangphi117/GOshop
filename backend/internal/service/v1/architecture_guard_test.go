package v1service_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestArchitectureGuard_OrderCoreDomainIsolation verifies that Order core domain
// does not directly depend on foreign repositories (Products, Coupons, StockMovements, Carts).
// Foreign services/repositories must only be accessed via Ports & Adapters defined in order_ports.go.
func TestArchitectureGuard_OrderCoreDomainIsolation(t *testing.T) {
	files, err := filepath.Glob("./order*.go")
	require.NoError(t, err)
	require.NotEmpty(t, files)

	prohibitedTypes := []string{
		"ProductRepository",
		"CouponRepository",
		"StockMovementRepository",
		"CartRepository",
		"UserRepository",
	}

	fset := token.NewFileSet()

	for _, file := range files {
		// Adapters and tests are anti-corruption layers and allowed to wire ports
		baseName := filepath.Base(file)
		if strings.HasSuffix(baseName, "_test.go") || strings.Contains(baseName, "_adapter.go") {
			continue
		}

		node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		require.NoError(t, err, "failed to parse file: %s", file)

		ast.Inspect(node, func(n ast.Node) bool {
			// Check selector expressions like repository.ProductRepository
			if sel, ok := n.(*ast.SelectorExpr); ok {
				for _, prohibited := range prohibitedTypes {
					if sel.Sel.Name == prohibited {
						t.Errorf("Architecture violation in %s: core order file must not reference %s directly. Use OrderPorts instead.", file, prohibited)
					}
				}
			}
			return true
		})
	}
}

// TestArchitectureGuard_OtherDomainsIsolationFromOrderRepository verifies that
// other service domains (e.g. Chat) do not directly access OrderRepository/OrderItemRepository.
// Cross-domain access must go through OrderReader port.
func TestArchitectureGuard_OtherDomainsIsolationFromOrderRepository(t *testing.T) {
	domains := []string{"./chat*.go"}
	prohibitedOrderTypes := []string{
		"OrderRepository",
		"OrderItemRepository",
	}

	fset := token.NewFileSet()

	for _, pattern := range domains {
		files, err := filepath.Glob(pattern)
		require.NoError(t, err)

		for _, file := range files {
			baseName := filepath.Base(file)
			if strings.HasSuffix(baseName, "_test.go") {
				continue
			}

			node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
			require.NoError(t, err, "failed to parse file: %s", file)

			ast.Inspect(node, func(n ast.Node) bool {
				if sel, ok := n.(*ast.SelectorExpr); ok {
					for _, prohibited := range prohibitedOrderTypes {
						if sel.Sel.Name == prohibited {
							t.Errorf("Architecture violation in %s: non-order file must not reference %s directly. Use OrderReader port instead.", file, prohibited)
						}
					}
				}
				return true
			})
		}
	}
}

// TestArchitectureGuard_GolangciLinterConfig verifies that .golangci.yml enforces depguard rules.
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

	assert.Contains(t, content, "chat_domain_isolation")
	assert.Contains(t, content, "inventory_domain_isolation")
	assert.Contains(t, content, "coupon_domain_isolation")
}
