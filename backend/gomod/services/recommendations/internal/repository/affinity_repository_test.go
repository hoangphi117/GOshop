package repository

import (
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
