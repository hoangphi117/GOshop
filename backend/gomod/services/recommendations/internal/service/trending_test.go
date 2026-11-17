package service

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	catalogcontract "goshop/services/catalog/contract"
	recsrepository "goshop/services/recommendations/internal/repository"
)

var errFake = errors.New("fake failure")

// Trending is an optional tier: every path that cannot produce a real trend must
// say so, because the caller then shows the all-time list under a different
// heading. Reporting a two-item "trend" or an error would either look broken or
// blank the row.
func TestTrendingReportsFalseWhenTheDayIsQuiet(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		repo     fakeRepo
		catalog  fakeCatalog
		wantOK   bool
		whyShort string
	}{
		{
			name:     "no sales today",
			repo:     fakeRepo{},
			wantOK:   false,
			whyShort: "an empty window is not a trend",
		},
		{
			name:     "one sale today",
			repo:     fakeRepo{trending: []recsrepository.BestSellerRow{{ProductUUID: uuid.New(), UnitsSold: 1}}},
			wantOK:   false,
			whyShort: "a single sale is a coincidence, not a trend",
		},
		{
			name:     "trending read failed",
			repo:     fakeRepo{trendingErr: errFake},
			wantOK:   false,
			whyShort: "a broken mirror degrades to best sellers, it does not error",
		},
		{
			name: "enough sales but the catalog cannot describe any",
			repo: fakeRepo{trending: []recsrepository.BestSellerRow{
				{ProductUUID: uuid.New(), UnitsSold: 5},
				{ProductUUID: uuid.New(), UnitsSold: 4},
			}},
			catalog:  fakeCatalog{},
			wantOK:   false,
			whyShort: "an all-archived trend is not worth a heading",
		},
		{
			name: "enough sales and the catalog describes them",
			repo: fakeRepo{trending: []recsrepository.BestSellerRow{
				{ProductUUID: uuid.MustParse("11111111-1111-4111-8111-111111111111"), UnitsSold: 5},
				{ProductUUID: uuid.MustParse("22222222-2222-4222-8222-222222222222"), UnitsSold: 4},
			}},
			catalog: fakeCatalog{products: []catalogcontract.ProductItemInfo{
				{ProductUUID: "11111111-1111-4111-8111-111111111111", ProductName: "A"},
				{ProductUUID: "22222222-2222-4222-8222-222222222222", ProductName: "B"},
			}},
			wantOK:   true,
			whyShort: "a real trend must be served so the caller can label it",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			svc := NewRecommendationService(c.repo, c.catalog, &fakeLister{})
			shelf, ok, err := svc.Trending(t.Context(), 12)
			if err != nil {
				t.Fatalf("trending must not error: %v (%s)", err, c.whyShort)
			}
			if ok != c.wantOK {
				t.Fatalf("reported=%v, want %v (%s)", ok, c.wantOK, c.whyShort)
			}
			if ok && shelf.Source != SourceTrending {
				t.Fatalf("source=%q, want %q", shelf.Source, SourceTrending)
			}
			if ok && len(shelf.Items) == 0 {
				t.Fatal("reported a trend with no items")
			}
		})
	}
}

// The caller labels a shelf from its source, so the tier that produced it must
// never borrow another tier's name. trending under "orders" would show today's
// winners as all-time best sellers.
func TestTrendingSourceIsDistinct(t *testing.T) {
	t.Parallel()

	for _, other := range []string{SourceOrders, SourceFallback, SourcePersonalized, SourcePersonalizedSession} {
		if SourceTrending == other {
			t.Fatalf("trending source collides with %q", other)
		}
	}
}
