package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"goshop/shared/pkg/llm"
)

// Regression: three real FAQ answers plus their questions used to overflow
// maxToolResultChars, so toolResult cut the JSON mid-object. The model then got
// unparseable data and answered "I couldn't find any return policy information"
// even though the catalog had the row. The payload must always stay valid JSON.
func TestSearchFAQsPayloadStaysValidJSON(t *testing.T) {
	// Sized to the real seed data: these three answers total ~1300 chars.
	long := strings.Repeat("Returns are accepted within 7 days of delivery. ", 3)
	faqs := []FAQ{
		{Question: "How do I return an item?", Answer: long},
		{Question: "How long does a refund take?", Answer: long},
		{Question: "Can I exchange instead of returning?", Answer: long},
	}
	svc := NewChatService(Options{LLM: &fakeLLM{}, FAQs: &fakeFAQs{items: faqs}}).(*chatService)

	out, ok := svc.executeTool(context.Background(),
		llm.ToolCall{Name: "search_faqs", Arguments: `{"query":"return policy"}`},
		map[string]Product{})
	if !ok {
		t.Fatalf("tool reported no results: %s", out)
	}

	var modelView []FAQ
	if err := json.Unmarshal([]byte(extractPayload(out)), &modelView); err != nil {
		t.Fatalf("payload is not valid JSON: %v\n%s", err, out)
	}
	if len(modelView) != len(faqs) {
		t.Fatalf("model saw %d FAQs, want %d", len(modelView), len(faqs))
	}
	// The retention window is the sentence the customer actually asked about;
	// it has to survive the per-field cap, and the whole payload has to fit
	// under the compiler's ceiling without being cut.
	if !strings.Contains(modelView[0].Answer, "7 days") {
		t.Fatalf("answer lost its policy detail: %q", modelView[0].Answer)
	}
	if strings.Contains(out, truncationMarker) {
		t.Fatalf("payload was truncated: %s", out)
	}
}
