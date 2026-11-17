package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// sseServer serves a fixed SSE body with the right content type.
func sseServer(t *testing.T, body string) *OpenAIClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return newTestClient(t, srv.URL)
}

// collect wraps an optional callback and records every chunk.
func collect(inner func(Chunk) error) (func(Chunk) error, *[]Chunk) {
	got := &[]Chunk{}
	return func(c Chunk) error {
		*got = append(*got, c)
		if inner != nil {
			return inner(c)
		}
		return nil
	}, got
}

const textSSE = `: keep-alive

data: {"choices":[{"delta":{"content":"Hel"},"finish_reason":""}]}

data: {"choices":[{"delta":{"content":"lo!"},"finish_reason":"stop"}]}

data: {"choices":[],"usage":{"prompt_tokens":11,"completion_tokens":3,"prompt_tokens_details":{"cached_tokens":7}}}

data: [DONE]

`

func TestCompleteStreamText(t *testing.T) {
	c := sseServer(t, textSSE)
	emit, chunks := collect(nil)
	reply, err := c.CompleteStream(context.Background(),
		[]Message{{Role: RoleUser, Content: "hi"}}, nil, emit)
	if err != nil {
		t.Fatalf("CompleteStream: %v", err)
	}
	if reply.Content != "Hello!" {
		t.Fatalf("Content = %q", reply.Content)
	}
	if reply.FinishReason != "stop" {
		t.Fatalf("FinishReason = %q", reply.FinishReason)
	}
	if reply.Usage.PromptTokens != 11 || reply.Usage.CompletionTokens != 3 || reply.Usage.CachedPromptTokens != 7 {
		t.Fatalf("Usage = %+v", reply.Usage)
	}

	var text strings.Builder
	for _, ch := range *chunks {
		text.WriteString(ch.Delta)
	}
	if text.String() != "Hello!" {
		t.Fatalf("emitted deltas = %q", text.String())
	}
	// Last chunk carries the terminal metadata, not text.
	last := (*chunks)[len(*chunks)-1]
	if last.Delta != "" || last.FinishReason != "stop" || last.Usage == nil {
		t.Fatalf("final chunk = %+v", last)
	}
}

// Tool arguments arrive as fragments keyed by index; fragments for two calls
// may interleave.
const toolSSE = `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_a","function":{"name":"search_products","arguments":"{\"query\":"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":1,"id":"call_b","function":{"name":"search_faqs","arguments":"{\"query\":"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"lantern\"}"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":1,"function":{"arguments":"\"ship\"}"}}]}}]}
data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}
data: [DONE]
`

func TestCompleteStreamToolCallFragments(t *testing.T) {
	c := sseServer(t, toolSSE)
	reply, err := c.CompleteStream(context.Background(),
		[]Message{{Role: RoleUser, Content: "hi"}}, nil, nil)
	if err != nil {
		t.Fatalf("CompleteStream: %v", err)
	}
	if reply.Content != "" {
		t.Fatalf("Content = %q, want empty", reply.Content)
	}
	if len(reply.ToolCalls) != 2 {
		t.Fatalf("ToolCalls = %d, want 2", len(reply.ToolCalls))
	}
	if reply.ToolCalls[0].ID != "call_a" || reply.ToolCalls[0].Name != "search_products" {
		t.Fatalf("ToolCall[0] = %+v", reply.ToolCalls[0])
	}
	if reply.ToolCalls[0].Arguments != `{"query":"lantern"}` {
		t.Fatalf("ToolCall[0].Arguments = %q", reply.ToolCalls[0].Arguments)
	}
	if reply.ToolCalls[1].ID != "call_b" || reply.ToolCalls[1].Arguments != `{"query":"ship"}` {
		t.Fatalf("ToolCall[1] = %+v", reply.ToolCalls[1])
	}
	if reply.FinishReason != "tool_calls" {
		t.Fatalf("FinishReason = %q", reply.FinishReason)
	}
}

// A provider that ignores stream:true answers with one JSON body. The caller
// must be able to fall back to Complete().
func TestCompleteStreamUnsupported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, completionBody)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, err := c.CompleteStream(context.Background(),
		[]Message{{Role: RoleUser, Content: "hi"}}, nil, nil)
	if !errors.Is(err, ErrStreamUnsupported) {
		t.Fatalf("err = %v, want ErrStreamUnsupported", err)
	}
}

// Once a delta reached the caller the request must NOT be replayed: replaying
// duplicates tokens in the transcript and burns quota.
func TestCompleteStreamNoRetryAfterEmit(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/event-stream")
		// Flush a delta, then die mid-stream.
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		panic(http.ErrAbortHandler)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	emit, chunks := collect(nil)
	_, err := c.CompleteStream(context.Background(),
		[]Message{{Role: RoleUser, Content: "hi"}}, nil, emit)
	if err == nil {
		t.Fatal("expected error from aborted stream")
	}
	if len(*chunks) == 0 || (*chunks)[0].Delta != "partial" {
		t.Fatalf("chunks = %+v", *chunks)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 (no replay after emit)", calls)
	}
}

// A 429 before the stream starts is safe to retry.
func TestCompleteStreamRetriesBeforeEmit(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"error":{"message":"rate limited"}}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, textSSE)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	reply, err := c.CompleteStream(context.Background(),
		[]Message{{Role: RoleUser, Content: "hi"}}, nil, nil)
	if err != nil {
		t.Fatalf("CompleteStream: %v", err)
	}
	if reply.Content != "Hello!" {
		t.Fatalf("Content = %q", reply.Content)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

// A malformed frame is skipped, not fatal: one bad line must not kill the
// whole answer.
func TestCompleteStreamSkipsMalformedFrame(t *testing.T) {
	body := "data: {not json\n\n" + textSSE
	c := sseServer(t, body)
	reply, err := c.CompleteStream(context.Background(),
		[]Message{{Role: RoleUser, Content: "hi"}}, nil, nil)
	if err != nil {
		t.Fatalf("CompleteStream: %v", err)
	}
	if reply.Content != "Hello!" {
		t.Fatalf("Content = %q", reply.Content)
	}
}

// The emit callback is how a handler learns the client went away; its error
// must abort the stream immediately and reach the caller unwrapped.
func TestCompleteStreamAbortsOnEmitError(t *testing.T) {
	c := sseServer(t, textSSE)
	stop := fmt.Errorf("client gone")
	calls := 0
	_, err := c.CompleteStream(context.Background(),
		[]Message{{Role: RoleUser, Content: "hi"}}, nil,
		func(Chunk) error {
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
