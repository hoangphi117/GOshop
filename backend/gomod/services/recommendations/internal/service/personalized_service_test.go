package service

import (
	"testing"

	"github.com/google/uuid"

	recsrepository "goshop/services/recommendations/internal/repository"
)

// The blend is the only place a popularity number meets a similarity number, so
// it is the only place the shelf can silently reorder into "bestsellers with a
// tilt". These cases pin the two properties that must hold.
func TestBlend(t *testing.T) {
	near, far, dead := uuid.New(), uuid.New(), uuid.New()
	rows := []recsrepository.SimilarRow{
		{ProductUUID: near, Similarity: 0.95},
		{ProductUUID: far, Similarity: 0.60},
		{ProductUUID: dead, Similarity: 0.90},
	}
	units := map[uuid.UUID]int64{near: 100, far: 100, dead: 0}

	out := blend(rows, units, 100)
	if len(out) != 3 {
		t.Fatalf("blend dropped rows: got %d", len(out))
	}

	// A strong match that also sells must stay first.
	if out[0].ProductUUID != near {
		t.Fatalf("expected the popular strong match first, got %s", out[0].ProductUUID)
	}
	// Dead stock at 0.90 must lose to a far-but-selling item at 0.60:
	// 0.7*0.60 + 0.3*1.0 = 0.72 beats 0.7*0.90 + 0 = 0.63.
	if out[1].ProductUUID != far {
		t.Fatalf("popularity did not demote dead stock: order=%v", uuids(out))
	}
	if out[2].ProductUUID != dead {
		t.Fatalf("dead stock should rank last, got %s", out[2].ProductUUID)
	}
}

// Without a rollup the ordering must be untouched: normalising every product to
// the same value would not reorder anything, but it would rescale the published
// similarity and make the numbers lie about how close the match is.
func TestBlendSkipsWithoutRollup(t *testing.T) {
	if got := blend(nil, nil, 0); len(got) != 0 {
		t.Fatalf("blend over nothing should stay empty, got %d", len(got))
	}
}

func uuids(rows []recsrepository.SimilarRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.ProductUUID.String()
	}
	return out
}
