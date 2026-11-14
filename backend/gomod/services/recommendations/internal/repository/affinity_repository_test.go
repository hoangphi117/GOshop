package repository

import (
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// The key scheme is decoded in two places (handler writes it, this query reads
// it), so a drift here silently empties every profile.
func TestSplitVisitorKey(t *testing.T) {
	id, anon, err := SplitVisitorKey("u:42")
	if err != nil || id == nil || *id != 42 || anon != nil {
		t.Fatalf("u:42 => id=%v anon=%v err=%v", id, anon, err)
	}

	want := uuid.New()
	anonID, anon, err := SplitVisitorKey("a:" + want.String())
	if err != nil || anon == nil || *anon != want || anonID != nil {
		t.Fatalf("a:<uuid> => id=%v anon=%v err=%v", anonID, anon, err)
	}

	// Empty and malformed keys mean "no profile", never an error: a bad cookie
	// must not turn the homepage shelf into a 500.
	for _, key := range []string{"", "u:", "u:notanumber", "a:not-a-uuid", "x:1"} {
		id, anon, err := SplitVisitorKey(key)
		if err != nil || id != nil || anon != nil {
			t.Fatalf("%q should decode to no profile, got id=%v anon=%v err=%v", key, id, anon, err)
		}
	}
}

// Both affinity queries are positional, so a placeholder that is off by one
// fails at runtime, not at compile time, and the session tier degrades to a
// warning while the profile quietly answers instead. That is how the session
// tier ran for a while without firing at all.
//
// This counts the shape, not the semantics: one $n per parameter, and every
// $n from 1 to that count used.
func TestAffinityParamsAreContiguous(t *testing.T) {
	for name, tc := range map[string]struct {
		sql   string
		count int
	}{
		"top":     {topAffinitySQL, 9},
		"session": {sessionAffinitySQL, 8},
		"search":  {searchSeedSQL, 5},
	} {
		seen := map[int]bool{}
		for _, m := range regexp.MustCompile(`\$(\d+)`).FindAllStringSubmatch(tc.sql, -1) {
			n := mustAtoi(m[1])
			seen[n] = true
			if n < 1 || n > tc.count {
				t.Fatalf("%s: $%d is out of range 1..%d", name, n, tc.count)
			}
		}
		for n := 1; n <= tc.count; n++ {
			if !seen[n] {
				t.Fatalf("%s: parameter $%d is never used", name, n)
			}
		}
	}
}

func mustAtoi(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}

// The session tier must not pick up an event that carries no session id. An
// empty string sorts as the newest session (it is not NULL), and on a UUID
// column it also fails to compare at all, so the filter is on the text form.
func TestSessionSQLIgnoresEmptySessionID(t *testing.T) {
	if !strings.Contains(sessionAffinitySQL, "session_id::text <> ''") {
		t.Fatal("sessionAffinitySQL lost the empty-session guard")
	}
}
