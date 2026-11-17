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
	// SourceTrending is today's demand: the tier between the personal shelves
	// and the all-time list, for a visitor nothing is known about.
	SourceTrending = "trending"
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
	// Trending returns what sold in the last day, or a short shelf when today
	// has no sales yet. reported=false means the window was empty, which the
	// caller answers with BestSellers — a trending row of two items is worse
	// than an all-time row of twelve.
	Trending(ctx context.Context, limit int) (Shelf, bool, error)
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

	items, err := s.ranked(ctx, rows, limit)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Catalog batch read failed for best sellers")
		return s.fallbackShelf(ctx, limit, err)
	}
	if len(items) == 0 {
		return s.fallbackShelf(ctx, limit, nil)
	}
	return Shelf{Source: SourceOrders, Items: items}, nil
}

// TrendingShelfWindow is how far back "trending" looks. One day: long enough
// that a quiet morning still has yesterday evening in it, short enough that the
// row changes daily instead of settling into the best-seller list.
const TrendingShelfWindow = 1

// TrendingMinimum is the share of a row below which the trending shelf is not
// worth serving. Two sales today is a coincidence, not a trend, and a two-item
// row under a "Trending today" heading looks broken next to an eleven-item
// best-seller row. Below this the caller gets reported=false and shows the
// all-time list instead.
const TrendingMinimum = 2

func (s *recommendationService) Trending(ctx context.Context, limit int) (Shelf, bool, error) {
	if limit <= 0 || limit > 50 {
		limit = 12
	}

	rows, err := s.repo.Trending(ctx, TrendingShelfWindow, limit)
	if err != nil {
		// Trending is an enhancement over best sellers, not a requirement: a
		// mirror that has not migrated yet must not blank the row.
		logger.Log.Warn().Err(err).Msg("Trending unavailable; caller falls back")
		return Shelf{}, false, nil
	}
	if len(rows) < min(limit, TrendingMinimum) {
		return Shelf{}, false, nil
	}

	items, err := s.ranked(ctx, rows, limit)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Catalog batch read failed for trending")
		return Shelf{}, false, nil
	}
	if len(items) == 0 {
		return Shelf{}, false, nil
	}
	return Shelf{Source: SourceTrending, Items: items}, true, nil
}

// ranked hydrates ranked ids through the catalog and keeps the input order.
//
// GetProductsBatch does not preserve request order, so every ranked shelf has to
// re-rank afterwards; and it cannot describe archived products, so the result
// may be shorter than the ranking. Both are the reason this exists once instead
// of in every caller.
func (s *recommendationService) ranked(
	ctx context.Context,
	rows []recsrepository.BestSellerRow,
	limit int,
) ([]recsdto.RecommendationItem, error) {
	uuids := make([]string, 0, len(rows))
	sold := make(map[string]int64, len(rows))
	for _, row := range rows {
		id := row.ProductUUID.String()
		uuids = append(uuids, id)
		sold[id] = row.UnitsSold
	}

	products, err := s.catalog.GetProductsBatch(ctx, uuids, nil)
	if err != nil {
		return nil, fmt.Errorf("catalog batch: %w", err)
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
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
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
