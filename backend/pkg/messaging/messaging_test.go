package messaging

import "testing"

func TestHeaderGetSet(t *testing.T) {
	var h Header
	if got := h.Get("x-retry-count"); got != "" {
		t.Fatalf("empty header Get = %q, want \"\"", got)
	}
	h.Set("x-retry-count", "1")
	h.Set("trace_id", "abc")
	if got := h.Get("x-retry-count"); got != "1" {
		t.Fatalf("Get after Set = %q, want %q", got, "1")
	}
	if got := h.Get("missing"); got != "" {
		t.Fatalf("Get missing key = %q, want \"\"", got)
	}
}
