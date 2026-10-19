package app_test

import (
	"context"
	"testing"

	"goshop/internal/app"
	"goshop/internal/testutils"
	couponcontract "goshop/services/coupon/contract"
	inventorycontract "goshop/services/inventory/contract"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCouponClient struct{}

func (m *mockCouponClient) PreviewCoupon(ctx context.Context, code string, orderSubtotal float64, userUUID string) (*couponcontract.PreviewCouponResult, error) {
	return nil, nil
}
func (m *mockCouponClient) ReserveCoupon(ctx context.Context, input couponcontract.ReserveCouponInput) (string, error) {
	return "", nil
}
func (m *mockCouponClient) ReleaseCoupon(ctx context.Context, input couponcontract.ReleaseCouponInput) error {
	return nil
}
func (m *mockCouponClient) Close() error {
	return nil
}

type mockInventoryClient struct{}

func (m *mockInventoryClient) ReserveStock(ctx context.Context, input inventorycontract.ReserveStockInput) ([]inventorycontract.StockItemInfo, error) {
	return nil, nil
}
func (m *mockInventoryClient) ReleaseStock(ctx context.Context, input inventorycontract.ReleaseStockInput) error {
	return nil
}
func (m *mockInventoryClient) GetStock(ctx context.Context, productUUIDs []string, productIDs []int32) ([]inventorycontract.StockItemInfo, error) {
	return nil, nil
}
func (m *mockInventoryClient) AdjustStock(ctx context.Context, input inventorycontract.AdjustStockInput) (*inventorycontract.AdjustStockResult, error) {
	return nil, nil
}
func (m *mockInventoryClient) Close() error {
	return nil
}

func TestNewOrderModule_NilContext(t *testing.T) {
	module, err := app.NewOrderModule(nil, app.OrderModuleDeps{})
	assert.Error(t, err)
	assert.Nil(t, module)
	assert.Contains(t, err.Error(), "ModuleContext is required")
}

func TestNewOrderModule_MissingCouponClient(t *testing.T) {
	ctx := &app.ModuleContext{
		InventoryClient: &mockInventoryClient{},
	}
	module, err := app.NewOrderModule(ctx, app.OrderModuleDeps{})
	assert.Error(t, err)
	assert.Nil(t, module)
	assert.Contains(t, err.Error(), "coupon client is required")
}

func TestNewOrderModule_MissingInventoryClient(t *testing.T) {
	ctx := &app.ModuleContext{
		CouponClient: &mockCouponClient{},
	}
	module, err := app.NewOrderModule(ctx, app.OrderModuleDeps{})
	assert.Error(t, err)
	assert.Nil(t, module)
	assert.Contains(t, err.Error(), "inventory client is required")
}

func TestNewOrderModule_Success(t *testing.T) {
	ctx := &app.ModuleContext{
		CouponClient:    &mockCouponClient{},
		InventoryClient: &mockInventoryClient{},
		TxManager:       testutils.NewNoopTxManager(),
	}
	module, err := app.NewOrderModule(ctx, app.OrderModuleDeps{})
	require.NoError(t, err)
	require.NotNil(t, module)
	assert.NotEmpty(t, module.Routes())
	assert.NotEmpty(t, module.AdminRoutes())
}
