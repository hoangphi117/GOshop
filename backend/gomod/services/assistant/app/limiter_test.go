package app

import (
	"context"
	"testing"

	assistantservice "goshop/services/assistant/internal/service"
)

// countLimiter records how many calls each bucket saw.
type countLimiter struct{ calls int }

func (c *countLimiter) Allow(context.Context, string) (bool, error) {
	c.calls++
	return true, nil
}

func TestCallerLimiterRoutesByPrefix(t *testing.T) {
	var member, guest countLimiter
	l := callerLimiter{member: &member, guest: &guest}

	for _, tc := range []struct {
		key  string
		want *countLimiter
	}{
		{"user:42", &member},
		{"sess:abc", &guest},
		{"", &guest},
		{"usr:7", &guest}, // near-miss prefix must not unlock the member bucket
	} {
		member.calls, guest.calls = 0, 0
		if _, err := l.Allow(context.Background(), tc.key); err != nil {
			t.Fatalf("Allow(%q): %v", tc.key, err)
		}
		if tc.want.calls != 1 {
			t.Fatalf("key %q went to the wrong bucket (member=%d guest=%d)",
				tc.key, member.calls, guest.calls)
		}
	}
}

// A nil redis client (unit tests, degraded boot) must never block chat.
func TestNewCallerLimiterDegradesWithoutRedis(t *testing.T) {
	l := newCallerLimiter(nil, 40)
	for _, key := range []string{"user:1", "sess:x"} {
		for i := 0; i < 50; i++ {
			if ok, err := l.Allow(context.Background(), key); !ok || err != nil {
				t.Fatalf("%s call %d = (%v, %v), want allowed", key, i, ok, err)
			}
		}
	}
	_ = assistantservice.Limiter(nil)
}
