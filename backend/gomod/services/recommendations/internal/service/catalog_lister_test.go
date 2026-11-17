package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fallback reads the paginated listing envelope, not the bare array the
// search route returns: decoding the wrong shape silently yielded zero products.
func TestHTTPCatalogListerDecodesPaginatedEnvelope(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  "success",
			"message": "Products retrieved successfully",
			"data": []map[string]any{
				{
					"uuid":          "11111111-1111-1111-1111-111111111111",
					"name":          "Headphones",
					"slug":          "headphones",
					"price":         199.5,
					"stock":         7,
					"category_name": "Audio",
					"image_url":     "https://cdn/h.png",
					"rating":        4.8,
					"reviews_count": 12,
				},
			},
			"pagination": map[string]any{"page": 1, "limit": 3, "total": 1},
		})
	}))
	defer srv.Close()

	lister, err := NewHTTPCatalogLister(srv.URL, 0)
	require.NoError(t, err)

	items, err := lister.ListTopRated(context.Background(), 3)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Headphones", items[0].ProductName)
	assert.Equal(t, 4.8, items[0].ProductAvgRating)
	assert.InDelta(t, 199.5, items[0].ProductPrice, 0.001)

	// Ranking must be asked of the catalog, not re-sorted after the fact.
	assert.Contains(t, gotQuery, "order_by=product_avg_rating")
	assert.Contains(t, gotQuery, "sort=desc")
	assert.Contains(t, gotQuery, "limit=3")
}
