package repository

import (
	"context"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CentroidRepository turns an affinity profile into a single point in
// embedding space.
type CentroidRepository interface {
	// Centroid returns the score-weighted mean of the vectors of the given
	// products, plus every product that was interacted with.
	//
	// The returned seen list is what the caller must exclude from the resulting
	// shelf. It includes products with no vector: a missing embedding is a
	// backfill gap, not a reason to show something the visitor already saw.
	// A nil centroid means none of the products has been embedded yet.
	Centroid(ctx context.Context, rows []AffinityRow) (centroid []float32, seen []uuid.UUID, err error)

	// SearchCentroid folds search keywords into the same point. Each keyword
	// borrows the mean vector of the results that were clicked out of it, so a
	// shopper who searched but never browsed still gets a shelf.
	//
	// The keyword's vector is averaged over its clicks rather than summed: a
	// query with ten clicks is not ten times the interest of one with a single
	// click, it is just the same interest expressed more broadly. Clicked
	// products join `seen` so the shelf cannot echo the result just opened.
	//
	// Same nil contract as Centroid: nothing embedded means no signal.
	SearchCentroid(ctx context.Context, seeds []QuerySeed) (centroid []float32, seen []uuid.UUID, err error)
}

type sqlCentroidRepository struct {
	pool *pgxpool.Pool
}

func NewSqlCentroidRepository(pool *pgxpool.Pool) CentroidRepository {
	return &sqlCentroidRepository{pool: pool}
}

// centroidSQL returns the stored vectors for the seeds. pgvector has no
// weighted-average aggregate, so the mean is computed in Go where the affinity
// weights (computed in affinity_repository) are already available.
const centroidSQL = `
SELECT product_uuid, embedding::real[]
  FROM product_embedding
 WHERE product_uuid = ANY($1)`

func (r *sqlCentroidRepository) Centroid(ctx context.Context, rows []AffinityRow) ([]float32, []uuid.UUID, error) {
	if len(rows) == 0 {
		return nil, nil, nil
	}
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ProductUUID)
	}

	vectors, err := r.loadVectors(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	return weightedCentroid(rows, vectors)
}

func (r *sqlCentroidRepository) loadVectors(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]float32, error) {
	rows, err := r.pool.Query(ctx, centroidSQL, ids)
	if err != nil {
		return nil, fmt.Errorf("load seed vectors: %w", err)
	}
	defer rows.Close()

	out := make(map[uuid.UUID][]float32, len(ids))
	for rows.Next() {
		var id uuid.UUID
		var vec []float32
		if err := rows.Scan(&id, &vec); err != nil {
			return nil, fmt.Errorf("scan seed vector: %w", err)
		}
		out[id] = vec
	}
	return out, rows.Err()
}

// weightedCentroid is the pure part: score-weighted mean, normalised to unit
// length so cosine distance treats it exactly like a stored product vector.
func weightedCentroid(rows []AffinityRow, vectors map[uuid.UUID][]float32) ([]float32, []uuid.UUID, error) {
	seen := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		seen = append(seen, row.ProductUUID)
	}

	var sum []float64
	total := 0.0
	for _, row := range rows {
		vec, ok := vectors[row.ProductUUID]
		// A zero or negative score is not an interest. Skipping it also keeps
		// the divisor away from zero.
		if !ok || row.Score <= 0 {
			continue
		}
		if sum == nil {
			sum = make([]float64, len(vec))
		}
		if len(vec) != len(sum) {
			return nil, seen, fmt.Errorf("seed vector %s has %d dims, expected %d", row.ProductUUID, len(vec), len(sum))
		}
		total += row.Score
		for i, x := range vec {
			sum[i] += float64(x) * row.Score
		}
	}
	if total == 0 {
		return nil, seen, nil
	}
	return normalize(sum, total), seen, nil
}

// SearchCentroid resolves each keyword to the mean vector of its clicks, then
// weights those per-keyword vectors by the keyword's score.
func (r *sqlCentroidRepository) SearchCentroid(_ context.Context, seeds []QuerySeed) ([]float32, []uuid.UUID, error) {
	return searchCentroidWith(seeds, r)
}

// searchCentroidWith is the testable body: it needs only loadVectors, so a fake
// loader can stand in for the pool.
func searchCentroidWith(seeds []QuerySeed, loader interface {
	loadVectors(context.Context, []uuid.UUID) (map[uuid.UUID][]float32, error)
}) ([]float32, []uuid.UUID, error) {
	if len(seeds) == 0 {
		return nil, nil, nil
	}
	ids := make([]uuid.UUID, 0, len(seeds)*searchClickCap)
	seen := make([]uuid.UUID, 0, len(seeds)*searchClickCap)
	for _, seed := range seeds {
		for _, id := range seed.Clicked {
			ids = append(ids, id)
			seen = append(seen, id)
		}
	}
	vectors, err := loader.loadVectors(context.Background(), ids)
	if err != nil {
		return nil, nil, err
	}

	var sum []float64
	total := 0.0
	for _, seed := range seeds {
		if seed.Score <= 0 {
			continue
		}
		var perQuery []float64
		clicks := 0
		for _, id := range seed.Clicked {
			vec, ok := vectors[id]
			if !ok {
				// Clicked but not embedded: a backfill gap. It still counts as
				// seen, it just cannot steer the direction.
				continue
			}
			if perQuery == nil {
				perQuery = make([]float64, len(vec))
			}
			if len(vec) != len(perQuery) {
				return nil, seen, fmt.Errorf("clicked vector %s has %d dims, expected %d", id, len(vec), len(perQuery))
			}
			for i, x := range vec {
				perQuery[i] += float64(x)
			}
			clicks++
		}
		if clicks == 0 {
			continue
		}
		if sum == nil {
			sum = make([]float64, len(perQuery))
		}
		if len(perQuery) != len(sum) {
			return nil, seen, fmt.Errorf("query %q has %d dims, expected %d", seed.Query, len(perQuery), len(sum))
		}
		// Divide by the click count so the keyword weighs `Score`, not
		// `Score * clicks`, and a broad query cannot outvote a focused one.
		scale := seed.Score / float64(clicks)
		for i, x := range perQuery {
			sum[i] += x * scale
		}
		total += seed.Score
	}
	if total == 0 {
		return nil, seen, nil
	}
	return normalize(sum, total), seen, nil
}

// normalize divides by total and rescales to unit length.
func normalize(sum []float64, total float64) []float32 {
	centroid := make([]float32, len(sum))
	var norm float64
	for i, x := range sum {
		v := float32(x / total)
		centroid[i] = v
		norm += float64(v) * float64(v)
	}
	if norm > 0 {
		inv := float32(1 / math.Sqrt(norm))
		for i := range centroid {
			centroid[i] *= inv
		}
	}
	return centroid
}
