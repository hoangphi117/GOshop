package service

import (
	"math"
	"testing"
)

// Ids chosen so alphabetical order and score order disagree: a tie-break that
// fell back to map iteration would produce a different page on every call.
const (
	idA = "aaaaaaaa-0000-0000-0000-000000000000"
	idB = "bbbbbbbb-0000-0000-0000-000000000000"
	idC = "cccccccc-0000-0000-0000-000000000000"
	idD = "dddddddd-0000-0000-0000-000000000000"
)

// Agreement at the top of both lists must beat a strong rank in one list only:
// that is the whole reason to fuse rather than pick a side.
func TestFuseRanksRewardsAgreement(t *testing.T) {
	keyword := []string{idA, idB, idC}
	vector := []string{idC, idA, idD}

	got := fuseRanks(keyword, vector)
	if len(got) != 4 {
		t.Fatalf("every id from either side must survive, got %v", got)
	}
	// A is rank 1 + rank 2, C is rank 3 + rank 1: close, but A wins because it
	// is strong on both. The exact winner matters less than the fact that the
	// two lists are combined at all.
	if got[0] != idA {
		t.Fatalf("expected the both-sides hit at rank 1, got %v", got)
	}
}

// A product only the vector side found must still appear: synonym recall is the
// entire point of the semantic half.
func TestFuseRanksKeepsUnionNotIntersection(t *testing.T) {
	got := fuseRanks([]string{idA}, []string{idA, idB})
	seen := map[string]bool{}
	for _, id := range got {
		seen[id] = true
	}
	if !seen[idB] {
		t.Fatalf("vector-only hit dropped: %v", got)
	}
}

// Weighting must let meaning outrank a rank-1 keyword hit when the vector side
// puts that product far down. With equal weights this test would invert.
func TestFuseRanksPrefersMeaningOnConflict(t *testing.T) {
	keyword := []string{idA, idB}
	vector := []string{idB, idC}

	got := fuseRanks(keyword, vector)
	if got[0] != idB {
		t.Fatalf("expected the vector-favoured id to win, got %v", got)
	}
}

func TestFuseRanksTieBreaksDeterministically(t *testing.T) {
	// Same rank in disjoint lists: two calls must agree, or a paginated result
	// shows the same product on two pages.
	keyword := []string{idA}
	vector := []string{idB}
	first := fuseRanks(keyword, vector)
	for i := 0; i < 20; i++ {
		again := fuseRanks(keyword, vector)
		if again[0] != first[0] || again[1] != first[1] {
			t.Fatalf("unstable order: %v vs %v", first, again)
		}
	}
}

func TestFuseRanksEmptySides(t *testing.T) {
	if got := fuseRanks(nil, nil); len(got) != 0 {
		t.Fatalf("expected no ids, got %v", got)
	}
	if got := fuseRanks(nil, []string{idA}); len(got) != 1 || got[0] != idA {
		t.Fatalf("vector-only list must pass through, got %v", got)
	}
}

func TestNormalizeQueryText(t *testing.T) {
	if got := normalizeQueryText("  wireless   earbuds \t"); got != "wireless earbuds" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeQueryText("   "); got != "" {
		t.Fatalf("whitespace-only query must normalize to empty, got %q", got)
	}
}

func TestCosine(t *testing.T) {
	if got := cosine([]float32{1, 0}, []float32{1, 0}); math.Abs(got-1) > 1e-9 {
		t.Fatalf("identical vectors must have cosine 1, got %v", got)
	}
	if got := cosine([]float32{1, 0}, []float32{0, 1}); math.Abs(got) > 1e-9 {
		t.Fatalf("orthogonal vectors must have cosine 0, got %v", got)
	}
}
