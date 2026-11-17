package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CatalogProduct is the text a product is embedded from.
//
// Only the fields that carry meaning are included: price, stock and status
// change constantly and would make similarity meaningless. ContentHash is over
// exactly these fields, so an edit to the name or description triggers a
// re-embed and nothing else does.
type CatalogProduct struct {
	ProductUUID uuid.UUID
	Name        string
	Description string
	Category    string
}

// SimilarRow is one neighbour of a product or of a taste centroid.
type SimilarRow struct {
	ProductUUID uuid.UUID
	Similarity  float64
	// Name is only filled by Candidates, which needs it to group the shelf by
	// brand before the catalog round trip. Empty elsewhere, where the caller
	// hydrates immediately and the name would be dead weight.
	Name string
	// CoVisited marks a row that entered the candidate pool because it was seen
	// together with a product the visitor opened, not because it is near their
	// taste centroid. It carries no cosine, so ranking must not read its zero
	// Similarity as "worst match".
	CoVisited bool
}

// EmbeddingRepository owns product_embedding and the catalog mirror.
type EmbeddingRepository interface {
	// CatalogProducts returns every sellable product with its current text.
	// It reads the catalog database through postgres_fdw (see app.go).
	CatalogProducts(ctx context.Context) ([]CatalogProduct, error)
	// CatalogProduct returns one product's current text, or nil when it is gone
	// or no longer sellable. This is the publish path's single-product read.
	CatalogProduct(ctx context.Context, productUUID uuid.UUID) (*CatalogProduct, error)
	// Delete removes a product's vector. A vector for a deleted product would
	// keep the ANN index offering something the catalog cannot describe.
	Delete(ctx context.Context, productUUID uuid.UUID) error
	// Upsert stores vectors for the given products, replacing prior rows.
	Upsert(ctx context.Context, items []ProductVector) error
	// EmbeddedHashes returns product_uuid -> source_hash for rows already
	// stored, so the backfill can skip products whose text did not change.
	EmbeddedHashes(ctx context.Context) (map[uuid.UUID]string, error)
	// Similar returns the nearest products to the given one, excluding itself.
	Similar(ctx context.Context, productUUID uuid.UUID, limit int) ([]SimilarRow, error)
	// NearestTo returns the products closest to an arbitrary vector. It is how
	// a taste centroid becomes a shelf.
	NearestTo(ctx context.Context, vector []float32, exclude []uuid.UUID, limit int) ([]SimilarRow, error)
	// Candidates returns a pool of neighbours of an arbitrary vector, widest
	// similarity order preserved. The caller re-ranks this pool by a blend of
	// similarity and popularity, which HNSW cannot do (it only orders by
	// distance), so this is deliberately over-fetched.
	Candidates(ctx context.Context, vector []float32, exclude []uuid.UUID, limit int) ([]SimilarRow, error)
	// NearestActive is NearestTo restricted to products the catalog still
	// sells *and* to the caller's facets. Hybrid search cannot hydrate its way
	// out of a bad vector hit the way a fixed-size shelf can: a page full of
	// archived or filtered-out products would leave the result short. Filtering
	// in SQL keeps the pool the caller asked for.
	NearestActive(ctx context.Context, vector []float32, filter FacetFilter, limit int) ([]SimilarRow, error)
	// ActiveAmong returns the subset of ids that are still sellable and match
	// the caller's facets. Query text lands in products through their
	// descriptions, so the keyword side has to be re-checked against the
	// catalog before fusion, and hydration cannot do it without also shrinking
	// the fused order.
	ActiveAmong(ctx context.Context, ids []uuid.UUID, filter FacetFilter) (map[uuid.UUID]struct{}, error)
	// ActiveCount counts sellable products matching the facets, optionally
	// restricted to ids. It answers "how many matches are there" for a ranked
	// search: a page-number row cannot show the last page without it, and
	// revealing one page per click is what happens when the count is guessed
	// from a page boundary. An empty ids slice means "count every match".
	ActiveCount(ctx context.Context, ids []uuid.UUID, filter FacetFilter) (int, error)
}

// FacetFilter is the shopper's explicit narrowing, shared by both halves of a
// hybrid search so neither can quietly widen the other.
//
// It is applied in SQL rather than as a post-filter on fused ids: filtering
// after fusion leaves holes in the ranking, and on a fixed-size page those
// holes surface as a short page for a query that actually had matches.
type FacetFilter struct {
	// CategoryUUID narrows to one department. Empty means every department.
	CategoryUUID uuid.UUID
	// MinPrice and MaxPrice are inclusive; zero means unbounded. The catalog
	// UI treats 0 as "no bound", so a free-product query uses the zero value.
	MinPrice  float64
	MaxPrice  float64
	MinRating float64
}

// constSQL renders the filter as one SQL fragment over the alias `p` (the
// mirrored catalog row), closing the argument numbering at *argNum, and
// returns the values to append. One builder keeps the vector query, the
// membership check and the count from drifting apart: a facet that is missing
// in any one of them would either leak filtered products into a page or make
// the page-number row promise results it cannot fill.
func (f FacetFilter) constSQL(argNum int) (string, []any) {
	clause := "p.product_deleted_at IS NULL AND p.product_status = 1"
	var args []any
	if f.CategoryUUID != uuid.Nil {
		argNum++
		// Resolve through the category table, not through a product: the
		// shopper sends the *category's* uuid, and it is welcome in the facets
		// before any product in that department exists.
		clause += fmt.Sprintf(` AND p.product_category_id = (
		SELECT c.category_id FROM catalog_mirror.categories c
		 WHERE c.category_uuid = $%d LIMIT 1)`, argNum)
		args = append(args, f.CategoryUUID)
	}
	if f.MinPrice > 0 {
		argNum++
		clause += fmt.Sprintf(" AND p.product_price >= $%d", argNum)
		args = append(args, f.MinPrice)
	}
	if f.MaxPrice > 0 {
		argNum++
		clause += fmt.Sprintf(" AND p.product_price <= $%d", argNum)
		args = append(args, f.MaxPrice)
	}
	if f.MinRating > 0 {
		argNum++
		clause += fmt.Sprintf(" AND p.product_avg_rating >= $%d", argNum)
		args = append(args, f.MinRating)
	}
	return clause, args
}

// ProductVector is one row ready to be written.
type ProductVector struct {
	ProductUUID uuid.UUID
	Vector      []float32
	Model       string
	SourceHash  string
}

type sqlEmbeddingRepository struct {
	pool *pgxpool.Pool
}

func NewSqlEmbeddingRepository(pool *pgxpool.Pool) EmbeddingRepository {
	return &sqlEmbeddingRepository{pool: pool}
}

// catalogSQL reads the mirrored catalog. status = 1 is active; archived or
// draft products never appear in a shelf, so embedding them is wasted work.
const catalogSQL = `
SELECT p.product_uuid, p.product_name, COALESCE(p.product_description, ''), COALESCE(c.category_name, '')
  FROM catalog_mirror.products p
  LEFT JOIN catalog_mirror.categories c ON c.category_id = p.product_category_id
 WHERE p.product_deleted_at IS NULL
   AND p.product_status = 1`

func (r *sqlEmbeddingRepository) CatalogProducts(ctx context.Context) ([]CatalogProduct, error) {
	rows, err := r.pool.Query(ctx, catalogSQL)
	if err != nil {
		return nil, fmt.Errorf("read catalog products: %w", err)
	}
	defer rows.Close()

	var out []CatalogProduct
	for rows.Next() {
		var p CatalogProduct
		if err := rows.Scan(&p.ProductUUID, &p.Name, &p.Description, &p.Category); err != nil {
			return nil, fmt.Errorf("scan catalog product: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// catalogOneSQL is catalogSQL narrowed to one row: pgx returns ErrNoRows for a
// product that was deleted or archived, which the caller treats as "drop the
// vector" rather than an error.
const catalogOneSQL = catalogSQL + `
   AND p.product_uuid = $1`

func (r *sqlEmbeddingRepository) CatalogProduct(ctx context.Context, productUUID uuid.UUID) (*CatalogProduct, error) {
	var p CatalogProduct
	err := r.pool.QueryRow(ctx, catalogOneSQL, productUUID).
		Scan(&p.ProductUUID, &p.Name, &p.Description, &p.Category)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read catalog product: %w", err)
	}
	return &p, nil
}

func (r *sqlEmbeddingRepository) Delete(ctx context.Context, productUUID uuid.UUID) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM product_embedding WHERE product_uuid = $1`, productUUID); err != nil {
		return fmt.Errorf("delete product embedding: %w", err)
	}
	return nil
}

// upsertSQL takes the vector as text and casts it: pgvector accepts
// '[0.1,0.2,...]' but pgx has no native codec for the vector type, and adding
// one means registering it on every pooled connection for no benefit at this
// write volume.
const upsertSQL = `
INSERT INTO product_embedding (product_uuid, embedding, model, source_hash, embedded_at)
VALUES ($1, $2::vector, $3, $4, now())
ON CONFLICT (product_uuid) DO UPDATE
   SET embedding   = EXCLUDED.embedding,
       model       = EXCLUDED.model,
       source_hash = EXCLUDED.source_hash,
       embedded_at = now()`

func (r *sqlEmbeddingRepository) Upsert(ctx context.Context, items []ProductVector) error {
	if len(items) == 0 {
		return nil
	}
	return sendBatched(ctx, r.pool, items)
}

func (r *sqlEmbeddingRepository) EmbeddedHashes(ctx context.Context) (map[uuid.UUID]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT product_uuid, source_hash FROM product_embedding`)
	if err != nil {
		return nil, fmt.Errorf("read embedded hashes: %w", err)
	}
	defer rows.Close()

	out := make(map[uuid.UUID]string)
	for rows.Next() {
		var id uuid.UUID
		var hash string
		if err := rows.Scan(&id, &hash); err != nil {
			return nil, fmt.Errorf("scan embedded hash: %w", err)
		}
		out[id] = hash
	}
	return out, rows.Err()
}

// similarSQL orders by cosine distance. `<=>` is pgvector's cosine operator and
// matches the HNSW index built with vector_cosine_ops, so the planner can use
// the index instead of scanning the table.
const similarSQL = `
SELECT e2.product_uuid, 1 - (e2.embedding <=> e1.embedding) AS similarity
  FROM product_embedding e1
  JOIN product_embedding e2 ON e2.product_uuid <> e1.product_uuid
 WHERE e1.product_uuid = $1
 ORDER BY e2.embedding <=> e1.embedding
 LIMIT $2`

func (r *sqlEmbeddingRepository) Similar(ctx context.Context, productUUID uuid.UUID, limit int) ([]SimilarRow, error) {
	rows, err := r.pool.Query(ctx, similarSQL, productUUID, limit)
	if err != nil {
		return nil, fmt.Errorf("similar products: %w", err)
	}
	return scanSimilar(rows)
}

const nearestSQL = `
SELECT product_uuid, 1 - (embedding <=> $1::vector) AS similarity
  FROM product_embedding
 WHERE NOT (product_uuid = ANY($2))
 ORDER BY embedding <=> $1::vector
 LIMIT $3`

func (r *sqlEmbeddingRepository) NearestTo(ctx context.Context, vector []float32, exclude []uuid.UUID, limit int) ([]SimilarRow, error) {
	excluded := exclude
	if excluded == nil {
		excluded = []uuid.UUID{}
	}
	rows, err := r.pool.Query(ctx, nearestSQL, VectorLiteral(vector), excluded, limit)
	if err != nil {
		return nil, fmt.Errorf("nearest products: %w", err)
	}
	return scanSimilar(rows)
}

// Candidates is NearestTo plus the product name.
//
// The shelf is grouped by brand before it is truncated, and brand is the first
// word of the name; joining the mirror here is one extra join on a query that
// already walks an index, and it keeps the grouping out of hydration, which
// drops rows it cannot describe (archived, out of stock) and would otherwise
// reorder the row after the fact.
const candidateSQL = `
SELECT e.product_uuid, 1 - (e.embedding <=> $1::vector) AS similarity, p.product_name
  FROM product_embedding e
  JOIN catalog_mirror.products p ON p.product_uuid = e.product_uuid
 WHERE NOT (e.product_uuid = ANY($2))
   AND p.product_status = 1
 ORDER BY e.embedding <=> $1::vector
 LIMIT $3`

func (r *sqlEmbeddingRepository) Candidates(ctx context.Context, vector []float32, exclude []uuid.UUID, limit int) ([]SimilarRow, error) {
	excluded := exclude
	if excluded == nil {
		excluded = []uuid.UUID{}
	}
	rows, err := r.pool.Query(ctx, candidateSQL, VectorLiteral(vector), excluded, limit)
	if err != nil {
		return nil, fmt.Errorf("candidate products: %w", err)
	}
	defer rows.Close()

	out := make([]SimilarRow, 0, limit)
	for rows.Next() {
		var row SimilarRow
		if err := rows.Scan(&row.ProductUUID, &row.Similarity, &row.Name); err != nil {
			return nil, fmt.Errorf("scan candidates: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// nearestActiveSQL is nearestSQL with the sellable-product predicate pushed into
// the query. The ORDER BY still drives the HNSW index; the EXISTS is a cheap
// post-filter on the few dozen rows the index walks, not a scan of the mirror.
// The facet fragment is appended by NearestActive, which owns its argument
// numbering and keyword args.
const nearestActiveSQL = `
SELECT e.product_uuid, 1 - (e.embedding <=> $1::vector) AS similarity
  FROM product_embedding e
 WHERE EXISTS (
       SELECT 1
         FROM catalog_mirror.products p
        WHERE p.product_uuid = e.product_uuid
          AND %s
 )

 ORDER BY e.embedding <=> $1::vector
 LIMIT $2`

func (r *sqlEmbeddingRepository) NearestActive(ctx context.Context, vector []float32, filter FacetFilter, limit int) ([]SimilarRow, error) {
	// $1 is the vector and $2 the limit, so facets start at index 2.
	clause, facetArgs := filter.constSQL(2)
	query := fmt.Sprintf(nearestActiveSQL, clause)
	args := append([]any{VectorLiteral(vector), limit}, facetArgs...)
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("nearest active products: %w", err)
	}
	return scanSimilar(rows)
}

// ActiveCount counts the ids that survive the facets, or every match when ids
// is empty. Same predicate as ActiveAmong by construction, so the page-number
// row can never promise a result the page itself would filter out.
func (r *sqlEmbeddingRepository) ActiveCount(ctx context.Context, ids []uuid.UUID, filter FacetFilter) (int, error) {
	start := 0
	if len(ids) > 0 {
		start = 1
	}
	clause, facetArgs := filter.constSQL(start)
	args := append([]any{}, facetArgs...)

	query := `SELECT COUNT(*) FROM catalog_mirror.products p WHERE ` + clause
	if len(ids) > 0 {
		query = `SELECT COUNT(*) FROM catalog_mirror.products p
 WHERE p.product_uuid = ANY($1) AND ` + clause
		args = append([]any{ids}, args...)
	}

	var count int
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active products: %w", err)
	}
	return count, nil
}

const activeAmongSQL = `
SELECT p.product_uuid
  FROM catalog_mirror.products p
 WHERE p.product_uuid = ANY($1)
   AND %s`

func (r *sqlEmbeddingRepository) ActiveAmong(ctx context.Context, ids []uuid.UUID, filter FacetFilter) (map[uuid.UUID]struct{}, error) {
	out := make(map[uuid.UUID]struct{}, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	// $1 is the id list, so facets start at index 1.
	clause, facetArgs := filter.constSQL(1)
	query := fmt.Sprintf(activeAmongSQL, clause)
	rows, err := r.pool.Query(ctx, query, append([]any{ids}, facetArgs...)...)
	if err != nil {
		return nil, fmt.Errorf("check active products: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan active product: %w", err)
		}
		out[id] = struct{}{}
	}
	return out, rows.Err()
}

func scanSimilar(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close()
}) ([]SimilarRow, error) {
	defer rows.Close()
	var out []SimilarRow
	for rows.Next() {
		var row SimilarRow
		if err := rows.Scan(&row.ProductUUID, &row.Similarity); err != nil {
			return nil, fmt.Errorf("scan similar row: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
