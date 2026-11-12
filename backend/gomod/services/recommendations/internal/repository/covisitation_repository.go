package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CovisitationRepository owns product_covisitation: the "seen together" graph.
type CovisitationRepository interface {
	// Rebuild recomputes the pairs from analytics events and orders. It is
	// idempotent: the table is replaced, never appended to.
	Rebuild(ctx context.Context, windowDays int) (int, error)
	// CoVisited returns products other visitors viewed alongside any of the
	// given products, strongest pair first, excluding the given products and
	// duplicates. An empty input returns nothing.
	CoVisited(ctx context.Context, products []uuid.UUID, limit int) ([]SimilarRow, error)
}

type sqlCovisitationRepository struct {
	pool *pgxpool.Pool
}

func NewSqlCovisitationRepository(pool *pgxpool.Pool) CovisitationRepository {
	return &sqlCovisitationRepository{pool: pool}
}

// covisitationRebuildSQL counts distinct co-viewers per pair.
//
// A pair is stored once, smaller uuid first, so the row count is half of what an
// ordered table would hold and a lookup scans one side. A visitor who viewed the
// same product twice counts once: the dedupe is over (visitor, product), not
// over events.
//
// Two sources, folded into one union before the dedupe so nobody is counted
// twice: analytics views, and products bought in the same order. The order
// mirror carries no user id, so an order is its own "visitor" â€” that is exactly
// the right identity for bought-together, and it keeps purchases in the graph
// on a store that has almost no view traffic yet.
//
// ponytail: purchases and views weigh the same. Weighting purchases higher needs
// enough orders to matter; two order lines would make the table orders-only.
const covisitationRebuildSQL = `
WITH touched AS (
  SELECT anon_id::text AS visitor, product_uuid
    FROM analytics_mirror.user_events
   WHERE product_uuid IS NOT NULL
     AND occurred_at > now() - make_interval(days => $1)
  UNION
  SELECT 'order:' || oi.order_item_order_id::text, oi.order_item_product_uuid
    FROM order_mirror.order_items oi
    JOIN order_mirror.orders o ON o.order_id = oi.order_item_order_id
   WHERE o.order_status IN (2, 3, 4)
     AND o.order_created_at > now() - make_interval(days => $1)
),
pairs AS (
  SELECT DISTINCT a.visitor,
         LEAST(a.product_uuid, b.product_uuid)    AS source_uuid,
         GREATEST(a.product_uuid, b.product_uuid) AS target_uuid
    FROM touched a
    JOIN touched b ON a.visitor = b.visitor AND a.product_uuid <> b.product_uuid
)
INSERT INTO product_covisitation (source_uuid, target_uuid, visitors, last_seen)
SELECT source_uuid, target_uuid, COUNT(*)::int, now()
  FROM pairs
 GROUP BY source_uuid, target_uuid
ON CONFLICT (source_uuid, target_uuid) DO UPDATE
   SET visitors  = EXCLUDED.visitors,
       last_seen = EXCLUDED.last_seen`

func (r *sqlCovisitationRepository) Rebuild(ctx context.Context, windowDays int) (int, error) {
	if windowDays <= 0 {
		windowDays = 30
	}
	// Pairs whose visitors all aged out must disappear; the upsert only touches
	// pairs that were seen again.
	if _, err := r.pool.Exec(ctx, `DELETE FROM product_covisitation WHERE last_seen < now() - make_interval(days => $1)`, windowDays); err != nil {
		return 0, fmt.Errorf("covisitation prune: %w", err)
	}
	tag, err := r.pool.Exec(ctx, covisitationRebuildSQL, windowDays)
	if err != nil {
		return 0, fmt.Errorf("covisitation rebuild: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// coVisitedSQL reads the pairs regardless of which end the product was stored on.
//
// Both directions matter: a pair is stored undirected, so a product that
// happened to be the larger uuid of a pair lives in target_uuid. The UNION ALL
// with visitors summed by the outer GROUP BY is what keeps a product that
// appears on both sides from returning twice. The name comes from the mirror
// because the brand cap groups on it; an unnamed candidate would each form its
// own bucket and defeat the cap.
const coVisitedSQL = `
SELECT p.product_uuid, p.product_name, SUM(edges.visitors) AS score
  FROM (
        SELECT target_uuid AS product_uuid, visitors
          FROM product_covisitation
         WHERE source_uuid = ANY($1)
        UNION ALL
        SELECT source_uuid AS product_uuid, visitors
          FROM product_covisitation
         WHERE target_uuid = ANY($1)
       ) edges
  JOIN catalog_mirror.products p ON p.product_uuid = edges.product_uuid
 WHERE NOT (edges.product_uuid = ANY($1))
   AND p.product_status = 1
 GROUP BY p.product_uuid, p.product_name
 ORDER BY score DESC
 LIMIT $2`

func (r *sqlCovisitationRepository) CoVisited(ctx context.Context, products []uuid.UUID, limit int) ([]SimilarRow, error) {
	if len(products) == 0 || limit <= 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, coVisitedSQL, products, limit)
	if err != nil {
		return nil, fmt.Errorf("covisitation lookup: %w", err)
	}
	defer rows.Close()

	out := make([]SimilarRow, 0, limit)
	seen := make(map[uuid.UUID]struct{}, limit)
	for rows.Next() {
		var (
			row   SimilarRow
			score int
		)
		if err := rows.Scan(&row.ProductUUID, &row.Name, &score); err != nil {
			return nil, fmt.Errorf("scan covisitation: %w", err)
		}
		if _, dup := seen[row.ProductUUID]; dup {
			continue
		}
		seen[row.ProductUUID] = struct{}{}
		// The pair count is not a cosine, so it is not reported as one; the
		// caller keeps these rows out of similarity ranking and uses them only
		// as extra candidates.
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("covisitation rows: %w", err)
	}
	return out, nil
}
