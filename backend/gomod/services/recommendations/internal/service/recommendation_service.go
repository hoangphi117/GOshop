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

	catalogcontract "goshop/services/catalog/contract"
	recsdto "goshop/services/recommendations/internal/dto"
	recsrepository "goshop/services/recommendations/internal/repository"
	"goshop/shared/pkg/logger"
)

// Source says where a shelf came from. It is always returned so a caller can
// tell measured best sellers from the promotional fallback: showing promoted
// products under a "Best sellers" label without saying so would be a lie.
const (
	SourceOrders   = "orders"
	SourceFallback = "fallback_rating"
)

// Shelf is the response of GET /api/v1/recommendations.
type Shelf = recsdto.Shelf

// CatalogLister reads the public catalog listing route.
//
// CatalogClient.GetProductsBatch cannot serve the fallback: the gRPC server
// returns an empty list when both uuids and ids are empty, and the order of the
// result is not guaranteed. Rankings are a catalog concern and the listing
// route already sorts by rating, so the fallback calls it instead of adding an
// RPC.
type CatalogLister interface {
	ListTopRated(ctx context.Context, limit int) ([]catalogcontract.ProductItemInfo, error)
}

// RecommendationService builds product shelves.
type RecommendationService interface {
	// BestSellers returns the top-selling products over the popularity window.
	// When there is not enough order history it falls back to catalog ratings.
	BestSellers(ctx context.Context, limit int) (Shelf, error)
}

type recommendationService struct {
	repo     recsrepository.PopularityRepository
	catalog  catalogcontract.CatalogClient
	fallback CatalogLister
}

func NewRecommendationService(
	repo recsrepository.PopularityRepository,
	catalog catalogcontract.CatalogClient,
	fallback CatalogLister,
) RecommendationService {
	return &recommendationService{repo: repo, catalog: catalog, fallback: fallback}
}

func (s *recommendationService) BestSellers(ctx context.Context, limit int) (Shelf, error) {
	if limit <= 0 || limit > 50 {
		limit = 12
	}

	rows, err := s.repo.Top(ctx, limit)
	if err != nil {
		// A missing rollup must not break the homepage; degrade and say so.
		logger.Log.Warn().Err(err).Msg("Best sellers unavailable, falling back to ratings")
		return s.fallbackShelf(ctx, limit, err)
	}
	if len(rows) == 0 {
		return s.fallbackShelf(ctx, limit, nil)
	}

	uuids := make([]string, 0, len(rows))
	sold := make(map[string]int64, len(rows))
	for _, row := range rows {
		id := row.ProductUUID.String()
		uuids = append(uuids, id)
		sold[id] = row.UnitsSold
	}

	// GetProductsBatch does not preserve request order, so re-rank afterwards.
	products, err := s.catalog.GetProductsBatch(ctx, uuids, nil)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Catalog batch read failed for best sellers")
		return s.fallbackShelf(ctx, limit, err)
	}

	byID := make(map[string]catalogcontract.ProductItemInfo, len(products))
	for _, p := range products {
		byID[p.ProductUUID] = p
	}

	items := make([]recsdto.RecommendationItem, 0, len(uuids))
	for _, id := range uuids {
		if p, ok := byID[id]; ok {
			item := toItem(p)
			item.UnitsSold = sold[id]
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return s.fallbackShelf(ctx, limit, nil)
	}
	return Shelf{Source: SourceOrders, Items: items}, nil
}

// fallbackShelf ranks by average rating. It is explicitly labelled at the API
// boundary, so a store with no sales still shows something plausible.
func (s *recommendationService) fallbackShelf(
	ctx context.Context,
	limit int,
	cause error,
) (Shelf, error) {
	products, err := s.fallback.ListTopRated(ctx, limit)
	if err != nil {
		// Nothing left to degrade to: surface the original failure.
		if cause != nil {
			return Shelf{}, cause
		}
		return Shelf{}, err
	}
	if len(products) > limit {
		products = products[:limit]
	}
	items := make([]recsdto.RecommendationItem, 0, len(products))
	for _, p := range products {
		items = append(items, toItem(p))
	}
	return Shelf{Source: SourceFallback, Items: items}, nil
}

// toItem flattens the catalog's transport struct into the response shape.
func toItem(p catalogcontract.ProductItemInfo) recsdto.RecommendationItem {
	return recsdto.RecommendationItem{
		UUID:         p.ProductUUID,
		Name:         p.ProductName,
		Slug:         p.ProductSlug,
		Description:  p.ProductDescription,
		Price:        p.ProductPrice,
		Stock:        p.ProductStock,
		SKU:          p.ProductSKU,
		CategoryUUID: p.ProductCategoryUUID,
		CategoryName: p.ProductCategoryName,
		ImageURL:     p.ProductImageURL,
		AvgRating:    p.ProductAvgRating,
		ReviewCount:  p.ProductReviewCount,
	}
}

// HTTPCatalogLister reads GET /api/v1/products?order_by=product_avg_rating.
type HTTPCatalogLister struct {
	baseURL string
	http    *http.Client
}

// NewHTTPCatalogLister builds the adapter. baseURL is the catalog origin, e.g.
// http://catalog-service:8086 (no trailing /api).
func NewHTTPCatalogLister(baseURL string, timeout time.Duration) (*HTTPCatalogLister, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("catalog list: base URL is required")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &HTTPCatalogLister{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: timeout},
	}, nil
}

// productListEnvelope mirrors utils.ResponseSuccess for GET /api/v1/products:
// the paginated shape puts the rows under "data" and the counters under
// "pagination". dto.ProductDTO marshals both snake_case and camelCase aliases;
// only the snake_case ones are stable.
type productListEnvelope struct {
	Data []struct {
		UUID         string  `json:"uuid"`
		Name         string  `json:"name"`
		Slug         string  `json:"slug"`
		Description  *string `json:"description"`
		Price        float64 `json:"price"`
		Stock        int32   `json:"stock"`
		SKU          *string `json:"sku"`
		CategoryUUID string  `json:"category_uuid"`
		CategoryName string  `json:"category_name"`
		ImageURL     *string `json:"image_url"`
		AvgRating    float64 `json:"rating"`
		ReviewCount  int32   `json:"reviews_count"`
	} `json:"data"`
}

func (c *HTTPCatalogLister) ListTopRated(ctx context.Context, limit int) ([]catalogcontract.ProductItemInfo, error) {
	if limit <= 0 || limit > 50 {
		limit = 12
	}

	q := url.Values{}
	q.Set("order_by", "product_avg_rating")
	q.Set("sort", "desc")
	q.Set("status", "1") // active products only
	q.Set("page", "1")
	q.Set("limit", strconv.Itoa(limit))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/api/v1/products?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("catalog list: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("catalog list: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalog list: status %d", resp.StatusCode)
	}

	var env productListEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("catalog list: decode: %w", err)
	}

	out := make([]catalogcontract.ProductItemInfo, 0, len(env.Data))
	for _, p := range env.Data {
		info := catalogcontract.ProductItemInfo{
			ProductUUID:         p.UUID,
			ProductName:         p.Name,
			ProductSlug:         p.Slug,
			ProductPrice:        p.Price,
			ProductStock:        p.Stock,
			ProductCategoryUUID: p.CategoryUUID,
			ProductCategoryName: p.CategoryName,
			ProductAvgRating:    p.AvgRating,
			ProductReviewCount:  p.ReviewCount,
		}
		if p.Description != nil {
			info.ProductDescription = *p.Description
		}
		if p.SKU != nil {
			info.ProductSKU = *p.SKU
		}
		if p.ImageURL != nil {
			info.ProductImageURL = *p.ImageURL
		}
		out = append(out, info)
	}
	return out, nil
}
