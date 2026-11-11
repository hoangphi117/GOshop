package repository

import "testing"

// Regression: the assistant sends whole sentences to GET /api/v1/faqs/search.
// The old ILIKE-only match returned zero rows for "return policy how many days
// to return", so the assistant told customers the shop had no return policy.
func TestToTSQueryBuildsORQuery(t *testing.T) {
	cases := map[string]string{
		"return policy how many days to return": "return | policy | how | many | days | to | return",
		"  Return!  ":                           "return",
		// Quotes, pipes, parens and colons are tsquery syntax; they must not
		// reach Postgres as operators.
		`a'|&!:*()return`: "return",
		"":                "zzzznomatch",
		"x":               "zzzznomatch",
	}
	for in, want := range cases {
		if got := toTSQuery(in); got != want {
			t.Errorf("toTSQuery(%q) = %q, want %q", in, got, want)
		}
	}
}
