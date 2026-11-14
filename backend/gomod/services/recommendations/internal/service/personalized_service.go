package service

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/google/uuid"

	catalogcontract "goshop/services/catalog/contract"
	recsdto "goshop/services/recommendations/internal/dto"
	"goshop/services/recommendations/internal/embed"
	recsrepository "goshop/services/recommendations/internal/repository"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/logger"
)

// Additional shelf sources. As with best sellers, the source is always sent so
// the storefront can label a shelf honestly: "Picked for you" must never be
// served when the list was really an anonymous best-seller fallback.
const (
	SourcePersonalized = "personalized"
	// SourcePersonalizedSession is the second tier: the long-term profile was
	// too thin to project, so the shelf was built from the visitor's latest
	// browsing pass instead. It is still a personal shelf from the shopper's
	// point of view — "you were just looking at this" — which is why the
	// storefront labels it like SourcePersonalized rather than as a fallback.
	SourcePersonalizedSession = "personalized_session"
	SourceSimilar             = "similar"
	// Hybrid search reports which halves of the ranking were available, so the
	// storefront can label the result honestly instead of calling a keyword-only
	// answer "smart search".
	SourceHybrid   = "hybrid"
	SourceKeyword  = "keyword"
	SourceSemantic = "semantic"
)

// affinitySeedSize is how many of a visitor's strongest products define their
// taste. More than this and the centroid drifts toward "everything the site
// sells", which is the same as no personalization at all.
const affinitySeedSize = 20

// searchSeedSize is how many keywords can define a visitor's taste. Deliberately
// smaller than the product side: a keyword is a coarser signal (it has to borrow
// its direction from clicks), so ten recent successful searches already describe
// the intent, and a long tail of one-off queries would only add noise.
const searchSeedSize = 10

// sessionSeedSize is how many products of the latest session define the
// session-tier centroid. A pass that touched more than ten products has stopped
// being a single intent, and the extra rows would pull the centroid toward
// "everything on this site" — the same failure affinitySeedSize guards against.
const sessionSeedSize = 10

// sessionThreshold is the profile depth below which the session tier takes over.
//
// The two tiers read the same 90-day events, so "profile is empty" is not the
// trigger that matters: a profile with one product produces a centroid, and a
// centroid of one product is just that product's neighbourhood, which is what
// the session would say anyway — only staler. Three is the point where the
// profile has more than one bougbt-or-browsed direction to average and is
// genuinely a better answer than the current pass.
const sessionThreshold = 3

// candidatePoolSize is how many neighbours the ANN step over-fetches before the
// popularity blend re-ranks them. A pure similarity top-12 is enough to fill the
// shelf, but the brand cap can only vary a row that contains several brands, so
// the pool has to be deep enough to reach past the first brand's block. At 60 a
// phone-heavy centroid returned two brands; at 300 it returns four.
const candidatePoolSize = 300

// covisitationDepth is how many "seen together" products one shelf lookup can
// add to the candidate pool. It is small on purpose: the vector pool is the
// ranking and co-visitation is the widening, so a dozen extra candidates is
// enough to break a one-brand neighbourhood without displacing it.
const covisitationDepth = 40

// Blend weights. Similarity dominates: this is still a personal shelf. The
// popularity term exists to break near-ties toward something people actually
// buy, which is the only defence against a stale or skewed centroid surfacing
// dead stock.
const (
	weightSimilarity = 0.7
	weightPopularity = 0.3
)

// PersonalizedService builds shelves from a visitor's interaction history and
// product similarity.
type PersonalizedService interface {
	// ForVisitor returns products near the centroid of a visitor's affinity
	// profile. reported means the visitor had enough history to personalize;
	// when false the caller must fall back, and items is empty.
	ForVisitor(ctx context.Context, visitorKey string, limit int) (Shelf, bool, error)
	// Similar returns products close to one product, excluding itself.
	Similar(ctx context.Context, productUUID uuid.UUID, limit int) (Shelf, error)
	// HybridSearch ranks the catalog by meaning and by keyword at once.
	//
	// A pure keyword index cannot answer "something to listen to at the gym"
	// and a pure vector index cannot honour a typed SKU, so the two lists are
	// fused (see fuseRanks) and the result is labeled with the source actually
	// used.
	HybridSearch(ctx context.Context, in HybridQuery) (Shelf, error)
	// Invalidate drops every cached shelf. Called after a publish so a new or
	// edited product does not wait out a TTL before a shelf can show it.
	Invalidate()
}

type personalizedService struct {
	affinity   recsrepository.AffinityRepository
	embeddings recsrepository.EmbeddingRepository
	centroids  recsrepository.CentroidRepository
	popularity recsrepository.PopularityRepository
	catalog    catalogcontract.CatalogClient
	// suggester and embedder are the two halves of hybrid search. Both are
	// optional: a dev box without Meilisearch or without the sidecar still
	// serves search, labeled with the half it actually used.
	suggester ProductSuggester
	embedder  embed.Embedder
	// cache is the shelf cache. Nil means "no cache configured": every read
	// goes to the database, which is slower but never wrong.
	cache cache.RedisCacheService
	// covisitation is the "seen together" graph. Nil disables it, which is the
	// behaviour a database without migration 000007 gets: the shelf falls back
	// to vector ranking alone.
	covisitation recsrepository.CovisitationRepository
}

func NewPersonalizedService(
	affinity recsrepository.AffinityRepository,
	embeddings recsrepository.EmbeddingRepository,
	centroids recsrepository.CentroidRepository,
	popularity recsrepository.PopularityRepository,
	catalog catalogcontract.CatalogClient,
	suggester ProductSuggester,
	embedder embed.Embedder,
	shelfCache cache.RedisCacheService,
	covisitation recsrepository.CovisitationRepository,
) PersonalizedService {
	return &personalizedService{
		affinity:     affinity,
		embeddings:   embeddings,
		centroids:    centroids,
		popularity:   popularity,
		catalog:      catalog,
		suggester:    suggester,
		embedder:     embedder,
		cache:        shelfCache,
		covisitation: covisitation,
	}
}

// ForVisitor projects the visitor's history into embedding space.
//
// The centroid is score-weighted: a purchased product pulls the result harder
// than a browsed one, because that is what the profile means. Products the
// visitor already engaged with are excluded — recommending a thing somebody
// just bought is the most visible way to look broken.
func (s *personalizedService) ForVisitor(ctx context.Context, visitorKey string, limit int) (Shelf, bool, error) {
	if limit <= 0 || limit > 50 {
		limit = 12
	}
	if visitorKey == "" {
		return Shelf{}, false, nil
	}

	// The cache stores a shelf, and "false" (no usable history) is a different
	// answer from "empty shelf". It is not cached: a visitor's first click is
	// what turns a fallback into a personal shelf, and caching the "no" would
	// delay that by a whole TTL.
	shelf, err := s.readThrough(ctx, homeCacheKey(visitorKey, limit), homeCacheTTL, func(ctx context.Context) (Shelf, error) {
		shelf, ok, err := s.buildForVisitor(ctx, visitorKey, limit)
		if err != nil || !ok {
			// A no-history answer is indistinguishable from a miss, so the
			// loader has to fail loudly enough to stop a store. readThrough
			// refuses empty shelves, which is exactly this case.
			return Shelf{}, err
		}
		return shelf, nil
	})
	if err != nil {
		return Shelf{}, false, err
	}
	if shelf.Source == "" {
		return Shelf{}, false, nil
	}
	return shelf, true, nil
}

// buildForVisitor is the uncached shelf. reported=false means "no usable
// history", which the caller answers with best sellers.
//
// Two tiers, and the order is by *evidence*, not by recency: the 90-day profile
// answers only when it is deep enough to mean something (sessionThreshold
// products), because a one-product profile is a single product's neighbourhood
// and the current session says the same thing with fresher data. Below the
// threshold the session wins; if it has nothing either, the profile is used as
// the last resort rather than falling straight through to best sellers.
func (s *personalizedService) buildForVisitor(ctx context.Context, visitorKey string, limit int) (Shelf, bool, error) {
	centroid, seen, depth, err := s.visitorCentroid(ctx, visitorKey)
	if err != nil {
		return Shelf{}, false, err
	}
	source := SourcePersonalized

	// A shallow profile is not evidence: swap it for the session when the
	// session has one. The session read is skipped for a deep profile, so a
	// regular shopper never pays for the extra query.
	if depth < sessionThreshold {
		sessionCentroid, sessionSeen, serr := s.sessionCentroid(ctx, visitorKey)
		switch {
		case serr != nil:
			// A broken session read must not blank a shelf the profile could
			// have filled; it only means there is no second chance.
			logger.Log.Warn().Err(serr).Msg("session affinity unavailable; using the profile")
		case sessionCentroid != nil:
			centroid, seen, source = sessionCentroid, sessionSeen, SourcePersonalizedSession
		}
	}
	if centroid == nil {
		return Shelf{}, false, nil
	}

	rows, err := s.embeddings.Candidates(ctx, centroid, seen, candidatePoolSize)
	if err != nil {
		return Shelf{}, false, fmt.Errorf("nearest products: %w", err)
	}
	if len(rows) == 0 {
		return Shelf{}, false, nil
	}

	// Co-visitation: what other visitors looked at alongside the products this
	// one browsed. The vector step answers "looks like", which on a catalog whose
	// products share one template description collapses to "same price, same
	// category". This answers "was looked at together", which the descriptions
	// cannot express.
	//
	// The pair list is appended after the vector pool rather than replacing it:
	// it is an addition to the ranking, and a visitor with no co-visited pairs
	// (a fresh table, or a product nobody else opened) must keep the shelf the
	// vector step already produced.
	rows = s.appendCoVisited(ctx, rows, seen)

	// The blend needs a sale history to mean anything. An empty rollup (fresh
	// database, or the postgres_fdw link was down at boot) leaves similarity as
	// the only ranking rather than scoring every product 0.7 * sim + 0 = the
	// same order, half-scaled and harder to reason about.
	if units, maxUnits, perr := s.popularity.UnitsByProduct(ctx); perr == nil && maxUnits > 0 {
		rows = blend(rows, units, maxUnits)
	}

	// Similarity alone fills a shelf with near-duplicates: on a catalog whose
	// products share a template description, the twelve nearest neighbours of
	// "I looked at cameras" are twelve phones that happen to cost the same.
	// Group by brand and pick round-robin, so the ranking decides which item
	// wins its brand and breadth decides how far down the row one brand gets.
	rows = diversify(rows, limit)
	items, err := s.hydrate(ctx, rows)
	if err != nil {
		return Shelf{}, false, err
	}
	if len(items) == 0 {
		return Shelf{}, false, nil
	}
	logger.Log.Debug().
		Int("category_coverage", categoryCoverage(items)).
		Float64("intra_list_similarity", intraListSimilarity(items)).
		Int("brands", brandCount(items)).
		Str("source", source).
		Msg("personalized shelf diversity")
	return Shelf{Source: source, Items: items}, true, nil
}

// sessionCentroid projects the visitor's latest session into embedding space.
//
// It is deliberately dumber than visitorCentroid: no search seeds, no
// per-identity weighting. A session is minutes long and holds a handful of
// clicks, so the only signal worth using is "these products, recently", and
// folding search keywords in would add a second, older source to a point that
// exists precisely because the older sources were empty.
//
// The same nil contract as visitorCentroid: a session whose products have no
// vectors is no session at all.
func (s *personalizedService) sessionCentroid(ctx context.Context, visitorKey string) ([]float32, []uuid.UUID, error) {
	rows, err := s.affinity.SessionTop(ctx, visitorKey, sessionSeedSize)
	if err != nil {
		return nil, nil, fmt.Errorf("session affinity: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil, nil
	}
	centroid, seen, err := s.centroids.Centroid(ctx, rows)
	if err != nil {
		return nil, nil, fmt.Errorf("session centroid: %w", err)
	}
	return centroid, seen, nil
}

// visitorCentroid merges the two signals a visitor leaves behind: products they
// interacted with, and keywords they searched.
//
// depth is how many of those products actually carry a vector. It is the number
// that decides whether this profile is worth ranking: top can be long while every
// product in it is un-embedded, and a centroid built from nothing is not depth
// the caller can trust.
//
// Search is folded in as a second centroid rather than as extra products,
// because a keyword has no vector of its own — it borrows the mean vector of
// the results that were clicked out of it (see CentroidRepository).
//
// Weighting is by seed count, not by raw score: product affinity and search
// affinity are scored on the same decay curve but their absolute magnitudes are
// not comparable, and a shopper with one purchase should not be drowned out by
// a shopper whose history happens to be all searches. Each side contributes in
// proportion to how much evidence it holds.
func (s *personalizedService) visitorCentroid(ctx context.Context, visitorKey string) ([]float32, []uuid.UUID, int, error) {
	top, err := s.affinity.Top(ctx, visitorKey, affinitySeedSize)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("affinity lookup: %w", err)
	}
	seeds, err := s.affinity.SearchSeeds(ctx, visitorKey, searchSeedSize)
	if err != nil {
		// Search affinity is an enhancement: a mirror column that has not been
		// migrated yet must not blank a shelf that product history could fill.
		logger.Log.Warn().Err(err).Msg("search affinity unavailable; using product history only")
		seeds = nil
	}

	var (
		sum      []float64
		total    float64
		seen     []uuid.UUID
		depth    int
		bulkSeen = make(map[uuid.UUID]struct{})
	)
	add := func(centroid []float32, weight float64) error {
		if centroid == nil || weight <= 0 {
			return nil
		}
		if sum == nil {
			sum = make([]float64, len(centroid))
		}
		if len(centroid) != len(sum) {
			return fmt.Errorf("centroid dimension mismatch: %d vs %d", len(centroid), len(sum))
		}
		for i, x := range centroid {
			sum[i] += float64(x) * weight
		}
		total += weight
		return nil
	}

	if len(top) > 0 {
		centroid, productSeen, err := s.centroids.Centroid(ctx, top)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("centroid: %w", err)
		}
		// Depth is the profile's own breadth, before vector availability: a
		// product whose embedding is missing still says the visitor cared about
		// it, and Centroid already returns nil when none of them can be
		// averaged. Counting only embedded rows here would make a catalogue
		// backfill gap look like "this visitor is new" and hand them the session
		// shelf every time.
		depth = len(top)
		if err := add(centroid, float64(len(top))); err != nil {
			return nil, nil, 0, err
		}
		for _, id := range productSeen {
			if _, ok := bulkSeen[id]; !ok {
				bulkSeen[id] = struct{}{}
				seen = append(seen, id)
			}
		}
	}

	if len(seeds) > 0 {
		centroid, clickSeen, err := s.centroids.SearchCentroid(ctx, seeds)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("search centroid: %w", err)
		}
		if err := add(centroid, float64(len(seeds))); err != nil {
			return nil, nil, 0, err
		}
		// A click is only excluded when it is not already covered above, so a
		// shopper who searched for something they had also viewed is not
		// "seen" twice. Exclusion is by product, never by score.
		for _, id := range clickSeen {
			if _, ok := bulkSeen[id]; ok {
				continue
			}
			bulkSeen[id] = struct{}{}
			seen = append(seen, id)
		}
	}

	if total == 0 || sum == nil {
		return nil, seen, depth, nil
	}
	return unitVector(sum, total), seen, depth, nil
}

// unitVector is the shared final step: average, then rescale to unit length so
// cosine distance treats a computed centroid exactly like a stored vector.
func unitVector(sum []float64, total float64) []float32 {
	out := make([]float32, len(sum))
	var norm float64
	for i, x := range sum {
		v := float32(x / total)
		out[i] = v
		norm += float64(v) * float64(v)
	}
	if norm > 0 {
		inv := float32(1 / math.Sqrt(norm))
		for i := range out {
			out[i] *= inv
		}
	}
	return out
}

// appendCoVisited adds the products other visitors opened alongside the ones
// this visitor opened, after the vector pool and without duplicates.
//
// They are appended with their pair count as a score, then the whole list is
// left in pool order: co-visited products are candidates, not answers. The brand
// cap and the popularity blend still decide which of them reach the row, so a
// co-visited product cannot jump ahead of a better match just for being
// co-visited.
//
// ponytail: appended, not weighted into the score. A real blend needs a weight
// fitted against clicks, which this store does not have yet; appending changes
// which products are available to the cap, which is the part that fixes the
// one-brand shelf. Revisit when CTR per source exists.
func (s *personalizedService) appendCoVisited(ctx context.Context, rows []recsrepository.SimilarRow, seeds []uuid.UUID) []recsrepository.SimilarRow {
	if s.covisitation == nil || len(seeds) == 0 {
		return rows
	}
	extra, err := s.covisitation.CoVisited(ctx, seeds, covisitationDepth)
	if err != nil {
		// A missing or empty pair table is not a reason to lose the shelf; the
		// vector pool already answered.
		logger.Log.Warn().Err(err).Msg("covisitation lookup failed; using vector pool only")
		return rows
	}
	if len(extra) == 0 {
		return rows
	}
	present := make(map[uuid.UUID]struct{}, len(rows))
	for _, row := range rows {
		present[row.ProductUUID] = struct{}{}
	}
	added := 0
	for _, extra := range extra {
		if _, ok := present[extra.ProductUUID]; ok {
			continue
		}
		present[extra.ProductUUID] = struct{}{}
		extra.Similarity = 0
		extra.CoVisited = true
		rows = append(rows, extra)
		added++
	}
	logger.Log.Debug().
		Int("seeds", len(seeds)).
		Int("covisited", len(extra)).
		Int("added", added).
		Msg("covisitation candidates")
	return rows
}

// blend re-ranks similarity neighbours by `0.7*sim + 0.3*normalised popularity`
// and returns the best `limit` of them.
// Ranking happens on the candidate pool before hydration, so a product the
// catalog later rejects as out of stock can still cost a slot — the shelf may
// come back a little short. That is the right trade: fetching past it would
// mean a second catalog round trip for a case that is rare on a shelf already
// filtered by stock at render time.
func blend(rows []recsrepository.SimilarRow, units map[uuid.UUID]int64, maxUnits int64) []recsrepository.SimilarRow {
	out := make([]recsrepository.SimilarRow, 0, len(rows))
	for _, row := range rows {
		// A co-visited row has no cosine to blend and no popularity claim to
		// make; it keeps its pool position instead of being sorted to the
		// bottom by a zero that does not mean "worst match".
		if row.CoVisited {
			out = append(out, row)
			continue
		}
		pop := 0.0
		if maxUnits > 0 {
			pop = float64(units[row.ProductUUID]) / float64(maxUnits)
		}
		sim := row.Similarity
		// Keep similarity in the DTO for the storefront's debug display, but
		// rank on the blend.
		row.Similarity = weightSimilarity*sim + weightPopularity*pop
		out = append(out, row)
	}
	// Stable sort on a key that puts co-visited rows back in the tail, where
	// appendCoVisited put them: they widen the pool, they do not outrank a
	// product the visitor's own history points at.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CoVisited != out[j].CoVisited {
			return !out[i].CoVisited
		}
		return out[i].Similarity > out[j].Similarity
	})
	return out
}

// Similar ranks by embedding distance, then drops anything out of stock.
func (s *personalizedService) Similar(ctx context.Context, productUUID uuid.UUID, limit int) (Shelf, error) {
	if limit <= 0 || limit > 50 {
		limit = 12
	}
	return s.readThrough(ctx, similarCacheKey(productUUID, limit), similarCacheTTL, func(ctx context.Context) (Shelf, error) {
		return s.buildSimilar(ctx, productUUID, limit)
	})
}

func (s *personalizedService) buildSimilar(ctx context.Context, productUUID uuid.UUID, limit int) (Shelf, error) {
	// Ask for extra rows: some will be filtered as unavailable.
	rows, err := s.embeddings.Similar(ctx, productUUID, limit*2)
	if err != nil {
		return Shelf{}, fmt.Errorf("similar products: %w", err)
	}
	items, err := s.hydrate(ctx, rows)
	if err != nil {
		return Shelf{}, err
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return Shelf{Source: SourceSimilar, Items: items}, nil
}

// hydrate resolves ranking ids through the catalog and keeps only products
// that exist and are in stock.
//
// Availibility comes from the catalog batch, not a second inventory call: an
// item the catalog cannot describe must not appear, and its stock field is the
// same value the product page shows.
func (s *personalizedService) hydrate(ctx context.Context, rows []recsrepository.SimilarRow) ([]recsdto.RecommendationItem, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ProductUUID.String())
	}

	products, err := s.catalog.GetProductsBatch(ctx, ids, nil)
	if err != nil {
		return nil, fmt.Errorf("catalog batch: %w", err)
	}
	byID := make(map[string]catalogcontract.ProductItemInfo, len(products))
	for _, p := range products {
		byID[p.ProductUUID] = p
	}

	items := make([]recsdto.RecommendationItem, 0, len(rows))
	for _, row := range rows {
		p, ok := byID[row.ProductUUID.String()]
		if !ok {
			// Deleted or archived between embedding and query.
			continue
		}
		if p.ProductStock <= 0 {
			continue
		}
		item := toItem(p)
		item.Similarity = row.Similarity
		items = append(items, item)
	}
	logger.Log.Debug().
		Int("requested", len(rows)).
		Int("returned", len(items)).
		Msg("hydrated recommendation rows")
	return items, nil
}

// HybridQuery is one search request as the gateway received it.
type HybridQuery struct {
	Query string
	Page  int
	Limit int
	// Narrowing is the shopper's facet selection. Empty means unfiltered.
	Narrowing SearchNarrowing
}

// facetFilter turns the narrowing into the repository's predicate. The two
// types do not merge: narrowing is what the shopper asked for and is also sent
// to Meilisearch, whose grammar differs, while FacetFilter is the exact SQL
// predicate over the mirrored catalog row.
func (n SearchNarrowing) facetFilter() recsrepository.FacetFilter {
	return recsrepository.FacetFilter{
		CategoryUUID: n.CategoryUUID,
		MinPrice:     n.MinPrice,
		MaxPrice:     n.MaxPrice,
		MinRating:    n.MinRating,
	}
}

// hybridShelfSize caps how many fused ids are hydrated. The storefront pages by
// twenty; hydrating much past that would spend a catalog round trip on rows the
// visitor never scrolls to.
const hybridShelfSize = 20

// searchShelfSize is how many results the visitor asked for at most. Fusion is
// only meaningful for a ranked list a person scrolls, not an unbounded export.
func (s *personalizedService) HybridSearch(ctx context.Context, in HybridQuery) (Shelf, error) {
	query := normalizeQueryText(in.Query)
	if query == "" {
		return Shelf{Source: SourceKeyword, Items: []recsdto.RecommendationItem{}}, nil
	}
	if in.Page < 1 {
		in.Page = 1
	}
	if in.Limit < 1 || in.Limit > 50 {
		in.Limit = hybridShelfSize
	}

	var (
		keywordIDs []string
		vectorIDs  []string
	)

	// Keyword side. A failure here is not fatal: it means "no text match", and
	// the vector side can still answer. The reverse is also true, which is why
	// both errors are collected rather than returned early.
	var keywordErr error
	if s.suggester != nil {
		keywordIDs, keywordErr = s.suggester.Suggest(ctx, query, in.Narrowing, fusionDepth)
	}

	// Vector side. The query is embedded on the fly; without an embedder there
	// is nothing to compare against, so the keyword side stands alone.
	var vectorErr error
	if s.embedder != nil {
		vectorIDs, vectorErr = s.vectorRanking(ctx, query, in.Narrowing)
	}

	if keywordErr != nil && vectorErr != nil {
		return Shelf{}, fmt.Errorf("hybrid search: keyword: %v; vector: %w", keywordErr, vectorErr)
	}

	ordered := fuseRanks(keywordIDs, vectorIDs)
	ids, err := uuidStrings(ordered)
	if err != nil {
		return Shelf{}, err
	}

	// Keyword hits reach products through their descriptions, so the catalog
	// has to confirm them before they are served. The vector side is already
	// filtered by NearestActive, but its filter is the same one, so running
	// both through this check keeps one predicate as the source of truth.
	if len(ids) == 0 {
		// No hits at all: the check below would take the "every id" branch and
		// hand back an empty page with no error attached to it.
		return Shelf{Source: s.hybridSource(vectorIDs, keywordIDs), Items: []recsdto.RecommendationItem{}, Total: 0}, nil
	}
	active, err := s.embeddings.ActiveAmong(ctx, ids, in.Narrowing.facetFilter())
	if err != nil {
		return Shelf{}, fmt.Errorf("active check: %w", err)
	}
	rows := make([]recsrepository.SimilarRow, 0, len(ids))
	for _, id := range ids {
		if _, ok := active[id]; !ok {
			continue
		}
		rows = append(rows, recsrepository.SimilarRow{ProductUUID: id})
	}

	// Paginate after filtering so a page is never padded with the products the
	// filter just removed. `start` is also what makes the total absolute below:
	// the count is over all matches, the page is one slice of them.
	start := (in.Page - 1) * in.Limit
	end := start + in.Limit
	if end > len(rows) {
		end = len(rows)
	}
	empty := start >= len(rows)

	var pageRows []recsrepository.SimilarRow
	if !empty {
		pageRows = rows[start:end]
	}

	items, err := s.hydrate(ctx, pageRows)
	if err != nil {
		return Shelf{}, err
	}

	// The total is the number of sellable matches, not the page length: a full
	// page might be the last one, so a page boundary cannot reveal it. Without
	// a real number the page-number row can only show the current page plus
	// one, which makes the pagination appear one page at a time as the visitor
	// clicks. Meilisearch's own estimate is not usable either — it still counts
	// products the catalog has archived.
	matched, cerr := s.embeddings.ActiveCount(ctx, matchingIDs(ordered), in.Narrowing.facetFilter())
	if cerr != nil {
		// Counting is presentation, not ranking: a failure here must not blank
		// a page that already has results. Falling back to the verified pool
		// understates the total, which is the honest direction.
		logger.Log.Warn().Err(cerr).Msg("hybrid search: total count unavailable")
		matched = len(rows)
	}
	// Total is absolute; the page is a slice of it. Adding `start` again would
	// make the count grow as the visitor pages, which breaks the page-number
	// row in exactly the way a guessed total did.
	total := matched

	// The label is what was actually used, never what was hoped for. A shopper
	// on a box without an embedder gets keyword results labeled as such.
	return Shelf{Source: s.hybridSource(vectorIDs, keywordIDs), Items: items, Total: total}, nil
}

// hybridSource names the halves that produced the ranking, so a degraded box is
// visible in the response instead of looking like a complete one.
func (s *personalizedService) hybridSource(vectorIDs, keywordIDs []string) string {
	switch {
	case s.embedder == nil || len(vectorIDs) == 0:
		return SourceKeyword
	case len(keywordIDs) == 0:
		return SourceSemantic
	}
	return SourceHybrid
}

// matchingIDs turns the fused id order into the UUID list the count needs,
// dropping anything the suggester returned in a shape we cannot use.
func matchingIDs(ordered []string) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(ordered))
	for _, raw := range ordered {
		if id, err := uuid.Parse(raw); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// vectorRanking embeds the query and returns the nearest sellable products that
// also pass the shopper's facets.
func (s *personalizedService) vectorRanking(ctx context.Context, query string, narrow SearchNarrowing) ([]string, error) {
	vectors, err := s.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(vectors) != 1 || len(vectors[0]) == 0 {
		return nil, fmt.Errorf("embed query: expected one vector, got %d", len(vectors))
	}
	rows, err := s.embeddings.NearestActive(ctx, vectors[0], narrow.facetFilter(), fusionDepth)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.ProductUUID.String())
	}
	return out, nil
}

// uuidStrings parses fused ids back into UUIDs, dropping the ones the suggester
// returned in a shape we cannot use. Suggestion ids are the catalog's product
// UUIDs in practice; a malformed one is a stale-index symptom, not a request
// error.
func uuidStrings(ids []string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(ids))
	for _, raw := range ids {
		id, err := uuid.Parse(raw)
		if err != nil {
			logger.Log.Debug().Str("id", raw).Msg("hybrid search: skipping non-uuid hit")
			continue
		}
		out = append(out, id)
	}
	return out, nil
}
