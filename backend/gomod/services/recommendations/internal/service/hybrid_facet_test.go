package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	recsrepository "goshop/services/recommendations/internal/repository"
)

// recordingSuggester captures the narrowing the keyword half was asked for.
// The facet has to reach Meilisearch, not just the post-fusion check: an
// unfiltered keyword list would spend the whole fusion pool on products the
// shopper excluded, and the page would come back short.
type recordingSuggester struct {
	got  SearchNarrowing
	ids  []string
	calls int
}

func (r *recordingSuggester) Suggest(_ context.Context, _ string, narrow SearchNarrowing, _ int) ([]string, error) {
	r.got = narrow
	r.calls++
	return r.ids, nil
}

// TestHybridSearchCarriesFacets pins the plumbing, which is the part that fails
// silently: a dropped facet changes the result set but never raises an error.
func TestHybridSearchCarriesFacets(t *testing.T) {
	category := uuid.New()
	suggester := &recordingSuggester{}

	svc := &personalizedService{
		suggester: suggester,
		embeddings: &fakeEmbeddingRepo{
			products: []recsrepository.CatalogProduct{{ProductUUID: uuid.New()}},
		},
	}
	in := HybridQuery{
		Query: "keyboard",
		Page:  1,
		Limit: 20,
		Narrowing: SearchNarrowing{
			CategoryUUID: category,
			MinPrice:     10,
			MaxPrice:     20,
			MinRating:    4,
		},
	}

	if _, err := svc.HybridSearch(context.Background(), in); err != nil {
		t.Fatalf("hybrid search: %v", err)
	}
	if suggester.calls != 1 {
		t.Fatalf("keyword side not consulted: %d calls", suggester.calls)
	}
	if suggester.got != in.Narrowing {
		t.Fatalf("keyword side got %+v, want %+v", suggester.got, in.Narrowing)
	}
}

// TestHybridSearchEmptyHits checks the degenerate path. With no hits at all the
// repository's "empty ids means everything" rule would otherwise turn an empty
// search into a whole-catalog count.
func TestHybridSearchEmptyHits(t *testing.T) {
	svc := &personalizedService{
		suggester:  &recordingSuggester{},
		embeddings: &fakeEmbeddingRepo{products: []recsrepository.CatalogProduct{{ProductUUID: uuid.New()}}},
	}

	shelf, err := svc.HybridSearch(context.Background(), HybridQuery{Query: "keyboard", Page: 1, Limit: 20})
	if err != nil {
		t.Fatalf("hybrid search: %v", err)
	}
	if shelf.Total != 0 || len(shelf.Items) != 0 {
		t.Fatalf("empty hits reported total=%d items=%d", shelf.Total, len(shelf.Items))
	}
	if shelf.Source != SourceKeyword {
		t.Fatalf("source=%q, want %q", shelf.Source, SourceKeyword)
	}
}

// TestSearchNarrowingFacetFilter keeps the two representations in step. They are
// separate types because Meilisearch's grammar differs from the SQL predicate,
// but only these four fields exist: dropping one here silently disables it in
// SQL while the keyword side still honours it.
func TestSearchNarrowingFacetFilter(t *testing.T) {
	category := uuid.New()
	narrow := SearchNarrowing{CategoryUUID: category, MinPrice: 1, MaxPrice: 2, MinRating: 3}
	got := narrow.facetFilter()

	if got.CategoryUUID != category || got.MinPrice != 1 || got.MaxPrice != 2 || got.MinRating != 3 {
		t.Fatalf("facet filter lost a field: %+v", got)
	}
}
