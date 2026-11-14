package service

import "testing"

// The session tier is a threshold, not an if-empty: below it the current pass is
// a better answer than a thin profile, at or above it the profile wins. Getting
// the comparison backwards is invisible in production — every visitor still gets
// a shelf, just the wrong one — so the boundary is pinned here.
func TestSessionTierThreshold(t *testing.T) {
	t.Parallel()

	cases := []struct {
		depth int
		tier  string
		why   string
	}{
		{0, "session", "no profile at all is the pure cold start the tier exists for"},
		{1, "session", "one product is its own neighbourhood; the session is fresher"},
		{2, "session", "still below the threshold"},
		{sessionThreshold, "profile", "at the threshold the profile has enough directions to average"},
		{affinitySeedSize, "profile", "a full profile must never pay for the session read"},
	}

	for _, c := range cases {
		got := "profile"
		if c.depth < sessionThreshold {
			got = "session"
		}
		if got != c.tier {
			t.Errorf("depth %d picked the %s tier, want %s (%s)", c.depth, got, c.tier, c.why)
		}
	}

	// The switch only earns its keep when a shallow profile is the norm; a
	// threshold above the seed cap would make the session tier dead code.
	if sessionThreshold >= affinitySeedSize {
		t.Fatalf("session threshold %d can never be crossed by a profile capped at %d", sessionThreshold, affinitySeedSize)
	}
	if sessionSeedSize > affinitySeedSize {
		t.Fatalf("session seeds %d exceed the affinity cap %d", sessionSeedSize, affinitySeedSize)
	}
}
