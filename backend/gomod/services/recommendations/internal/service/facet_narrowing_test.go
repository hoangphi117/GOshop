package service

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TestMeiliFilterOmitsRating guards the one facet Meilisearch cannot express.
// product_avg_rating is not a filterable attribute on the index, and sending it
// makes Meilisearch reject the entire query, which would drop every filtered
// search to its vector half with no error the shopper can see.
func TestMeiliFilterOmitsRating(t *testing.T) {
	category := uuid.New()
	filter := SearchNarrowing{
		CategoryUUID: category,
		MinPrice:     10,
		MaxPrice:     20,
		MinRating:    4,
	}.meiliFilter()
	if strings.Contains(filter, "product_avg_rating") {
		t.Fatalf("filter must not mention product_avg_rating: %q", filter)
	}
	for _, want := range []string{
		"category_uuid = '" + category.String() + "'",
		"product_price >= 10",
		"product_price <= 20",
	} {
		if !strings.Contains(filter, want) {
			t.Errorf("missing %q in %q", want, filter)
		}
	}
	// Zero price bounds mean "unbounded", never "free": rendering them would
	// turn a category-only filter into an empty result set.
	if got := (SearchNarrowing{}).meiliFilter(); got != "" {
		t.Errorf("empty narrowing should render no filter, got %q", got)
	}
}
