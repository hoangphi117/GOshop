package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ErrUnavailable marks a provider failure the caller should surface as a
// degraded feature (503 + fallback to a human) rather than a 500.
var ErrUnavailable = errors.New("llm provider unavailable")

// OpenAIConfig configures the OpenAI-compatible adapter.
type OpenAIConfig struct {
	BaseURL     string // e.g. https://api.river.ai/v1
	APIKey      string
	Model       string
	MaxTokens   int
	Temperature float64
	Timeout     time.Duration

	// HTTPClient overrides the default client (tests). Timeout still applies
	// through the ctx when set here.
	HTTPClient *http.Client
}

// OpenAIClient talks to POST {BaseURL}/chat/completions.
type OpenAIClient struct {
	cfg  OpenAIConfig
	http *http.Client
}

// NewOpenAIClient builds the adapter. It returns an error when the required
// fields are missing so a misconfiguration fails at wiring time, not on the
// first customer message.
func NewOpenAIClient(cfg OpenAIConfig) (*OpenAIClient, error) {
	if cfg.BaseURL == "" || cfg.Model == "" {
		return nil, errors.New("llm: BaseURL and Model are required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 120 * time.Second
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: cfg.Timeout}
	}
	return &OpenAIClient{cfg: cfg, http: hc}, nil
}

// --- wire types -------------------------------------------------------------

type wireFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type wireTool struct {
	Type     string       `json:"type"`
	Function wireFunction `json:"function"`
}

type wireToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type wireMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolCalls  []wireToolCall `json:"tool_calls,omitempty"`
}

type wireRequest struct {
	Model       string        `json:"model"`
	Messages    []wireMessage `json:"messages"`
	Tools       []wireTool    `json:"tools,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream"`
}

type wireResponse struct {
	Choices []struct {
		Message      wireMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		PromptTokensDetails struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// --- adapter ----------------------------------------------------------------

// Complete sends one round trip. Retries 429 and 503 up to maxRetries times,
// honouring Retry-After when the provider sends it.
func (c *OpenAIClient) Complete(ctx context.Context, msgs []Message, tools []Tool) (Reply, error) {
	body, err := json.Marshal(c.buildRequest(msgs, tools))
	if err != nil {
		return Reply{}, fmt.Errorf("llm: encode request: %w", err)
	}

	const maxRetries = 2
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			if err := sleepBackoff(ctx, attempt, lastErr); err != nil {
				return Reply{}, err
			}
		}

		reply, retryable, err := c.once(ctx, body)
		if err == nil {
			return reply, nil
		}
		lastErr = err
		if !retryable {
			return Reply{}, err
		}
	}
	return Reply{}, lastErr
}

func (c *OpenAIClient) buildRequest(msgs []Message, tools []Tool) wireRequest {
	wire := make([]wireMessage, 0, len(msgs))
	for _, m := range msgs {
		wm := wireMessage{Role: string(m.Role), Content: m.Content, ToolCallID: m.ToolCallID}
		for _, tc := range m.ToolCalls {
			var wc wireToolCall
			wc.ID = tc.ID
			wc.Type = "function"
			wc.Function.Name = tc.Name
			wc.Function.Arguments = tc.Arguments
			wm.ToolCalls = append(wm.ToolCalls, wc)
		}
		wire = append(wire, wm)
	}

	req := wireRequest{
		Model:       c.cfg.Model,
		Messages:    wire,
		MaxTokens:   c.cfg.MaxTokens,
		Temperature: c.cfg.Temperature,
		Stream:      false,
	}
	for _, t := range tools {
		req.Tools = append(req.Tools, wireTool{
			Type:     "function",
			Function: wireFunction(t),
		})
	}
	return req
}

// once performs a single HTTP exchange. retryable reports whether the failure
// is worth another attempt (429/503/timeout/5xx).
func (c *OpenAIClient) once(ctx context.Context, body []byte) (Reply, bool, error) {
	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Reply{}, false, fmt.Errorf("llm: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		// Network error or ctx deadline: retrying a deadline is pointless.
		if ctx.Err() != nil {
			return Reply{}, false, fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		return Reply{}, true, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if readErr != nil {
		return Reply{}, true, fmt.Errorf("%w: read body: %v", ErrUnavailable, readErr)
	}

	if resp.StatusCode != http.StatusOK {
		var parsed wireResponse
		_ = json.Unmarshal(raw, &parsed)
		detail := strings.TrimSpace(string(raw))
		if parsed.Error != nil && parsed.Error.Message != "" {
			detail = parsed.Error.Message
		}
		if len(detail) > 300 {
			detail = detail[:300]
		}
		retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		// Retry-After is carried back through lastErr to the backoff.
		wait, _ := strconv.Atoi(resp.Header.Get("Retry-After"))
		err := fmt.Errorf("%w: status %d: %s", ErrUnavailable, resp.StatusCode, detail)
		if retryable && wait > 0 {
			err = &retryAfterError{err: err, wait: time.Duration(wait) * time.Second}
		}
		return Reply{}, retryable, err
	}

	var parsed wireResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Reply{}, false, fmt.Errorf("llm: decode response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return Reply{}, false, errors.New("llm: empty choices")
	}

	choice := parsed.Choices[0]
	reply := Reply{
		Content:      choice.Message.Content,
		FinishReason: choice.FinishReason,
		Usage: Usage{
			PromptTokens:       parsed.Usage.PromptTokens,
			CompletionTokens:   parsed.Usage.CompletionTokens,
			CachedPromptTokens: parsed.Usage.PromptTokensDetails.CachedTokens,
		},
	}
	for _, tc := range choice.Message.ToolCalls {
		reply.ToolCalls = append(reply.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}
	// Note: reasoning_content (returned for GLM/Kimi on River) is deliberately
	// ignored. Replaying it in history is rejected or wastes budget, and the
	// user-visible answer always lives in Content.
	return reply, false, nil
}

// retryAfterError lets the backoff honour a provider-supplied Retry-After.
type retryAfterError struct {
	err  error
	wait time.Duration
}

func (e *retryAfterError) Error() string { return e.err.Error() }
func (e *retryAfterError) Unwrap() error { return e.err }

var errRetryAfter = errors.New("retry-after")

// Is exposes errRetryAfter so errors.Is(err, errRetryAfter) works.
func (e *retryAfterError) Is(target error) bool { return target == errRetryAfter }

// sleepBackoff waits for Retry-After when present, otherwise exponential
// backoff with jitter: 500ms, 1s (+0-500ms).
func sleepBackoff(ctx context.Context, attempt int, lastErr error) error {
	var wait time.Duration
	var ra *retryAfterError
	if errors.As(lastErr, &ra) {
		wait = ra.wait
	} else {
		wait = time.Duration(500<<uint(attempt-1)) * time.Millisecond
		wait += time.Duration(rand.Intn(500)) * time.Millisecond
	}

	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
