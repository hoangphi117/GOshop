package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	analyticsdto "goshop/services/analytics/internal/dto"
	catalogcontract "goshop/services/catalog/contract"
	inventorycontract "goshop/services/inventory/contract"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Self-check for the cross-domain composition paths that no longer run in SQL:
// identity customer counts plus low-stock catalog enrichment. Only the calls a
// test configures are exercised; any other repository call panics via the nil
// embedded interface.

func numeric(f float64) pgtype.Numeric {
	// Scan(float64) leaves Int/Exp unset; Scan of the decimal string the
	// database would send does set them.
	n := pgtype.Numeric{}
	_ = n.Scan(strconv.FormatFloat(f, 'f', 2, 64))
	return n
}

type stubCatalog struct {
	products []catalogcontract.ProductItemInfo
}

func (s stubCatalog) GetProductsBatch(_ context.Context, _ []string, ids []int32) ([]catalogcontract.ProductItemInfo, error) {
	want := make(map[int32]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	out := make([]catalogcontract.ProductItemInfo, 0, len(ids))
	for _, p := range s.products {
		if want[p.ProductID] {
			out = append(out, p)
		}
	}
	return out, nil
}

type stubInventory struct {
	items []inventorycontract.StockItemInfo
	total int64
}

func (s stubInventory) ListLowStockProducts(context.Context, int32, int32) ([]inventorycontract.StockItemInfo, int64, error) {
	return s.items, s.total, nil
}

func TestGetOverview_ComposesCustomerCounts(t *testing.T) {
	svc := &analyticsService{
		repo: overviewRepo{revenue: 100, prevRevenue: 50, orders: 4, prevOrders: 2},
		customers: counterFunc(func(_ context.Context, from, _ time.Time) (int64, error) {
			if from.Before(time.Now().Add(-45 * 24 * time.Hour)) {
				return 6, nil // previous window
			}
			return 12, nil // current window
		}),
	}

	res, err := svc.GetOverview(context.Background(), analyticsdto.OverviewStatsQuery{})
	if err != nil {
		t.Fatalf("GetOverview: %v", err)
	}
	if res.NewCustomers != 12 || res.CustomerGrowth != 100 || res.RevenueGrowth != 100 {
		t.Fatalf("unexpected overview %+v", res)
	}
}

func TestGetLowStockProducts_EnrichesFromCatalog(t *testing.T) {
	svc := &analyticsService{
		catalog: stubCatalog{products: []catalogcontract.ProductItemInfo{{
			ProductID: 3, ProductUUID: uuid.New().String(), ProductName: "Mug", ProductSlug: "mug",
			ProductPrice: 12.5, ProductCategoryName: "Mugs",
		}}},
		inventory: stubInventory{
			items: []inventorycontract.StockItemInfo{{ProductID: 3, AvailableQuantity: 0}},
			total: 4,
		},
	}

	res, err := svc.GetLowStockProducts(context.Background(), analyticsdto.LowStockProductsQuery{})
	if err != nil {
		t.Fatalf("GetLowStockProducts: %v", err)
	}
	if res.Threshold != 5 || res.TotalLow != 4 || len(res.Items) != 1 {
		t.Fatalf("unexpected response %+v", res)
	}
	if res.Items[0].AlertLevel != "critical" || res.Items[0].Name != "Mug" {
		t.Fatalf("unexpected item %+v", res.Items[0])
	}
}
