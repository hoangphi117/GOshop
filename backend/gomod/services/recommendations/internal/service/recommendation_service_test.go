package service

import (
	"context"
	"errors"
	"testing"

	catalogcontract "goshop/services/catalog/contract"
	recsrepository "goshop/services/recommendations/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	rows []recsrepository.BestSellerRow
	err  error
}

func (f fakeRepo) Rebuild(context.Context, int) (int, error) { return 0, nil }

func (f fakeRepo) Top(context.Context, int) ([]recsrepository.BestSellerRow, error) {
	return f.rows, f.err
}

func (f fakeRepo) UnitsByProduct(context.Context) (map[uuid.UUID]int64, int64, error) {
	units := make(map[uuid.UUID]int64, len(f.rows))
	var max int64
	for _, row := range f.rows {
		units[row.ProductUUID] = row.UnitsSold
		if row.UnitsSold > max {
			max = row.UnitsSold
		}
	}
	return units, max, f.err
}

type fakeCatalog struct {
	products []catalogcontract.ProductItemInfo
	err      error
}

func (f fakeCatalog) GetProductByUUID(context.Context, string) (*catalogcontract.ProductItemInfo, error) {
	return nil, errors.New("not used")
}

func (f fakeCatalog) GetProductByID(context.Context, int32) (*catalogcontract.ProductItemInfo, error) {
	return nil, errors.New("not used")
}

func (f fakeCatalog) GetProductBySlug(context.Context, string) (*catalogcontract.ProductItemInfo, error) {
	return nil, errors.New("not used")
}

func (f fakeCatalog) GetProductsBatch(context.Context, []string, []int32) ([]catalogcontract.ProductItemInfo, error) {
	return f.products, f.err
}

func (f fakeCatalog) ValidateProducts(context.Context, []string) (bool, []string, error) {
	return false, nil, errors.New("not used")
}

func (f fakeCatalog) GetCategoryByUUID(context.Context, string) (*catalogcontract.CategoryItemInfo, error) {
	return nil, errors.New("not used")
}

func (f fakeCatalog) ListCategories(context.Context, bool) ([]catalogcontract.CategoryItemInfo, error) {
	return nil, errors.New("not used")
}

func (f fakeCatalog) Close() error { return nil }

type fakeLister struct {
	products []catalogcontract.ProductItemInfo
	err      error
	calls    int
}

func (f *fakeLister) ListTopRated(context.Context, int) ([]catalogcontract.ProductItemInfo, error) {
	f.calls++
	return f.products, f.err
}

func rows(n int) []recsrepository.BestSellerRow {
	out := make([]recsrepository.BestSellerRow, n)
	for i := 0; i < n; i++ {
		out[i] = recsrepository.BestSellerRow{ProductUUID: uuid.New(), UnitsSold: int64(n - i)}
	}
	return out
}

// The rollup order must survive the round trip through GetProductsBatch, which
// returns rows in whatever order the database felt like.
func TestBestSellersKeepsRollupOrder(t *testing.T) {
	popular := rows(3)
	first := popular[0].ProductUUID.String()
	third := popular[2].ProductUUID.String()

	catalog := fakeCatalog{products: []catalogcontract.ProductItemInfo{
		{ProductUUID: third, ProductName: "third"},
		{ProductUUID: first, ProductName: "first"},
	}}
	svc := NewRecommendationService(fakeRepo{rows: popular}, catalog, &fakeLister{})

	shelf, err := svc.BestSellers(context.Background(), 3)
	require.NoError(t, err)
	assert.Equal(t, SourceOrders, shelf.Source)
	require.Len(t, shelf.Items, 2)
	assert.Equal(t, first, shelf.Items[0].UUID)
	assert.Equal(t, third, shelf.Items[1].UUID)
	assert.Equal(t, int64(3), shelf.Items[0].UnitsSold, "units sold must ride along")
}

func TestBestSellersFallsBackWhenRollupEmpty(t *testing.T) {
	lister := &fakeLister{products: []catalogcontract.ProductItemInfo{
		{ProductUUID: "a", ProductAvgRating: 4.9},
		{ProductUUID: "b", ProductAvgRating: 4.1},
		{ProductUUID: "c", ProductAvgRating: 3.0},
	}}
	svc := NewRecommendationService(fakeRepo{}, fakeCatalog{}, lister)

	shelf, err := svc.BestSellers(context.Background(), 2)
	require.NoError(t, err)
	assert.Equal(t, SourceFallback, shelf.Source)
	assert.Equal(t, 1, lister.calls)
	require.Len(t, shelf.Items, 2, "limit must be honoured")
}

func TestBestSellersFallsBackWhenRollupErrors(t *testing.T) {
	lister := &fakeLister{products: []catalogcontract.ProductItemInfo{{ProductUUID: "a"}}}
	svc := NewRecommendationService(fakeRepo{err: errors.New("relation does not exist")}, fakeCatalog{}, lister)

	shelf, err := svc.BestSellers(context.Background(), 12)
	require.NoError(t, err, "a broken rollup must not break the homepage")
	assert.Equal(t, SourceFallback, shelf.Source)
}

func TestBestSellersSurfacesRollupErrorWhenFallbackAlsoFails(t *testing.T) {
	rollupErr := errors.New("relation does not exist")
	svc := NewRecommendationService(
		fakeRepo{err: rollupErr},
		fakeCatalog{},
		&fakeLister{err: errors.New("catalog down")},
	)

	_, err := svc.BestSellers(context.Background(), 12)
	assert.ErrorIs(t, err, rollupErr, "the original cause is the useful one")
}
