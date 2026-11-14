package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BestSellerRow is one row of the popularity rollup.
type BestSellerRow struct {
	ProductUUID uuid.UUID
	UnitsSold   int64
	Revenue     float64
}

// PopularityRepository reads and rebuilds the best-seller rollup.
type PopularityRepository interface {
	// Rebuild recomputes product_popularity from orders in the last windowDays.
	// It is idempotent: the table is rewritten, never appended to.
	Rebuild(ctx context.Context, windowDays int) (int, error)
	// Top returns the highest-selling products, best first.
	Top(ctx context.Context, limit int) ([]BestSellerRow, error)
	// UnitsByProduct returns units sold per product, plus the largest value, so
	// a caller can normalise popularity onto 0..1 without a second query.
	UnitsByProduct(ctx context.Context) (map[uuid.UUID]int64, int64, error)
	// Trending returns what visitors are buying over the last windowDays,
	// freshest first.
	//
	// It reads the event mirror, not product_popularity: the rollup is a 30-day
	// aggregate rebuilt at boot, so it cannot answer "today" and would keep
	// serving last week's winners. Trending is the tier below the personal
	// shelves and above the all-time list — for a visitor we know nothing
	// about, today's demand beats a month's average.
	//
	// Purchases only. A view is not demand: counting clicks here made the
	// trending row a duplicate of whatever the last few shoppers opened.
	Trending(ctx context.Context, windowDays int, limit int) ([]BestSellerRow, error)
}

type sqlPopularityRepository struct {
	pool *pgxpool.Pool
}

func NewSqlPopularityRepository(pool *pgxpool.Pool) PopularityRepository {
	return &sqlPopularityRepository{pool: pool}
}

// Rebuild is the whole aggregation. Status 2/3/4 are confirmed, shipped and
// delivered: a paid-for item that has not been cancelled or refunded.
//
// The order tables live in another service's database, mirrored here by
// postgres_fdw (see 000001_popularity.up.sql). When that link is missing the
// query fails and the caller falls back to catalog ratings, so the homepage
// degrades instead of breaking.
const rebuildSQL = `
INSERT INTO product_popularity (product_uuid, units_sold, revenue, window_days, refreshed_at)
SELECT oi.order_item_product_uuid,
       SUM(oi.order_item_quantity)::bigint,
       SUM(oi.order_item_subtotal),
       $1,
       now()
  FROM order_mirror.order_items oi
  JOIN order_mirror.orders o ON o.order_id = oi.order_item_order_id
 WHERE o.order_status IN (2, 3, 4)
   AND o.order_created_at > now() - make_interval(days => $1)
   AND oi.order_item_product_uuid <> '00000000-0000-0000-0000-000000000000'::uuid
 GROUP BY oi.order_item_product_uuid
ON CONFLICT (product_uuid) DO UPDATE
   SET units_sold   = EXCLUDED.units_sold,
       revenue      = EXCLUDED.revenue,
       window_days  = EXCLUDED.window_days,
       refreshed_at = EXCLUDED.refreshed_at`

func (r *sqlPopularityRepository) Rebuild(ctx context.Context, windowDays int) (int, error) {
	if windowDays <= 0 {
		windowDays = 30
	}
	// Rows outside the window must disappear; the upsert above only touches
	// products that still sold.
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin popularity rebuild: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM product_popularity`); err != nil {
		return 0, fmt.Errorf("clear popularity rollup: %w", err)
	}
	tag, err := tx.Exec(ctx, rebuildSQL, windowDays)
	if err != nil {
		return 0, fmt.Errorf("aggregate order items: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit popularity rebuild: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

func (r *sqlPopularityRepository) Top(ctx context.Context, limit int) ([]BestSellerRow, error) {
	if limit <= 0 || limit > 50 {
		limit = 12
	}
	rows, err := r.pool.Query(ctx,
		`SELECT product_uuid, units_sold, revenue
		   FROM product_popularity
		  ORDER BY units_sold DESC, revenue DESC
		  LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("query popularity: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (BestSellerRow, error) {
		var r BestSellerRow
		err := row.Scan(&r.ProductUUID, &r.UnitsSold, &r.Revenue)
		return r, err
	})
}

// UnitsByProduct reads the whole rollup. It is a few thousand rows at most (one
// per product that sold in the window), so pulling it into a map costs one
// round trip and keeps the blend a pure function.
func (r *sqlPopularityRepository) UnitsByProduct(ctx context.Context) (map[uuid.UUID]int64, int64, error) {
	rows, err := r.pool.Query(ctx, `SELECT product_uuid, units_sold FROM product_popularity`)
	if err != nil {
		return nil, 0, fmt.Errorf("query popularity units: %w", err)
	}
	defer rows.Close()

	out := make(map[uuid.UUID]int64)
	var max int64
	for rows.Next() {
		var id uuid.UUID
		var units int64
		if err := rows.Scan(&id, &units); err != nil {
			return nil, 0, fmt.Errorf("scan popularity units: %w", err)
		}
		out[id] = units
		if units > max {
			max = units
		}
	}
	return out, max, rows.Err()
}

// trendingSQL counts purchased units per product inside the window.
//
// Purchases are one event per line with `props.quantity` carrying the count, so
// it coalesces to 1 when a caller (an older bundle, or a test) omitted it: an
// event that exists at all is one unit. Ordering by units then by the newest
// purchase breaks ties toward whatever is selling right now.
const trendingSQL = `
SELECT product_uuid,
       SUM(COALESCE((props->>'quantity')::int, 1))::bigint AS units,
       0::float8                                          AS revenue
  FROM analytics_mirror.user_events
 WHERE event_type = 'purchase'
   AND product_uuid IS NOT NULL
   AND occurred_at > now() - make_interval(days => $1::int)
 GROUP BY product_uuid
 ORDER BY units DESC, MAX(occurred_at) DESC
 LIMIT $2`

func (r *sqlPopularityRepository) Trending(ctx context.Context, windowDays int, limit int) ([]BestSellerRow, error) {
	if windowDays <= 0 {
		windowDays = 1
	}
	if limit <= 0 || limit > 50 {
		limit = 12
	}
	rows, err := r.pool.Query(ctx, trendingSQL, windowDays, limit)
	if err != nil {
		return nil, fmt.Errorf("query trending: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (BestSellerRow, error) {
		var b BestSellerRow
		err := row.Scan(&b.ProductUUID, &b.UnitsSold, &b.Revenue)
		return b, err
	})
}
