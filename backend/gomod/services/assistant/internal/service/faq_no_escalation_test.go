package service

import (
	"testing"
)

// Regression: every FAQ answer arrives as a single search_faqs result, so
// len(seen)==0 is normal for a policy question. The rule was "tool called but no
// data" → hand-off, which flagged correct answers. Nothing found is already
// detectable from the answer text (notFoundMarkers), so a grounded FAQ must not
// count as an empty tool run.
func TestIsEscalationTreatsAnsweredFAQAsNoEscalation(t *testing.T) {
	answer := "Returns are accepted within 7 days of delivery. To start one, go to Account > Orders, then choose Request return."
	if isEscalation(answer) {
		t.Fatalf("grounded FAQ answer flagged as escalation: %q", answer)
	}
}
