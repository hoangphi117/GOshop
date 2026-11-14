package repository

import (
	"strings"
	"testing"
)

// The predicate is a const, so the only thing a unit test can assert is its
// shape. That is still worth doing: the join between the two identity trails is
// an OR list where one missing branch silently resets every shelf on login
// instead of erroring, so a revert would ship as "personalization is bad" rather
// than "the filter is broken".
func TestVisitorPredicateCoversBothIdentityTrails(t *testing.T) {
	t.Parallel()

	// Each branch answers one case. Losing any of them is a visible regression:
	//   user_id = $1      -> what the account did while signed in
	//   anon_id = $2      -> what this device did while signed out
	//   user_id IN (...)  -> the account's earlier signed-out browsing
	//   anon_id IN (...)  -> this device's earlier signed-in browsing
	want := []string{
		"user_id = $1::int",
		"anon_id = $2::uuid",
		"SELECT DISTINCT user_id FROM analytics_mirror.user_events\n            WHERE anon_id = $2::uuid AND user_id IS NOT NULL",
		"SELECT DISTINCT anon_id FROM analytics_mirror.user_events\n            WHERE user_id = $1::int AND anon_id IS NOT NULL",
	}
	for _, branch := range want {
		if !strings.Contains(visitorPredicate, branch) {
			t.Errorf("predicate is missing identity branch:\n%s\ngot:\n%s", branch, visitorPredicate)
		}
	}

	// The old filter made the anon branch conditional on the visitor being
	// anonymous. Keeping that guard would drop the signed-out trail for a signed
	// in visitor, which is the bug this predicate exists to fix.
	if strings.Contains(visitorPredicate, "$1::int IS NULL AND anon_id") {
		t.Errorf("anon branch must not be gated on $1 being NULL: %s", visitorPredicate)
	}
}
