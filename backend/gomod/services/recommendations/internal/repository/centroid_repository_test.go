package repository

import (
	"math"
	"testing"

	"github.com/google/uuid"
)

func affinity(u string, score float64) AffinityRow {
	return AffinityRow{VisitorKey: "u:1", ProductUUID: uuid.MustParse(u), Score: score}
}

func norm(v []float32) float64 {
	var s float64
	for _, x := range v {
		s += float64(x) * float64(x)
	}
	return math.Sqrt(s)
}

const (
	u1 = "11111111-1111-1111-1111-111111111111"
	u2 = "22222222-2222-2222-2222-222222222222"
	u3 = "33333333-3333-3333-3333-333333333333"
)

// A purchase (score 5) must outvote a browse (score 1): the centroid has to sit
// near the purchased product's vector, not midway between the two.
func TestWeightedCentroidPullsTowardHighScore(t *testing.T) {
	rows := []AffinityRow{affinity(u1, 5), affinity(u2, 1)}
	vectors := map[uuid.UUID][]float32{
		uuid.MustParse(u1): {1, 0, 0},
		uuid.MustParse(u2): {0, 1, 0},
	}

	got, seen, err := weightedCentroid(rows, vectors)
	if err != nil {
		t.Fatalf("centroid: %v", err)
	}
	if len(seen) != 2 {
		t.Fatalf("expected both products marked as seen, got %d", len(seen))
	}
	if got[0] <= got[1] {
		t.Fatalf("centroid is not weighted toward the higher score: %v", got)
	}
	if math.Abs(norm(got)-1) > 1e-6 {
		t.Fatalf("centroid is not unit length: %v", norm(got))
	}
}

func TestWeightedCentroidSkipsMissingVectorsButStillMarksSeen(t *testing.T) {
	rows := []AffinityRow{affinity(u1, 5), affinity(u2, 1), affinity(u3, 2)}
	vectors := map[uuid.UUID][]float32{
		uuid.MustParse(u1): {1, 0, 0},
	}

	got, seen, err := weightedCentroid(rows, vectors)
	if err != nil {
		t.Fatalf("centroid: %v", err)
	}
	if got == nil {
		t.Fatal("expected a centroid from the embedded seed")
	}
	if len(seen) != 3 {
		t.Fatalf("unembedded products must still be excluded from the shelf, got %d", len(seen))
	}
}

// No vectors at all is not an error: the visitor simply has no personalised
// shelf yet, and the caller falls back.
func TestWeightedCentroidWithoutVectorsReturnsNil(t *testing.T) {
	got, seen, err := weightedCentroid([]AffinityRow{affinity(u1, 5)}, map[uuid.UUID][]float32{})
	if err != nil {
		t.Fatalf("centroid: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil centroid, got %v", got)
	}
	if len(seen) != 1 {
		t.Fatalf("expected the seed to be marked seen, got %d", len(seen))
	}
}

func TestWeightedCentroidRejectsMixedDimensions(t *testing.T) {
	rows := []AffinityRow{affinity(u1, 1), affinity(u2, 1)}
	vectors := map[uuid.UUID][]float32{
		uuid.MustParse(u1): {1, 0, 0},
		uuid.MustParse(u2): {1, 0},
	}
	if _, _, err := weightedCentroid(rows, vectors); err == nil {
		t.Fatal("expected an error for mismatched vector dimensions")
	}
}

func TestVectorLiteralFormat(t *testing.T) {
	if got := VectorLiteral([]float32{0.1, 0.25}); got != "[0.1,0.25]" {
		t.Fatalf("unexpected literal: %s", got)
	}
	if got := VectorLiteral(nil); got != "[]" {
		t.Fatalf("unexpected empty literal: %s", got)
	}
}
