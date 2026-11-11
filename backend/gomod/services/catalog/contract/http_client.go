package contract

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// HTTPProductSearchClient reads the public catalog search route.
//
// It exists because CatalogService (gRPC) exposes no free-text query: search
// lives in Meilisearch behind GET /api/v1/products/search. Calling that route
// reuses the storefront's ranking and the product_status = 1 filter instead of
// adding an RPC and three regenerated proto files for one query.
type HTTPProductSearchClient struct {
	baseURL string
	http    *http.Client
}

// NewHTTPProductSearchClient builds the adapter. baseURL is the catalog origin,
// e.g. http://catalog-service:8086 (no trailing /api).
func NewHTTPProductSearchClient(baseURL string, timeout time.Duration) (*HTTPProductSearchClient, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("catalog search: base URL is required")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &HTTPProductSearchClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: timeout},
	}, nil
}

// productSearchEnvelope mirrors utils.ResponseSuccess for GET
// /api/v1/products/search, whose handler wraps the list in
// {"status","message","data":[...]}. It is a bare array, not {data,pagination} —
// decoding it as the nested shape silently produced zero results, so every
// product question fell back to "the product search isn't working".
//
// Each element is decoded separately because dto.ProductDTO has a custom
// MarshalJSON that emits BOTH snake_case and camelCase aliases; only the
// snake_case ones are stable, so the struct below reads those.
type productSearchEnvelope struct {
	Data []struct {
		UUID         string  `json:"uuid"`
		Name         string  `json:"name"`
		Slug         string  `json:"slug"`
		Price        float64 `json:"price"`
		Stock        int32   `json:"stock"`
		CategoryName string  `json:"category_name"`
		ImageURL     *string `json:"image_url"`
		Description  *string `json:"description"`
	} `json:"data"`
}

// Search returns at most limit products matching the free-text query.
// A non-200 response is an error, not an empty list: the assistant must be able
// to tell "no matches" from "catalog is down".
func (c *HTTPProductSearchClient) Search(ctx context.Context, query string, limit int) ([]ProductItemInfo, error) {
	if limit <= 0 || limit > 20 {
		limit = 5
	}

	q := url.Values{}
	q.Set("q", query)
	q.Set("limit", strconv.Itoa(limit))
	q.Set("page", "1")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/api/v1/products/search?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("catalog search: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("catalog search: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalog search: status %d", resp.StatusCode)
	}

	var env productSearchEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("catalog search: decode: %w", err)
	}
	if env.Data == nil {
		return nil, nil
	}

	out := make([]ProductItemInfo, 0, len(env.Data))
	for _, p := range env.Data {
		info := ProductItemInfo{
			ProductUUID:         p.UUID,
			ProductName:         p.Name,
			ProductSlug:         p.Slug,
			ProductPrice:        p.Price,
			ProductStock:        p.Stock,
			ProductCategoryName: p.CategoryName,
		}
		if p.Description != nil {
			info.ProductDescription = *p.Description
		}
		if p.ImageURL != nil {
			info.ProductImageURL = *p.ImageURL
		}
		out = append(out, info)
	}
	return out, nil
}
