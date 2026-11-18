package handler

import "testing"

// HEALTH_SKIP is the escape hatch for a dependency a deployment runs without
// (Redis, in the current Render setup). The parse must survive the shapes an
// operator actually types into an env field.
func TestParseSkipList(t *testing.T) {
	cases := map[string]map[string]bool{
		"":                  {},
		"redis":             {"redis": true},
		"redis,messaging":   {"redis": true, "messaging": true},
		" redis , Storage ": {"redis": true, "storage": true},
		"redis,,redis":      {"redis": true},
	}

	for raw, want := range cases {
		got := parseSkipList(raw)
		if len(got) != len(want) {
			t.Fatalf("%q: got %v, want %v", raw, got, want)
		}
		for k := range want {
			if !got[k] {
				t.Fatalf("%q: missing %q in %v", raw, k, got)
			}
		}
	}
}
