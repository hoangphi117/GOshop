package repository

import (
	"context"
	"math"
	"testing"

	"github.com/google/uuid"
)

// fakeVectorLoader stands in for loadVectors so SearchCentroid can be tested
// without a database.
type fakeVectorLoader map[uuid.UUID][]float32

func (f fakeVectorLoader) loadVectors(context.Context, []uuid.UUID) (map[uuid.UUID][]float32, error) {
	return f, nil
}

func searchCentroid(t *testing.T, seeds []QuerySeed, vectors fakeVectorLoader) ([]float32, []uuid.UUID) {
	t.Helper()
	centroid, seen, err := searchCentroidWith(seeds, vectors)
	if err != nil {
		t.Fatalf("search centroid: %v", err)
	}
	return centroid, seen
}

func TestSearchCentroidAveragesClicksPerQuery(t *testing.T) {
	// One keyword, two clicks pointing the same way. Averaging them must not
	// change the direction, and the result must still be unit length.
	vectors := fakeVectorLoader{
		uuid.MustParse(u1): {1, 0, 0},
		uuid.MustParse(u2): {1, 0, 0},
	}
	seeds := []QuerySeed{{Query: "mechanical keyboard", Score: 1, Clicked: []uuid.UUID{uuid.MustParse(u1), uuid.MustParse(u2)}}}

	got, seen := searchCentroid(t, seeds, vectors)
	if got == nil {
		t.Fatal("expected a centroid from clicked products")
	}
	if math.Abs(norm(got)-1) > 1e-6 {
		t.Fatalf("centroid is not unit length: %v", norm(got))
	}
	if got[0] < 0.99 {
		t.Fatalf("centroid should point at the clicked products, got %v", got)
	}
	if len(seen) != 2 {
		t.Fatalf("both clicked products must be excluded from the shelf, got %d", len(seen))
	}
}

// A query with several clicks must not outweigh a focused query just because it
// produced more rows: per-query vectors are averaged first.
func TestSearchCentroidDoesNotLetBroadQueryDominate(t *testing.T) {
	vectors := fakeVectorLoader{
		uuid.MustParse(u1): {1, 0, 0},
		uuid.MustParse(u2): {0, 1, 0},
		uuid.MustParse(u3): {0, 0, 1},
	}
	seeds := []QuerySeed{
		{Query: "scattered", Score: 1, Clicked: []uuid.UUID{uuid.MustParse(u2), uuid.MustParse(u3)}},
		{Query: "focused", Score: 1, Clicked: []uuid.UUID{uuid.MustParse(u1)}},
	}

	got, _ := searchCentroid(t, seeds, vectors)
	// Two queries of equal score: the focused one contributes a full unit
	// vector, the scattered one contributes its unit-length mean. Neither can
	// dominate by row count.
	if got[0] <= 0 {
		t.Fatalf("focused query lost its signal entirely: %v", got)
	}
	if math.Abs(norm(got)-1) > 1e-6 {
		t.Fatalf("centroid is not unit length: %v", norm(got))
	}
}

func TestSearchCentroidSkipsUnembeddedClicksButKeepsThemSeen(t *testing.T) {
	// u2 was clicked but never embedded; u1 carries the direction. The shelf
	// must still not recommend u2.
	vectors := fakeVectorLoader{uuid.MustParse(u1): {1, 0, 0}}
	seeds := []QuerySeed{{Query: "amplifier", Score: 1, Clicked: []uuid.UUID{uuid.MustParse(u1), uuid.MustParse(u2)}}}

	got, seen := searchCentroid(t, seeds, vectors)
	if got == nil {
		t.Fatal("expected a centroid from the embedded click")
	}
	if len(seen) != 2 {
		t.Fatalf("unembedded click must still be excluded, got %d", len(seen))
	}
}

func TestSearchCentroidWithoutClicksReturnsNil(t *testing.T) {
	got, seen := searchCentroid(t, []QuerySeed{{Query: "nothing", Score: 1}}, fakeVectorLoader{})
	if got != nil {
		t.Fatalf("expected nil centroid, got %v", got)
	}
	if len(seen) != 0 {
		t.Fatalf("expected no seen products, got %d", len(seen))
	}
}
