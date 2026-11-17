package service

import (
	"testing"
	"time"

	"github.com/google/uuid"

	analyticsdto "goshop/services/analytics/internal/dto"
)

// The event validator is the trust boundary: identity is assumed filled from
// the session, but everything inside the body is client-controlled.
func TestBuildEvent(t *testing.T) {
	anon := uuid.New()
	product := uuid.NewString()
	now := time.Now().UTC()

	ok := buildInput(product)
	ev, err := buildEvent(ok, &anon, nil, now)
	if err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}
	if ev.Query == nil || *ev.Query != "gaming laptop" {
		t.Fatalf("query not normalised, got %v", ev.Query)
	}

	cases := []struct {
		name  string
		input analyticsdto.EngagementEventInput
	}{
		{"unknown type", func() analyticsdto.EngagementEventInput { i := ok; i.EventType = "page_view"; return i }()},
		{"bad uuid", func() analyticsdto.EngagementEventInput { i := ok; i.EventID = "nope"; return i }()},
		{"search without query", func() analyticsdto.EngagementEventInput { i := ok; i.Query = "   "; return i }()},
		{"view without product", func() analyticsdto.EngagementEventInput {
			i := ok
			i.EventType = "view_item"
			i.Query = ""
			i.ProductUUID = ""
			return i
		}()},
		{"future timestamp", func() analyticsdto.EngagementEventInput {
			i := ok
			i.OccurredAt = now.Add(time.Hour).Format(time.RFC3339)
			return i
		}()},
		{"stale timestamp", func() analyticsdto.EngagementEventInput {
			i := ok
			i.OccurredAt = now.Add(-30 * 24 * time.Hour).Format(time.RFC3339)
			return i
		}()},
	}
	for _, tc := range cases {
		if _, err := buildEvent(tc.input, &anon, nil, now); err == nil {
			t.Errorf("%s: expected rejection, got none", tc.name)
		}
	}
}

// User identity wins over the anon cookie so history survives a cookie clear.
func TestSubjectPrefersUserID(t *testing.T) {
	kind, id, err := subject(uuid.NewString(), 42)
	if err != nil || kind != "user" || id != "42" {
		t.Fatalf("got %q/%q/%v", kind, id, err)
	}
	if _, _, err := subject("not-a-uuid", 0); err == nil {
		t.Fatal("expected error for malformed anon id with no user")
	}
}

func buildInput(product string) analyticsdto.EngagementEventInput {
	return analyticsdto.EngagementEventInput{
		EventID:     uuid.NewString(),
		EventType:   "search",
		OccurredAt:  time.Now().UTC().Format(time.RFC3339),
		Query:       "  gaming   laptop ",
		ProductUUID: product,
	}
}
