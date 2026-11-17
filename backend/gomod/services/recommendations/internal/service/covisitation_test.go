package service

import (
	"testing"

	"github.com/google/uuid"

	recsrepository "goshop/services/recommendations/internal/repository"
)

// TestBlendKeepsCoVisitedInTheTail pins the promise appendCoVisited makes: a
// co-visited row has no cosine, so a zero must not be read as "worst match" and
// must not let it outrank products the visitor's own history points at.
func TestBlendKeepsCoVisitedInTheTail(t *testing.T) {
	hot := row("Nokia A", 0.80)
	cold := row("Samsung B", 0.10)
	co := recsrepository.SimilarRow{ProductUUID: uuid.New(), Name: "GoPro C", CoVisited: true}

	out := blend([]recsrepository.SimilarRow{hot, co, cold}, nil, 0)
	got := []string{out[0].Name, out[1].Name, out[2].Name}
	want := []string{"Nokia A", "Samsung B", "GoPro C"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

// TestBlendCoVisitedKeepsSimilarityZero: the row keeps its zero so the card is
// not given a fabricated score, and ranking still puts it last.
func TestBlendCoVisitedKeepsSimilarityZero(t *testing.T) {
	co := recsrepository.SimilarRow{ProductUUID: uuid.New(), Name: "GoPro C", CoVisited: true}
	out := blend([]recsrepository.SimilarRow{co, row("Nokia A", 0.5)}, nil, 0)
	if !out[1].CoVisited || out[1].Similarity != 0 {
		t.Fatalf("co-visited row = %+v, want CoVisited with Similarity 0", out[1])
	}
}

// TestDiversifyTreatsCoVisitedByBrand: a co-visited row carries a name, so the
// brand cap counts it like any other candidate instead of giving it its own
// bucket.
func TestDiversifyTreatsCoVisitedByBrand(t *testing.T) {
	var rows []recsrepository.SimilarRow
	for i := 0; i < 6; i++ {
		rows = append(rows, row("Samsung S #"+string(rune('a'+i)), 0.83-float64(i)/1000))
	}
	co := recsrepository.SimilarRow{ProductUUID: uuid.New(), Name: "Samsung T #1", CoVisited: true}
	rows = append(rows, co)

	out := diversify(rows, 12)
	if got := brands(out)["Samsung"]; got != cap12 {
		t.Fatalf("Samsung took %d slots, want %d (a co-visited Samsung counts toward the cap)", got, cap12)
	}
}
