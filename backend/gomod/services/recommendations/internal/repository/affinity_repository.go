package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AffinityRow is one (visitor, product) pair of the taste profile.
type AffinityRow struct {
	VisitorKey  string
	ProductUUID uuid.UUID
	Score       float64
}

// QuerySeed is one keyword the visitor used, weighted the same way products are.
//
// A query is not a product: it has no vector of its own. It becomes a seed by
// borrowing the vectors of the results the visitor actually clicked, which is
// what a search materially *means*. The click products stay in the caller's
// `seen` set so the shelf does not recommend what was just opened.
type QuerySeed struct {
	Query string
	Score float64
	// Clicked are the products the visitor opened from this query's results.
	Clicked []uuid.UUID
}

// AffinityRepository reads a visitor's taste profile.
//
// Profiles are computed on read, straight from the event mirror, instead of at
// boot into a `user_affinity` table. The old rollup meant a visitor's first
// three product views did nothing until the service was restarted, which is the
// single most visible way a recommendation shelf looks broken.
type AffinityRepository interface {
	// Top returns the strongest products for one visitor, most relevant first.
	Top(ctx context.Context, visitorKey string, limit int) ([]AffinityRow, error)
	// SearchSeeds returns the visitor's most recent successful keywords, each
	// with the products they clicked. A searched-but-never-clicked query is
	// excluded: it says what the shopper wanted, not what the catalog has.
	SearchSeeds(ctx context.Context, visitorKey string, limit int) ([]QuerySeed, error)
}

type sqlAffinityRepository struct {
	pool *pgxpool.Pool
}

func NewSqlAffinityRepository(pool *pgxpool.Pool) AffinityRepository {
	return &sqlAffinityRepository{pool: pool}
}

// Event weights. A purchase is the only unambiguous signal; a cart add is a
// strong intent; a click out of search is a deliberate choice among visible
// alternatives; a view is weak. A purchase plus a cart add therefore always
// outranks any number of views a session can produce.
const (
	weightView     = 1.0
	weightAddCart  = 3.0
	weightPurchase = 5.0
)

const (
	// affinityWindowDays bounds how far back history is read.
	affinityWindowDays = 90
	// affinityScoreCap keeps one runaway session from crowding out the rest of
	// a profile: a product cannot weigh more than a purchase plus a cart add.
	affinityScoreCap = 20.0
	// affinityHalfLifeDays is the recency falloff: an event from two weeks ago
	// counts half as much as one from today. Without it, a month-old browsing
	// session would weigh the same as this morning's.
	affinityHalfLifeDays = 14
)

// visitorPredicate is the shared visitor filter, emitted as two column
// predicates rather than a cast on a computed key: an expression on top of a
// foreign table cannot use the remote index, so this keeps anon lookups on
// (anon_id, occurred_at) and signed-in lookups on (user_id, occurred_at).
//
// $1 = user id (NULL when anonymous), $2 = anon id (NULL when signed in).
const visitorPredicate = `(
      ($1::int IS NOT NULL AND user_id = $1::int)
   OR ($2::uuid IS NOT NULL AND $1::int IS NULL AND anon_id = $2::uuid)
)`

// topAffinitySQL scores every product the visitor touched and returns the
// strongest ones.
const topAffinitySQL = `
WITH weighted AS (
    SELECT product_uuid,
           CASE event_type
               WHEN 'purchase'    THEN $3::float8
               WHEN 'add_to_cart' THEN $4::float8
               ELSE $5::float8
           END AS w,
           occurred_at
      FROM analytics_mirror.user_events
     WHERE event_type IN ('view_item', 'add_to_cart', 'purchase')
       AND product_uuid IS NOT NULL
       AND occurred_at > now() - make_interval(days => $6::int)
       AND ` + visitorPredicate + `
),
scored AS (
    SELECT product_uuid,
           SUM(w * exp(-ln(2.0) * EXTRACT(EPOCH FROM (now() - occurred_at)) / 86400.0 / $7::float8)) AS score,
           MAX(occurred_at) AS last_event_at
      FROM weighted
     GROUP BY product_uuid
)
SELECT product_uuid,
       LEAST(score, $8::float8) AS score
  FROM scored
 WHERE score > 0
 ORDER BY LEAST(score, $8::float8) DESC, last_event_at DESC
 LIMIT $9`

// searchSeedSQL scores each successful keyword by its last use and joins the
// products clicked out of its result page.
//
// Recency is per query (MAX(occurred_at) over its search events), not per
// click: a query the shopper retyped this morning should outrank one from last
// month even if the click is older. One row per (query, clicked product); the
// caller folds them into QuerySeed values.
const searchSeedSQL = `
WITH searched AS (
    SELECT lower(query) AS q,
           MIN(query) AS display_query,
           MAX(occurred_at) AS last_search_at
      FROM analytics_mirror.user_events
     WHERE event_type = 'search'
       AND query IS NOT NULL
       AND query <> ''
       AND (result_count IS NULL OR result_count > 0)
       AND occurred_at > now() - make_interval(days => $3::int)
       AND ` + visitorPredicate + `
     GROUP BY lower(query)
),
clicked AS (
    SELECT lower(query) AS q, product_uuid, MAX(occurred_at) AS last_click_at
      FROM analytics_mirror.user_events
     WHERE event_type = 'search_click'
       AND query IS NOT NULL
       AND product_uuid IS NOT NULL
       AND occurred_at > now() - make_interval(days => $3::int)
       AND ` + visitorPredicate + `
     GROUP BY lower(query), product_uuid
)
SELECT s.display_query,
       exp(-ln(2.0) * EXTRACT(EPOCH FROM (now() - s.last_search_at)) / 86400.0 / $4::float8) AS score,
       c.product_uuid
  FROM searched s
  JOIN clicked c ON c.q = s.q
 ORDER BY score DESC, s.last_search_at DESC, c.last_click_at DESC
 LIMIT $5`

// affinitySeedLimit is the hard ceiling Top accepts. The service asks for its
// own affinitySeedSize; this bound only stops an unbounded caller.
const affinitySeedLimit = 50

func (r *sqlAffinityRepository) Top(ctx context.Context, visitorKey string, limit int) ([]AffinityRow, error) {
	userID, anonID, err := SplitVisitorKey(visitorKey)
	if err != nil {
		return nil, err
	}
	if userID == nil && anonID == nil {
		return nil, nil
	}
	if limit <= 0 || limit > affinitySeedLimit {
		limit = affinitySeedLimit
	}
	rows, err := r.pool.Query(ctx, topAffinitySQL,
		userID, anonID,
		weightPurchase, weightAddCart, weightView,
		affinityWindowDays, affinityHalfLifeDays, affinityScoreCap, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("read affinity: %w", err)
	}
	defer rows.Close()

	var out []AffinityRow
	for rows.Next() {
		var row AffinityRow
		if err := rows.Scan(&row.ProductUUID, &row.Score); err != nil {
			return nil, fmt.Errorf("scan affinity: %w", err)
		}
		row.VisitorKey = visitorKey
		out = append(out, row)
	}
	return out, rows.Err()
}

// searchClickCap bounds how many clicked products one keyword can contribute,
// so a search with fifty clicks does not outvote a purchase.
const searchClickCap = 10

func (r *sqlAffinityRepository) SearchSeeds(ctx context.Context, visitorKey string, limit int) ([]QuerySeed, error) {
	userID, anonID, err := SplitVisitorKey(visitorKey)
	if err != nil {
		return nil, err
	}
	if userID == nil && anonID == nil {
		return nil, nil
	}
	if limit <= 0 || limit > affinitySeedLimit {
		limit = affinitySeedLimit
	}
	// The row limit is per (query, product) pair, so multiply it by the click
	// cap: otherwise one heavily-clicked keyword would fill the whole window.
	rows, err := r.pool.Query(ctx, searchSeedSQL,
		userID, anonID, affinityWindowDays, affinityHalfLifeDays, limit*searchClickCap,
	)
	if err != nil {
		return nil, fmt.Errorf("read search seeds: %w", err)
	}
	defer rows.Close()

	// Rows arrive ordered by score desc, so a map would lose the ranking. Keep
	// insertion order and index the seeds by query.
	var out []QuerySeed
	index := make(map[string]int)
	for rows.Next() {
		var query string
		var score float64
		var productUUID uuid.UUID
		if err := rows.Scan(&query, &score, &productUUID); err != nil {
			return nil, fmt.Errorf("scan search seed: %w", err)
		}
		seedIdx, ok := index[query]
		if !ok {
			if len(out) >= limit {
				continue
			}
			index[query] = len(out)
			out = append(out, QuerySeed{Query: query, Score: score})
			seedIdx = len(out) - 1
		}
		if len(out[seedIdx].Clicked) < searchClickCap {
			out[seedIdx].Clicked = append(out[seedIdx].Clicked, productUUID)
		}
	}
	return out, rows.Err()
}

// SplitVisitorKey decodes the 'u:<id>' / 'a:<uuid>' scheme used by both the
// handler and the affinity query. Anything else is treated as "no profile"
// rather than an error the shelf should report to a shopper.
func SplitVisitorKey(visitorKey string) (userID *int32, anonID *uuid.UUID, err error) {
	if len(visitorKey) < 3 {
		return nil, nil, nil
	}
	switch visitorKey[:2] {
	case "u:":
		var id int32
		if _, scanErr := fmt.Sscanf(visitorKey[2:], "%d", &id); scanErr != nil {
			// A malformed key must not 500 the homepage shelf.
			return nil, nil, nil
		}
		return &id, nil, nil
	case "a:":
		parsed, parseErr := uuid.Parse(visitorKey[2:])
		if parseErr != nil {
			return nil, nil, nil
		}
		return nil, &parsed, nil
	}
	return nil, nil, nil
}
