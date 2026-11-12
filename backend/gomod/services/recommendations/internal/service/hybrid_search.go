package service

import (
	"math"
	"sort"
	"strings"
)

// Reciprocal-rank-fusion constant. Established practice for fusing two ranked
// lists: a document's contribution is 1/(k + rank), so agreement near the top
// dominates and agreement at rank 50 barely matters. 60 is the value from the
// original RRF paper.
const rrfK = 60

// fusionDepth is how deep each side is fetched before fusing. Deep enough that
// a hit ranked 40th in one list can still win once the other agrees with it, and
// shallow enough that page 1 never waits on a large ANN walk.
const fusionDepth = 50

// Fusion weights. Meaning outranks text: a shopper typing "wireless earbuds"
// wants something that *is* wireless earbuds, not something whose description
// happens to contain both words. Pure RRF would treat the two lists as equally
// trustworthy, which is the wrong prior for a semantic model this good.
const (
	fusionWeightVector  = 1.3
	fusionWeightKeyword = 1.0
)

// HybridScorer fuses a keyword ranking and a vector ranking into one order.
//
// It is pure so it can be tested without Meilisearch, pgvector or an embedder;
// the inputs are exactly what the two retrievers return.
type HybridScorer struct {
	// Keyword holds product ids in best-first order, already restricted to the
	// caller's filters. An unfiltered keyword list would fuse in products the
	// visitor explicitly excluded with a price or category facet.
	Keyword []string
	// Vector holds product ids in nearest-first order, filtered the same way.
	Vector []string
}

// Score returns product ids best-first, then the per-id fused score.
//
// A product that only one side found still enters the result: the keyword side
// can match an exact SKU the embedder never saw, and the vector side can find a
// synonym ("earbuds" -> "in-ear headphones") the index never wrote down. That is
// the entire point of a hybrid; intersecting the two lists first would throw it
// away.
func (h HybridScorer) Score() ([]string, map[string]float64) {
	scores := make(map[string]float64, len(h.Keyword)+len(h.Vector))

	for rank, id := range h.Keyword {
		scores[id] += fusionWeightKeyword / float64(rrfK+rank+1)
	}
	for rank, id := range h.Vector {
		scores[id] += fusionWeightVector / float64(rrfK+rank+1)
	}

	ids := make([]string, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}
	// Deterministic tie-break: map iteration order is random, and a paginated
	// result whose ties shuffle between pages shows the same product twice.
	sort.Slice(ids, func(i, j int) bool {
		if scores[ids[i]] != scores[ids[j]] {
			return scores[ids[i]] > scores[ids[j]]
		}
		return ids[i] < ids[j]
	})
	return ids, scores
}

// fuseRanks is Score with the map elided, for callers that only need the order.
func fuseRanks(keyword, vector []string) []string {
	ids, _ := HybridScorer{Keyword: keyword, Vector: vector}.Score()
	return ids
}

// normalizeQueryText collapses whitespace so two spellings of the same query hit
// the same vector and the same batch of results. Nothing else is rewritten: the
// embedding model has no search syntax of its own, and stemming or lowercasing
// here would only drift from what Meilisearch does with the same string.
func normalizeQueryText(q string) string {
	return strings.Join(strings.Fields(q), " ")
}

// cosine is used only by tests, to assert a centroid handed to the vector side
// still points where it should; the database does the real distance maths.
func cosine(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		if i >= len(b) {
			break
		}
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
