package service

import (
	"testing"

	"github.com/google/uuid"

	recsrepository "goshop/services/recommendations/internal/repository"
)

// cap12 is the brand cap for the storefront's twelve-card row.
const cap12 = 3

// row builds a candidate the way Candidates returns one: ranked, with the name
// the brand is read from.
func row(name string, similarity float64) recsrepository.SimilarRow {
	return recsrepository.SimilarRow{ProductUUID: uuid.New(), Name: name, Similarity: similarity}
}

func brands(rows []recsrepository.SimilarRow) map[string]int {
	out := make(map[string]int, len(rows))
	for _, r := range rows {
		out[firstWord(r.Name)]++
	}
	return out
}

// TestMaxPerBucketScalesWithShelf: the cap is a share of the row, not a
// constant. Two per brand filled four of twelve slots on the storefront.
func TestMaxPerBucketScalesWithShelf(t *testing.T) {
	for _, tc := range []struct{ limit, want int }{{2, 2}, {8, 2}, {12, 3}, {20, 4}, {50, 4}} {
		if got := maxPerBucket(tc.limit); got != tc.want {
			t.Errorf("maxPerBucket(%d) = %d, want %d", tc.limit, got, tc.want)
		}
	}
}

// TestDiversifyBreaksUpOneBrand is the reported shelf: the nearest neighbours of
// a phone-heavy profile were almost all Samsung. A pool of two brands cannot
// fill twelve slots under a cap of three without repeating a brand, so the honest
// answer is six cards — not twelve with Samsung nine times.
func TestDiversifyBreaksUpOneBrand(t *testing.T) {
	var rows []recsrepository.SimilarRow
	for i := 0; i < 40; i++ {
		rows = append(rows, row("Samsung Pixel #"+string(rune('a'+i%26)), 0.83-float64(i)/1000))
	}
	for i := 0; i < 11; i++ {
		rows = append(rows, row("Nokia Pixel #"+string(rune('a'+i)), 0.80-float64(i)/1000))
	}

	out := diversify(rows, 12)
	if len(out) != 6 {
		t.Fatalf("shelf has %d cards, want 6 (2 brands x cap %d)", len(out), cap12)
	}
	for brand, n := range brands(out) {
		if n > cap12 {
			t.Errorf("brand %q took %d slots, cap is %d", brand, n, cap12)
		}
	}
	if got := brands(out)["Nokia"]; got != 3 {
		t.Errorf("Nokia took %d slots, want its full share of 3 over a much deeper Samsung pool", got)
	}
	if out[0].ProductUUID != rows[0].ProductUUID {
		t.Errorf("first card = %q, want the top-ranked %q", out[0].Name, rows[0].Name)
	}
}

// TestDiversifyFillsWhenPoolIsWide: four brands at the top of the neighbourhood
// is what the deeper pool returns, and that fills the twelve-card row exactly.
func TestDiversifyFillsWhenPoolIsWide(t *testing.T) {
	var rows []recsrepository.SimilarRow
	for _, brand := range []string{"Samsung", "Nokia", "Google", "GoPro"} {
		for i := 0; i < 10; i++ {
			rows = append(rows, row(brand+" Item #"+string(rune('a'+i)), 0.82-float64(i)/1000))
		}
	}
	out := diversify(rows, 12)
	if len(out) != 12 {
		t.Fatalf("shelf has %d cards, want 12", len(out))
	}
	for brand, n := range brands(out) {
		if n > cap12 {
			t.Errorf("brand %q took %d slots, cap is %d", brand, n, cap12)
		}
	}
}

// TestDiversifyKeepsTopOfRank: diversity reorders the tail, it does not demote
// the answer.
func TestDiversifyKeepsTopOfRank(t *testing.T) {
	rows := []recsrepository.SimilarRow{
		row("Nokia B #2", 0.9),
		row("Nokia A #1", 0.8),
		row("GoPro D #4", 0.1),
	}
	out := diversify(rows, 2)
	if out[0].ProductUUID != rows[0].ProductUUID {
		t.Fatalf("first card = %q, want the top-ranked %q", out[0].Name, rows[0].Name)
	}
	if out[1].ProductUUID != rows[2].ProductUUID {
		t.Fatalf("second card = %q, want the lower-ranked other brand", out[1].Name)
	}
}

// TestDiversifyHonoursLimitAndEmptyPool pins the two edges the caller relies on.
func TestDiversifyHonoursLimitAndEmptyPool(t *testing.T) {
	if got := diversify(nil, 12); got != nil {
		t.Fatalf("empty pool returned %d rows", len(got))
	}
	rows := []recsrepository.SimilarRow{row("Nokia A", 0.9), row("GoPro B", 0.8), row("Sony C", 0.7)}
	if got := diversify(rows, 0); got != nil {
		t.Fatalf("limit 0 returned %d rows", len(got))
	}
	if got := diversify(rows, 99); len(got) != len(rows) {
		t.Fatalf("limit past the pool returned %d rows, want %d", len(got), len(rows))
	}
}
