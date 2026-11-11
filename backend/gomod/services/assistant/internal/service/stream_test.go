package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"goshop/shared/pkg/llm"
	"goshop/shared/pkg/utils"
)

// streamingFake is fakeLLM plus the optional streaming half. Each scripted
// reply's Content is split into deltas so the test can assert the order.
type streamingFake struct {
	fakeLLM
	chunkSize int
}

func (f *streamingFake) CompleteStream(
	_ context.Context,
	msgs []llm.Message,
	_ []llm.Tool,
	emit func(llm.Chunk) error,
) (llm.Reply, error) {
	f.calls = append(f.calls, append([]llm.Message(nil), msgs...))
	if f.err != nil {
		return llm.Reply{}, f.err
	}
	if len(f.replies) == 0 {
		return llm.Reply{}, nil
	}
	r := f.replies[0]
	f.replies = f.replies[1:]

	size := f.chunkSize
	if size <= 0 {
		size = 4
	}
	for i := 0; i < len(r.Content); i += size {
		end := min(i+size, len(r.Content))
		if err := emit(llm.Chunk{Delta: r.Content[i:end]}); err != nil {
			return llm.Reply{}, err
		}
	}
	if err := emit(llm.Chunk{ToolCalls: r.ToolCalls, FinishReason: r.FinishReason}); err != nil {
		return llm.Reply{}, err
	}
	return r, nil
}

func collectDeltas() (func(string) error, *strings.Builder) {
	var b strings.Builder
	return func(d string) error {
		b.WriteString(d)
		return nil
	}, &b
}

// Streaming must produce the same answer as Ask, delivered in pieces.
func TestAskStreamEmitsDeltasInOrder(t *testing.T) {
	fake := &streamingFake{chunkSize: 5}
	fake.replies = []llm.Reply{{
		Content:      "We have the Bamboo Lantern for 250000 VND.",
		FinishReason: "stop",
		Usage:        llm.Usage{PromptTokens: 12, CompletionTokens: 9},
	}}

	svc := NewChatService(Options{LLM: fake, Model: "test-model"})
	emit, out := collectDeltas()
	got, err := svc.AskStream(context.Background(), "user:1", "", nil, "lantern?", emit)
	if err != nil {
		t.Fatalf("AskStream: %v", err)
	}
	if out.String() != got.Answer {
		t.Fatalf("deltas = %q, answer = %q", out.String(), got.Answer)
	}
	if got.PromptTokens != 12 || got.CompletionTokens != 9 {
		t.Fatalf("usage = %+v", got)
	}
}

// A tool round must still work over the streaming path, and only the final
// answer's text reaches emit (tool rounds carry no content).
func TestAskStreamRunsToolThenAnswers(t *testing.T) {
	fake := &streamingFake{}
	fake.replies = []llm.Reply{
		{
			ToolCalls:    []llm.ToolCall{{ID: "call_1", Name: "search_products", Arguments: `{"query":"lantern"}`}},
			FinishReason: "tool_calls",
		},
		{Content: "Yes, the Bamboo Lantern is in stock.", FinishReason: "stop"},
	}
	products := &fakeProducts{items: []Product{lantern()}}

	svc := NewChatService(Options{LLM: fake, Products: products, Model: "test-model"})
	emit, out := collectDeltas()
	got, err := svc.AskStream(context.Background(), "k", "", nil, "lantern?", emit)
	if err != nil {
		t.Fatalf("AskStream: %v", err)
	}
	if len(products.queries) != 1 {
		t.Fatalf("catalog queries = %v", products.queries)
	}
	if got.ToolCalls != 1 || len(got.Products) != 1 {
		t.Fatalf("answer = %+v", got)
	}
	if out.String() != "Yes, the Bamboo Lantern is in stock." {
		t.Fatalf("deltas = %q", out.String())
	}
}

// A provider that cannot stream still answers: the whole text arrives as one
// delta after the turn completes.
func TestAskStreamFallsBackForNonStreamingProvider(t *testing.T) {
	fake := &fakeLLM{replies: []llm.Reply{{Content: "Shipping takes 2-4 days."}}}
	svc := NewChatService(Options{LLM: fake, Model: "test-model"})

	emit, out := collectDeltas()
	got, err := svc.AskStream(context.Background(), "k", "", nil, "shipping?", emit)
	if err != nil {
		t.Fatalf("AskStream: %v", err)
	}
	if out.String() != "Shipping takes 2-4 days." || got.Answer != out.String() {
		t.Fatalf("deltas = %q, answer = %q", out.String(), got.Answer)
	}
}

// An emit failure means the client is gone: abort immediately, do not keep
// calling the provider, and surface the caller's own error.
func TestAskStreamStopsWhenEmitFails(t *testing.T) {
	fake := &streamingFake{chunkSize: 2}
	fake.replies = []llm.Reply{{Content: "0123456789", FinishReason: "stop"}}
	svc := NewChatService(Options{LLM: fake, Model: "test-model"})

	stop := errors.New("client disconnected")
	calls := 0
	_, err := svc.AskStream(context.Background(), "k", "", nil, "hi",
		func(string) error {
			calls++
			return stop
		})
	if !errors.Is(err, stop) {
		t.Fatalf("err = %v, want %v", err, stop)
	}
	if calls != 1 {
		t.Fatalf("emits = %d, want 1", calls)
	}
}

// Validation and quota apply to the streaming path too: no deltas, no provider
// call.
func TestAskStreamValidationAndQuota(t *testing.T) {
	fake := &streamingFake{}
	svc := NewChatService(Options{LLM: fake, Limiter: denyLimiter{}})

	emit, out := collectDeltas()
	_, err := svc.AskStream(context.Background(), "k", "", nil, "  ", emit)
	var appErr *utils.AppError
	if !errors.As(err, &appErr) || appErr.Code != utils.ErrCodeBadRequest {
		t.Fatalf("err = %v, want BAD_REQUEST", err)
	}

	_, err = svc.AskStream(context.Background(), "k", "", nil, "hi", emit)
	if !errors.As(err, &appErr) || appErr.Code != utils.ErrCodeTooManyRequests {
		t.Fatalf("err = %v, want TOO_MANY_REQUESTS", err)
	}
	if out.Len() != 0 {
		t.Fatalf("deltas = %q, want none", out.String())
	}
	if len(fake.calls) != 0 {
		t.Fatalf("llm calls = %d, want 0", len(fake.calls))
	}
}

// A cancelled request is the customer leaving, not an outage: it must not be
// reported as SERVICE_UNAVAILABLE.
func TestAskStreamCancelledContextIsNotAnOutage(t *testing.T) {
	fake := &streamingFake{}
	fake.err = context.Canceled
	svc := NewChatService(Options{LLM: fake, Model: "test-model"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.AskStream(ctx, "k", "", nil, "hi", func(string) error { return nil })
	var appErr *utils.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("err = %v, want *utils.AppError", err)
	}
	if appErr.Code != ErrCodeClientClosed {
		t.Fatalf("code = %s, want %s", appErr.Code, ErrCodeClientClosed)
	}
}
