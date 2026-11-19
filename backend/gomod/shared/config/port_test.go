package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Env values arrive with stray characters from a copy-paste or a dashboard
// form. A trailing backtick in a port turned into
//
//	dial tcp: lookup tcp/8081`: unknown port
//
// on Render, after the listener had already logged that it started, so the
// failure surfaced as a crash loop instead of a config error. Port() is the one
// place that strips them, and this test pins the behaviour.
func TestPort_NormalisesCommonPasteDamage(t *testing.T) {
	cases := map[string]string{
		"8081":     "8081",
		" 8081 ":   "8081",
		"8081`":    "8081",
		"`8081`":   "8081",
		"8081\r":   "8081",
		"8081\n":   "8081",
		"\"8081\"": "8081",
		"':8081'":  "8081",
	}
	for raw, want := range cases {
		assert.Equal(t, want, Port(raw), "raw %q", raw)
	}
}

// An empty or unusable value must fall back, not produce a listener on ":".
func TestPort_InvalidFallsBack(t *testing.T) {
	assert.Equal(t, "8080", Port(""))
	assert.Equal(t, "8080", Port("`"))
	assert.Equal(t, "8080", Port("   "))
	assert.Equal(t, "8080", Port("notaport"))
}

func TestServerAddress_UsesPortNormalisation(t *testing.T) {
	t.Setenv("SERVER_PORT", "8081`")
	assert.Equal(t, ":8081", (&Config{}).serverAddress())

	t.Setenv("SERVER_PORT", "")
	assert.Equal(t, ":8080", (&Config{}).serverAddress())
	assert.True(t, strings.HasPrefix((&Config{}).serverAddress(), ":"))
}
