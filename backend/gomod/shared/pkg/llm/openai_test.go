package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(t *testing.T, url string) *OpenAIClient {
	t.Helper()
	c, err := NewOpenAIClient(OpenAIConfig{
		BaseURL:   url,
		APIKey:    "rv_test",
		Model:     "deepseek-ai/DeepSeek-V4.1-Flash-1017",
		MaxTokens: 64,
		HTTPClient: &http.Client{
			// Short timeout so the retry test does not wait real seconds.
			Timeout: 2 * time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewOpenAIClient: %v", err)
	}
	return c
}

const completionBody = `{
  "choices":[{"message":{"role":"assistant","content":"Hello!"},"finish_reason":"stop"}],
  "usage":{"prompt_tokens":11,"completion_tokens":3,"prompt_tokens_details":{"cached_tokens":7}}
}`

const toolCallBody = `{
  "choices":[{"message":{"role":"assistant","content":"","tool_calls":[
    {"id":"call_1","type":"function","function":{"name":"search_products","arguments":"{\"query\":\"lantern\"}"}}
  ]},"finish_reason":"tool_calls"}],
  "usage":{"prompt_tokens":20,"completion_tokens":9}
}`

func TestComplete(t *testing.T) {
	cases := map[string]struct {
		body   string
		assert func(t *testing.T, r Reply)
		tools  []Tool
		method string
	}{
		"plain completion": {
			body: completionBody,
			assert: func(t *testing.T, r Reply) {
				if r.Content != "Hello!" {
					t.Fatalf("Content = %q", r.Content)
				}
				if r.FinishReason != "stop" {
					t.Fatalf("FinishReason = %q", r.FinishReason)
				}
				if r.Usage.PromptTokens != 11 || r.Usage.CompletionTokens != 3 || r.Usage.CachedPromptTokens != 7 {
					t.Fatalf("Usage = %+v", r.Usage)
				}
			},
		},
		"tool call": {
			body: toolCallBody,
			tools: []Tool{{
				Name:        "search_products",
				Description: "Find products by free text.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`),
			}},
			assert: func(t *testing.T, r Reply) {
				if len(r.ToolCalls) != 1 {
					t.Fatalf("ToolCalls = %d, want 1", len(r.ToolCalls))
				}
				tc := r.ToolCalls[0]
				if tc.ID != "call_1" || tc.Name != "search_products" {
					t.Fatalf("ToolCall = %+v", tc)
				}
				if tc.Arguments != `{"query":"lantern"}` {
					t.Fatalf("Arguments = %q", tc.Arguments)
				}
				if r.FinishReason != "tool_calls" {
					t.Fatalf("FinishReason = %q", r.FinishReason)
				}
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var gotAuth, gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get("Authorization")
				gotPath = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			c := newTestClient(t, srv.URL)
			reply, err := c.Complete(context.Background(),
				[]Message{{Role: RoleUser, Content: "hi"}}, tc.tools)
			if err != nil {
				t.Fatalf("Complete: %v", err)
			}
			if gotAuth != "Bearer rv_test" {
				t.Fatalf("Authorization = %q", gotAuth)
			}
			if gotPath != "/chat/completions" {
				t.Fatalf("path = %q", gotPath)
			}
			tc.assert(t, reply)
		})
	}
}

// River returns 429/503 with Retry-After; the adapter must retry exactly once
// more and succeed on the second call.
func TestCompleteRetriesOn429(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
			return
		}
		_, _ = w.Write([]byte(completionBody))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	reply, err := c.Complete(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if reply.Content != "Hello!" {
		t.Fatalf("Content = %q", reply.Content)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("calls = %d, want exactly 2", got)
	}
}

// A 400 is the caller's fault; retrying it only burns quota.
func TestCompleteDoesNotRetryOn400(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad tools"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, err := c.Complete(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("calls = %d, want 1", got)
	}
}

// Exhausting retries must still return ErrUnavailable, not a raw HTTP error.
func TestCompleteGivesUpAfterRetries(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, err := c.Complete(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("calls = %d, want 3 (1 + 2 retries)", got)
	}
}

func TestNewOpenAIClientRequiresConfig(t *testing.T) {
	if _, err := NewOpenAIClient(OpenAIConfig{BaseURL: "", Model: "m"}); err == nil {
		t.Fatal("expected error for empty BaseURL")
	}
	if _, err := NewOpenAIClient(OpenAIConfig{BaseURL: "http://x", Model: ""}); err == nil {
		t.Fatal("expected error for empty Model")
	}
}
