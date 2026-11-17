package service

import "testing"

// A grounded, complete answer must not offer a human. The old marker list
// contained "our team", so "Let me connect you with our team" and any stray
// mention of the team lit the hand-off flag, and real policy answers showed a
// pointless "Talk to a human" button.
func TestIsEscalationIgnoresCompleteAnswers(t *testing.T) {
	complete := []string{
		"Returns are accepted within 7 days of delivery. To start one, go to Account > Orders, choose the order, then Request return.",
		"Once we receive and inspect the item, refunds are issued within 3-5 business days.",
		"Yes, cash on delivery is available nationwide for orders up to 20,000,000 VND.",
	}
	for _, text := range complete {
		if isEscalation(text) {
			t.Errorf("flagged a complete answer as an escalation: %q", text)
		}
	}
}

func TestIsEscalationCatchesHandOffs(t *testing.T) {
	handOffs := []string{
		"I cannot answer that from our shop data. Let me connect you with our team.",
		"I couldn't find any products in the catalog for that.",
		"Our product search isn't working right now, so I can't confirm the price.",
		"Please contact support for that.",
	}
	for _, text := range handOffs {
		if !isEscalation(text) {
			t.Errorf("missed a hand-off: %q", text)
		}
	}
}
