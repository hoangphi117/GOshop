package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"goshop/shared/pkg/llm"
	"goshop/shared/pkg/utils"
)

// fakeLLM replays a scripted list of replies and records what it was asked.
type fakeLLM struct {
	replies []llm.Reply
	calls   [][]llm.Message
	err     error
}

func (f *fakeLLM) Complete(_ context.Context, msgs []llm.Message, tools []llm.Tool) (llm.Reply, error) {
	f.calls = append(f.calls, append([]llm.Message(nil), msgs...))
	if f.err != nil {
		return llm.Reply{}, f.err
	}
	if len(f.replies) == 0 {
		return llm.Reply{Content: "fallback"}, nil
	}
	r := f.replies[0]
	f.replies = f.replies[1:]
	return r, nil
}

type fakeProducts struct {
	items   []Product
	queries []string
	limits  []int
}

func (f *fakeProducts) Search(_ context.Context, q string, limit int) ([]Product, error) {
	f.queries = append(f.queries, q)
	f.limits = append(f.limits, limit)
	return f.items, nil
}

type fakeReader struct{ p Product }

func (f *fakeReader) Get(context.Context, string) (Product, error) { return f.p, nil }

type fakeFAQs struct{ items []FAQ }

func (f *fakeFAQs) Search(context.Context, string, int) ([]FAQ, error) { return f.items, nil }

type denyLimiter struct{}

func (denyLimiter) Allow(context.Context, string) (bool, error) { return false, nil }

type memSessions struct{ appended []llm.Message }

func (m *memSessions) Load(context.Context, string) ([]llm.Message, error) { return nil, nil }
func (m *memSessions) Append(_ context.Context, _ string, msgs ...llm.Message) error {
	m.appended = append(m.appended, msgs...)
	return nil
}

func lantern() Product {
	return Product{UUID: "p-1", Name: "Bamboo Lantern", Slug: "bamboo-lantern", Price: 250000, Stock: 4}
}

// The happy path: the model asks for a product search, gets the result, then
// answers. The tool result must reach the second model call.
func TestAskRunsToolThenAnswers(t *testing.T) {
	products := &fakeProducts{items: []Product{lantern()}}
	sessions := &memSessions{}
	fake := &fakeLLM{replies: []llm.Reply{
		{
			ToolCalls:    []llm.ToolCall{{ID: "call_1", Name: "search_products", Arguments: `{"query":"lantern"}`}},
			FinishReason: "tool_calls",
			Usage:        llm.Usage{PromptTokens: 100, CompletionTokens: 10},
		},
		{
			Content:      "We have the Bamboo Lantern for 250000 VND.",
			FinishReason: "stop",
			Usage:        llm.Usage{PromptTokens: 150, CompletionTokens: 20, CachedPromptTokens: 80},
		},
	}}

	svc := NewChatService(Options{LLM: fake, Products: products, Sessions: sessions, Model: "test-model"})
	got, err := svc.Ask(context.Background(), "user:1", "sess-1", nil, "do you have a lantern?")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}

	if !strings.Contains(got.Answer, "Bamboo Lantern") {
		t.Fatalf("Answer = %q", got.Answer)
	}
	if len(got.Products) != 1 || got.Products[0].UUID != "p-1" {
		t.Fatalf("Products = %+v", got.Products)
	}
	if got.ToolCalls != 1 {
		t.Fatalf("ToolCalls = %d, want 1", got.ToolCalls)
	}
	if got.PromptTokens != 250 || got.CompletionTokens != 30 || got.CachedTokens != 80 {
		t.Fatalf("usage = %+v", got)
	}
	if len(products.queries) != 1 || products.queries[0] != "lantern" {
		t.Fatalf("search queries = %v", products.queries)
	}
	if products.limits[0] != defaultProductLimit {
		t.Fatalf("limit = %d, want default %d", products.limits[0], defaultProductLimit)
	}

	if len(fake.calls) != 2 {
		t.Fatalf("llm calls = %d, want 2", len(fake.calls))
	}
	second := fake.calls[1]
	last := second[len(second)-1]
	if last.Role != llm.RoleTool || last.ToolCallID != "call_1" {
		t.Fatalf("last message = %+v, want tool result for call_1", last)
	}
	if !strings.Contains(last.Content, "p-1") {
		t.Fatalf("tool result missing product: %s", last.Content)
	}

	if len(sessions.appended) != 2 {
		t.Fatalf("session appended %d messages, want 2", len(sessions.appended))
	}
}

func TestAskReturnsServiceUnavailableWhenProviderFails(t *testing.T) {
	fake := &fakeLLM{err: llm.ErrUnavailable}
	svc := NewChatService(Options{LLM: fake, Products: &fakeProducts{}})

	_, err := svc.Ask(context.Background(), "user:1", "", nil, "hi")
	var appErr *utils.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("err = %v, want *utils.AppError", err)
	}
	if appErr.Code != utils.ErrCodeServiceUnavailable {
		t.Fatalf("code = %s, want %s", appErr.Code, utils.ErrCodeServiceUnavailable)
	}
}

func TestAskRejectsEmptyAndOversizedQuestion(t *testing.T) {
	svc := NewChatService(Options{LLM: &fakeLLM{}})

	if _, err := svc.Ask(context.Background(), "k", "", nil, "   "); err == nil {
		t.Fatal("expected error for empty question")
	}
	big := strings.Repeat("x", maxQuestionChars+1)
	if _, err := svc.Ask(context.Background(), "k", "", nil, big); err == nil {
		t.Fatal("expected error for oversized question")
	}
}

func TestAskHonoursQuota(t *testing.T) {
	svc := NewChatService(Options{LLM: &fakeLLM{}, Limiter: denyLimiter{}})
	_, err := svc.Ask(context.Background(), "user:1", "", nil, "hi")

	var appErr *utils.AppError
	if !errors.As(err, &appErr) || appErr.Code != utils.ErrCodeTooManyRequests {
		t.Fatalf("err = %v, want TOO_MANY_REQUESTS", err)
	}
}

// An unknown tool must not crash the loop: the model gets an error payload and
// the next round can still answer.
func TestAskSurvivesUnknownTool(t *testing.T) {
	fake := &fakeLLM{replies: []llm.Reply{
		{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "drop_database", Arguments: `{}`}}},
		{Content: "I can help with products and policies."},
	}}
	svc := NewChatService(Options{LLM: fake})

	got, err := svc.Ask(context.Background(), "k", "", nil, "hi")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if got.ToolCalls != 1 {
		t.Fatalf("ToolCalls = %d", got.ToolCalls)
	}
	last := fake.calls[1][len(fake.calls[1])-1]
	if !strings.Contains(last.Content, "unknown tool") {
		t.Fatalf("tool error payload = %s", last.Content)
	}
}

// Exhausting the loop without a final answer must escalate to a human.
func TestAskFallsBackAfterToolLoopExhausted(t *testing.T) {
	replies := make([]llm.Reply, maxToolRounds+2)
	for i := range replies {
		replies[i] = llm.Reply{ToolCalls: []llm.ToolCall{{ID: "c", Name: "search_faqs", Arguments: `{"query":"x"}`}}}
	}
	svc := NewChatService(Options{LLM: &fakeLLM{replies: replies}, FAQs: &fakeFAQs{}})

	got, err := svc.Ask(context.Background(), "k", "", nil, "hi")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if !got.FallbackToHuman {
		t.Fatal("FallbackToHuman = false, want true after loop exhaustion")
	}
	if got.ToolCalls != maxToolRounds {
		t.Fatalf("ToolCalls = %d, want %d", got.ToolCalls, maxToolRounds)
	}
}

func TestToolArgumentValidation(t *testing.T) {
	// Malformed arguments must produce a model-readable error, not a panic.
	products := &fakeProducts{}
	svc := NewChatService(Options{LLM: &fakeLLM{}, Products: products}).(*chatService)
	out, ok := svc.executeTool(context.Background(),
		llm.ToolCall{Name: "search_products", Arguments: `not json`}, map[string]Product{})
	if ok || !strings.Contains(out, "error") {
		t.Fatalf("out = %s, ok = %v", out, ok)
	}
	if len(products.queries) != 0 {
		t.Fatal("catalog was called with malformed arguments")
	}
}

func TestProductJSONShape(t *testing.T) {
	// The model reads this JSON; keep the wire shape stable.
	b, _ := json.Marshal([]Product{lantern()})
	for _, want := range []string{`"product_name":"Bamboo Lantern"`, `"product_price":250000`, `"product_stock":4`} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("product json %s missing %s", b, want)
		}
	}
}

// The model escalating in its own text must still set the hand-off flag: the
// frontend uses it to show "Talk to a human", and an out-of-scope question is
// the exact case that needs that button.
func TestAskSetsFallbackWhenModelEscalates(t *testing.T) {
	fake := &fakeLLM{replies: []llm.Reply{
		{Content: "I cannot answer that from our shop data. Let me connect you with our team."},
	}}
	svc := NewChatService(Options{LLM: fake, Sessions: &memSessions{}, Model: "test-model"})

	got, err := svc.Ask(context.Background(), "user:1", "sess-1", nil, "who won the 1998 World Cup?")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if !got.FallbackToHuman {
		t.Fatalf("FallbackToHuman = false for an escalating answer %q", got.Answer)
	}
}

// A model that invents an answer after its tool came back empty is still shown
// a hand-off: the wording is what the prompt tells it to use ("couldn't find"),
// and a non-grounded confident answer must never stand on its own. The old
// structural rule (no products → hand-off) was dropped because it fired on every
// correct FAQ answer.
func TestAskSetsFallbackWhenToolsReturnNothing(t *testing.T) {
	fake := &fakeLLM{replies: []llm.Reply{
		{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "search_products", Arguments: `{"query":"unicorn"}`}}},
		{Content: "I couldn't find any matching products. Let me connect you with a human agent."},
	}}
	svc := NewChatService(Options{
		LLM:      fake,
		Products: &fakeProducts{items: nil},
		Sessions: &memSessions{},
		Model:    "test-model",
	})

	got, err := svc.Ask(context.Background(), "user:1", "sess-1", nil, "do you have a unicorn?")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if !got.FallbackToHuman {
		t.Fatalf("FallbackToHuman = false after empty tool results %q", got.Answer)
	}
}

// A grounded answer must not set the flag, or every reply shows the button.
func TestAskDoesNotFlagGroundedAnswer(t *testing.T) {
	fake := &fakeLLM{replies: []llm.Reply{
		{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "search_products", Arguments: `{"query":"lantern"}`}}},
		{Content: "Yes, the Bamboo Lantern is in stock at 250000 VND."},
	}}
	svc := NewChatService(Options{
		LLM:      fake,
		Products: &fakeProducts{items: []Product{{UUID: "p-1", Name: "Bamboo Lantern", Price: 250000, Stock: 4}}},
		Sessions: &memSessions{},
		Model:    "test-model",
	})

	got, err := svc.Ask(context.Background(), "user:1", "sess-1", nil, "do you have a lantern?")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if got.FallbackToHuman {
		t.Fatalf("FallbackToHuman = true for a grounded answer %q", got.Answer)
	}
}
