package contract_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"goshop/services/catalog/contract"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mirrors the real handler output: ResponseSuccess puts the product list
// directly in "data" (a bare array), and dto.ProductDTO emits both snake_case
// and camelCase aliases.
const searchResponse = `{
  "status":"success",
  "message":"Products searched successfully",
  "data":[
    {"uuid":"p-1","name":"Bamboo Lantern","slug":"bamboo-lantern","price":250000,"stock":4,
     "category_name":"Lighting","image_url":"https://cdn/x.jpg","description":"Handmade.",
     "imageUrl":"https://cdn/x.jpg","categoryUuid":"c-1","avgRating":4.5},
    {"uuid":"p-2","name":"Linen Lamp","slug":"linen-lamp","price":310000,"stock":0,
     "category_name":"Lighting"}
  ]
}`

func TestHTTPProductSearchClientDecodesHandlerShape(t *testing.T) {
	var gotQuery, gotLimit, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("q")
		gotLimit = r.URL.Query().Get("limit")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(searchResponse))
	}))
	defer srv.Close()

	c, err := contract.NewHTTPProductSearchClient(srv.URL, time.Second)
	require.NoError(t, err)

	items, err := c.Search(context.Background(), "lamp for reading", 5)
	require.NoError(t, err)

	assert.Equal(t, "/api/v1/products/search", gotPath)
	assert.Equal(t, "lamp for reading", gotQuery)
	assert.Equal(t, "5", gotLimit)

	require.Len(t, items, 2)
	assert.Equal(t, "p-1", items[0].ProductUUID)
	assert.Equal(t, "Bamboo Lantern", items[0].ProductName)
	assert.Equal(t, 250000.0, items[0].ProductPrice)
	assert.Equal(t, int32(4), items[0].ProductStock)
	assert.Equal(t, "Lighting", items[0].ProductCategoryName)
	assert.Equal(t, "https://cdn/x.jpg", items[0].ProductImageURL)
	assert.Equal(t, "Handmade.", items[0].ProductDescription)

	// Optional pointers must not panic when absent.
	assert.Empty(t, items[1].ProductImageURL)
	assert.Equal(t, int32(0), items[1].ProductStock)
}

// The route the assistant actually reaches now is the gateway's hybrid search,
// which nests the list under data.items. Decoding only the flat array left the
// model with zero products — the "search isn't responding" answer.
const hybridSearchResponse = `{
  "status":"success",
  "message":"Products searched successfully",
  "data":{
    "source":"hybrid",
    "total":53,
    "items":[
      {"uuid":"p-1","name":"Nokia Pixel","slug":"nokia-pixel","price":3000000,"stock":76,
       "category_name":"Smartphones & Tablets","image_url":"https://cdn/y.jpg"}
    ]
  }
}`

func TestHTTPProductSearchClientDecodesHybridShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(hybridSearchResponse))
	}))
	defer srv.Close()

	c, err := contract.NewHTTPProductSearchClient(srv.URL, time.Second)
	require.NoError(t, err)

	items, err := c.Search(context.Background(), "phones", 5)
	require.NoError(t, err)

	require.Len(t, items, 1)
	assert.Equal(t, "p-1", items[0].ProductUUID)
	assert.Equal(t, "Nokia Pixel", items[0].ProductName)
	assert.Equal(t, 3000000.0, items[0].ProductPrice)
	assert.Equal(t, int32(76), items[0].ProductStock)
	assert.Equal(t, "Smartphones & Tablets", items[0].ProductCategoryName)
	assert.Equal(t, "https://cdn/y.jpg", items[0].ProductImageURL)
}

// An empty hybrid shelf must read as "no matches", not as an error.
func TestHTTPProductSearchClientEmptyHybridShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","data":{"source":"hybrid","items":[],"total":0}}`))
	}))
	defer srv.Close()

	c, err := contract.NewHTTPProductSearchClient(srv.URL, time.Second)
	require.NoError(t, err)

	items, err := c.Search(context.Background(), "zzz", 5)
	require.NoError(t, err)
	assert.Empty(t, items)
}

// A 5xx must surface as an error so the model can escalate instead of being
// told "no products found".
func TestHTTPProductSearchClientErrorsOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c, err := contract.NewHTTPProductSearchClient(srv.URL, time.Second)
	require.NoError(t, err)

	_, err = c.Search(context.Background(), "lamp", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestHTTPProductSearchClientDefaultsLimit(t *testing.T) {
	var gotLimit string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLimit = r.URL.Query().Get("limit")
		_, _ = w.Write([]byte(`{"status":"success","data":[]}`))
	}))
	defer srv.Close()

	c, err := contract.NewHTTPProductSearchClient(srv.URL, time.Second)
	require.NoError(t, err)

	items, err := c.Search(context.Background(), "x", 0) // out of range
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, "5", gotLimit)
}

func TestHTTPProductSearchClientRequiresBaseURL(t *testing.T) {
	_, err := contract.NewHTTPProductSearchClient("  ", time.Second)
	assert.Error(t, err)
}
