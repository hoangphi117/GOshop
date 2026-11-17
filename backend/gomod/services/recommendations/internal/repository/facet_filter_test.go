package repository

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TestFacetFilterSQL pins the generated predicate. The fragment is spliced into
// three queries (vector, membership, count), so a facet that silently renders no
// clause would leak filtered products into a page while the total stayed
// filtered — the class of bug a page-number row cannot show.
func TestFacetFilterSQL(t *testing.T) {
	empty, args := FacetFilter{}.constSQL(2)
	if len(args) != 0 {
		t.Fatalf("empty filter should take no arguments, got %d", len(args))
	}
	if empty != "p.product_deleted_at IS NULL AND p.product_status = 1" {
		t.Fatalf("empty filter lost the sellable predicate: %q", empty)
	}

	category := uuid.New()
	full, args := FacetFilter{
		CategoryUUID: category,
		MinPrice:     10,
		MaxPrice:     99.5,
		MinRating:    4,
	}.constSQL(2)

	// Argument numbering starts where the caller says, so ids and the vector can
	// keep $1 and $2 without the fragment colliding with them.
	for _, placeholder := range []string{"$3", "$4", "$5", "$6"} {
		if !strings.Contains(full, placeholder) {
			t.Errorf("missing %s in %q", placeholder, full)
		}
	}
	if strings.Contains(full, "$2") || strings.Contains(full, "$1") {
		t.Errorf("filter reuses a caller argument slot: %q", full)
	}
	if !strings.Contains(full, "p.product_deleted_at IS NULL") || !strings.Contains(full, "p.product_status = 1") {
		t.Errorf("filter dropped the sellable predicate: %q", full)
	}
	if len(args) != 4 || args[0] != category || args[1] != 10.0 || args[2] != 99.5 || args[3] != 4.0 {
		t.Errorf("unexpected args: %v", args)
	}

	// Zero is the "unbounded" sentinel the storefront uses for price, so it must
	// not render a `>= 0` clause (and must not shift the numbering above).
	onlyMin, args := FacetFilter{MinPrice: 5}.constSQL(1)
	if strings.Contains(onlyMin, "$3") || len(args) != 1 {
		t.Errorf("single facet rendered extra slots: %q %v", onlyMin, args)
	}
}
