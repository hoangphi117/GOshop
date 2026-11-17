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

// productSearchItem is one element of the search response. Each field is
// decoded on its own because dto.ProductDTO has a custom MarshalJSON that emits
// BOTH snake_case and camelCase aliases; only the snake_case ones are stable.
type productSearchItem struct {
	UUID         string  `json:"uuid"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	Price        float64 `json:"price"`
	Stock        int32   `json:"stock"`
	CategoryName string  `json:"category_name"`
	ImageURL     *string `json:"image_url"`
	Description  *string `json:"description"`
}

// productSearchEnvelope accepts both shapes the route has served under
// {"status","message","data":...}:
//
//   - hybrid search (recommendations): data = {"source","items":[...],"total"}
//   - catalog keyword listing:          data = [...]
//
// Decoding only one of them silently yields zero items, which reaches the model
// as "the search isn't responding".
type productSearchEnvelope struct {
	Data json.RawMessage `json:"data"`
}

func (e productSearchEnvelope) items() ([]productSearchItem, error) {
	if len(e.Data) == 0 || string(e.Data) == "null" {
		return nil, nil
	}
	if e.Data[0] == '[' {
		var flat []productSearchItem
		if err := json.Unmarshal(e.Data, &flat); err != nil {
			return nil, err
		}
		return flat, nil
	}
	var nested struct {
		Items []productSearchItem `json:"items"`
	}
	if err := json.Unmarshal(e.Data, &nested); err != nil {
		return nil, err
	}
	return nested.Items, nil
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
	raw, err := env.items()
	if err != nil {
		return nil, fmt.Errorf("catalog search: decode data: %w", err)
	}
	if raw == nil {
		return nil, nil
	}

	out := make([]ProductItemInfo, 0, len(raw))
	for _, p := range raw {
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
