package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"goshop/shared/pkg/logger"
)

// SearchNarrowing is the shopper's explicit facet selection. It is carried
// separately from FacetFilter because the two halves of a hybrid search can
// honour different amounts of it: the keyword index can filter a department but
// not a price, and the mirrored catalog row can filter all three. Each side
// applies what it can and the post-fusion check applies the rest.
type SearchNarrowing struct {
	CategoryUUID uuid.UUID
	MinPrice     float64
	MaxPrice     float64
	MinRating    float64
}

// ProductSuggester is the keyword half of hybrid search: Meilisearch, read
// through the catalog service's suggestion route.
//
// It is deliberately the *suggestion* route and not the listing route. The
// listing route paginates and sorts for a human, but fusion needs a plain
// relevance order of ids, and suggestions return exactly that without a second
// filter grammar to keep in sync.
type ProductSuggester interface {
	// Suggest returns matching ids best-first, restricted to narrow. A narrowing
	// it cannot express (price, rating) is left out of the keyword filter on
	// purpose; dropping hits there is the vector side's and the active check's
	// job, and dropping them twice would only shrink the pool.
	Suggest(ctx context.Context, query string, narrow SearchNarrowing, limit int) ([]string, error)
}

// catalogSuggester is the keyword half of hybrid search.
//
// It reads the catalog service's own suggestion route rather than the public
// listing route: fusion needs a plain relevance-ordered id list, and that route
// already owns Meilisearch's settings, typo tolerance and filter, so hybrid
// search cannot drift from what the autocomplete shows for the same prefix.
type catalogSuggester struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewCatalogSuggester returns nil when no catalog URL is configured, which
// degrades search to its vector half instead of failing to start.
//
// apiKey is used only by tests and local runs against the catalog directly. In
// the real topology the URL points at the gateway, which injects the key for
// this one route; callers with no key are therefore the normal case.
func NewCatalogSuggester(baseURL, apiKey string) ProductSuggester {
	if strings.TrimSpace(baseURL) == "" {
		return nil
	}
	return &catalogSuggester{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		// Shorter than the request's own budget: a slow keyword side must not
		// be able to hold the page open when the vector side could answer.
		client: &http.Client{Timeout: 3 * time.Second},
	}
}

// meiliFilter renders the narrowing as the Meilisearch filter grammar the index
// declares. product_price is declared filterable; product_avg_rating is not, so
// asking for it here would make Meilisearch reject the whole query and silently
// demote every filtered search to its vector half.
func (n SearchNarrowing) meiliFilter() string {
	var parts []string
	if n.CategoryUUID != uuid.Nil {
		parts = append(parts, "category_uuid = '"+n.CategoryUUID.String()+"'")
	}
	if n.MinPrice > 0 {
		parts = append(parts, fmt.Sprintf("product_price >= %v", n.MinPrice))
	}
	if n.MaxPrice > 0 {
		parts = append(parts, fmt.Sprintf("product_price <= %v", n.MaxPrice))
	}
	return strings.Join(parts, " AND ")
}

// suggestionEnvelope mirrors utils.APIResponse for the one field we read.
// data is a list of suggestion objects each carrying `uuid`.
type suggestionEnvelope struct {
	Data []struct {
		UUID string `json:"uuid"`
	} `json:"data"`
}

func (c *catalogSuggester) Suggest(ctx context.Context, query string, narrow SearchNarrowing, limit int) ([]string, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("limit", strconv.Itoa(limit))
	if filter := narrow.meiliFilter(); filter != "" {
		params.Set("filter", filter)
	}
	endpoint := c.baseURL + "/api/v1/products/suggest?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("suggest request: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-KEY", c.apiKey)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("suggest: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("suggest: status %d", resp.StatusCode)
	}

	var env suggestionEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("suggest decode: %w", err)
	}
	out := make([]string, 0, len(env.Data))
	for _, item := range env.Data {
		if item.UUID != "" {
			out = append(out, item.UUID)
		}
	}
	logger.Log.Debug().
		Str("q", query).
		Str("filter", narrow.meiliFilter()).
		Int("hits", len(out)).
		Msg("hybrid search keyword side")
	return out, nil
}
