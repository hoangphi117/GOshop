package service

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"

	recsdto "goshop/services/recommendations/internal/dto"
)

// TTLs. Each one is chosen from what makes its shelf go stale, not from how
// expensive it was to compute:
const (
	// homeCacheTTL matches the affinity decay curve: a visitor's taste profile
	// moves slower than a minute, so a minute of staleness is invisible to the
	// shopper and saves a centroid plus an ANN search on every page view.
	homeCacheTTL = 1 * time.Minute
	// similarCacheTTL is longer because "products like this one" changes only
	// when the catalog does, and the catalog publishes re-embeds through the
	// outbox rather than per click.
	similarCacheTTL = 5 * time.Minute
)

// homeCacheKey is per visitor, because the shelf is. An empty visitorKey never
// reaches the cache: it means "no guest cookie and nobody signed in", which the
// handler answers with best sellers anyway.
func homeCacheKey(visitorKey string, limit int) string {
	return "recs:home:" + visitorKey + ":" + strconv.Itoa(limit)
}

// similarCacheKey is per product. The limit is part of the key for the same
// reason: a twelve-item shelf is not interchangeable with a fifty-item one.
func similarCacheKey(productUUID uuid.UUID, limit int) string {
	return "recs:similar:" + productUUID.String() + ":" + strconv.Itoa(limit)
}

// cacheShelf is the cache payload. It is a distinct type from Shelf so a field
// added to Shelf does not silently change how old entries decode, and the
// version tag makes a format change explicit instead of accidental.
type cacheShelf struct {
	V      int                          `json:"v"`
	Source string                       `json:"source"`
	Items  []recsdto.RecommendationItem `json:"items"`
	Total  int                          `json:"total,omitempty"`
}

// cacheV changes when cacheShelf's meaning changes. Bumping it orphans old
// entries instead of serving them as if they were current.
const cacheV = 1

// readThrough returns a cached shelf when there is one, otherwise calls load,
// stores a non-empty result, and returns it.
//
// Three rules make this safe in front of a DB-backed shelf:
//   - a cache miss and a cache error are the same event, so Redis being down
//     costs latency, never a page error;
//   - only shelves with a source and at least one item are stored, so a
//     transient empty answer cannot pin itself in place for a minute;
//   - the returned error is the loader's, so failing to store is not failing
//     to render.
func (s *personalizedService) readThrough(ctx context.Context, key string, ttl time.Duration, load func(context.Context) (Shelf, error)) (Shelf, error) {
	if s.cache != nil {
		var hit cacheShelf
		if err := s.cache.Get(key, &hit); err == nil && hit.V == cacheV && hit.Source != "" {
			return Shelf{Source: hit.Source, Items: hit.Items, Total: hit.Total}, nil
		}
	}

	shelf, err := load(ctx)
	if err != nil {
		return Shelf{}, err
	}
	if len(shelf.Items) == 0 || shelf.Source == "" || s.cache == nil {
		return shelf, nil
	}
	payload := cacheShelf{V: cacheV, Source: shelf.Source, Items: shelf.Items, Total: shelf.Total}
	// A failure here is a missed optimization, not a broken page.
	_ = s.cache.Set(key, payload, ttl)
	return shelf, nil
}

// invalidateShelves drops every cached shelf.
//
// This is blunt on purpose: a product's price or stock feeds every shelf it
// appears on, and at this key space a pattern clear is one SCAN. Narrow it only
// if the flush ever becomes measurable, not pre-emptively.
func (s *personalizedService) invalidateShelves() {
	if s.cache == nil {
		return
	}
	_ = s.cache.Clear("recs:*")
}

// Invalidate is the exported form, for the publish path.
func (s *personalizedService) Invalidate() { s.invalidateShelves() }
