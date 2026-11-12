package service

import (
	recsrepository "goshop/services/recommendations/internal/repository"
)

// maxPerBucket is how many products of one brand may occupy a shelf.
//
// It scales with the shelf: a card-per-brand rule of two filled only four of
// twelve slots when the nearest neighbourhood held two brands. The floor keeps
// a two-card row varied, the ceiling keeps a long row from becoming one brand.
func maxPerBucket(limit int) int {
	return min(max(limit/4, 2), 4)
}

// diversify picks at most `limit` candidates, at most two per brand.
//
// The pool arrives in rank order, and walking it top-down is what makes a shelf
// repetitive: when the vector index returns sixty near-neighbours of one
// concept, they share a brand and occupy every slot. This takes one card per
// brand first, then a second card per brand, so rank decides which product wins
// its brand and breadth decides how fast the row moves on.
//
// Brand is the first word of the product name, exactly as the catalog writes it
// ("GoPro Z Body Prime 105 #5"). It is a heuristic rather than a column: it
// breaks on names that open with a descriptor.
//
// The cap is absolute. A pool of four brands yields eight cards from a
// twelve-card request rather than repeating a brand; a short row is honest, and
// the caller's fallback covers an empty one. Repeating a brand to fill the row
// is the bug this function exists to fix.
//
// ponytail: brand only. Add category to the key when one department can offer
// twelve brands, which today it cannot (8 categories x ~9 brands, two per brand
// already fills a twelve-card row).
func diversify(rows []recsrepository.SimilarRow, limit int) []recsrepository.SimilarRow {
	if limit <= 0 || len(rows) == 0 {
		return nil
	}
	brandOf := make([]string, len(rows))
	for i, row := range rows {
		brandOf[i] = firstWord(row.Name)
	}

	out := make([]recsrepository.SimilarRow, 0, min(limit, len(rows)))
	taken := make([]bool, len(rows))
	counts := make(map[string]int, 8)
	cap := maxPerBucket(limit)
	// Co-visitation widens the pool; it does not own the row. Without this
	// ceiling the pair table supplies most cards on a catalog whose vector
	// neighbourhood is one brand, which trades "all Samsung" for "no taste".
	coLimit := max(limit/3, 1)
	coUsed := 0

	for pass := 0; pass < cap && len(out) < limit; pass++ {
		for i := range rows {
			if len(out) >= limit {
				return out
			}
			if taken[i] || counts[brandOf[i]] != pass {
				continue
			}
			if rows[i].CoVisited {
				if coUsed >= coLimit {
					continue
				}
				coUsed++
			}
			taken[i] = true
			counts[brandOf[i]]++
			out = append(out, rows[i])
		}
	}
	return out
}

// firstWord is the brand as this catalog spells it.
func firstWord(name string) string {
	for i, r := range name {
		if r == ' ' {
			return name[:i]
		}
	}
	return name
}
