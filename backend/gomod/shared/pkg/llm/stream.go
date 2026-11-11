package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ErrStreamUnsupported lets the caller fall back to Complete() for providers
// that answer a stream:true request with a plain JSON body.
var ErrStreamUnsupported = errors.New("llm: provider does not stream")

// Chunk is one incremental piece of a streamed reply. Either Delta carries
// plain text, or the final chunk carries the accumulated ToolCalls. Usage is
// only set on the last chunk (OpenAI sends it there when
// stream_options.include_usage is set; other providers may omit it entirely).
type Chunk struct {
	Delta        string
	ToolCalls    []ToolCall
	FinishReason string
	Usage        *Usage
}

type wireStreamRequest struct {
	wireRequest
	StreamOptions *struct {
		IncludeUsage bool `json:"include_usage"`
	} `json:"stream_options,omitempty"`
}

type wireStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	RawUsage struct {
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

// CompleteStream sends one round trip with stream:true and calls emit for every
// text delta. It returns the assembled Reply (content + tool calls + usage) once
// the provider sends [DONE] or closes the body.
//
// Retries are deliberately NOT applied here: once the caller has flushed a
// single byte, replaying the request would duplicate tokens on screen. A
// pre-stream failure (429/503 on the first response) is still retried, because
// nothing has been emitted yet.
func (c *OpenAIClient) CompleteStream(
	ctx context.Context,
	msgs []Message,
	tools []Tool,
	emit func(Chunk) error,
) (Reply, error) {
	body, err := c.buildStreamBody(msgs, tools)
	if err != nil {
		return Reply{}, fmt.Errorf("llm: encode stream request: %w", err)
	}

	const maxRetries = 2
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			if err := sleepBackoff(ctx, attempt, lastErr); err != nil {
				return Reply{}, err
			}
		}
		reply, emitted, err := c.streamOnce(ctx, body, emit)
		if err == nil {
			return reply, nil
		}
		lastErr = err
		if emitted || !IsRetryable(err) {
			return Reply{}, err
		}
	}
	return Reply{}, lastErr
}

// IsRetryable reports whether err came from a transient provider failure.
func IsRetryable(err error) bool {
	var ra *retryAfterError
	if errors.As(err, &ra) {
		return true
	}
	return errors.Is(err, errStreamTransient)
}

var errStreamTransient = errors.New("llm: transient")

func (c *OpenAIClient) buildStreamBody(msgs []Message, tools []Tool) ([]byte, error) {
	base := c.buildRequest(msgs, tools)
	base.Stream = true
	return json.Marshal(wireStreamRequest{
		wireRequest: base,
		StreamOptions: &struct {
			IncludeUsage bool `json:"include_usage"`
		}{IncludeUsage: true},
	})
}

// streamOnce performs the HTTP exchange and parses SSE frames.
// emitted reports whether any text reached the caller (retry is unsafe then).
func (c *OpenAIClient) streamOnce(ctx context.Context, body []byte, emit func(Chunk) error) (Reply, bool, error) {
	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Reply{}, false, fmt.Errorf("llm: build stream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return Reply{}, false, fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		return Reply{}, false, fmt.Errorf("%w: %v", errStreamTransient, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		detail := strings.TrimSpace(string(raw))
		if len(detail) > 300 {
			detail = detail[:300]
		}
		retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		wait, _ := strconv.Atoi(resp.Header.Get("Retry-After"))
		base := fmt.Errorf("%w: status %d: %s", ErrUnavailable, resp.StatusCode, detail)
		if retryable && wait > 0 {
			return Reply{}, false, &retryAfterError{err: base, wait: time.Duration(wait) * time.Second}
		}
		if retryable {
			return Reply{}, false, fmt.Errorf("%w: %v", errStreamTransient, base)
		}
		return Reply{}, false, base
	}

	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		// Provider ignored stream:true and returned one JSON body.
		return Reply{}, false, ErrStreamUnsupported
	}

	return readSSE(resp.Body, emit)
}

// readSSE parses the SSE framing:
//
//	data: {json}
//	data: {json}
//	data: [DONE]
//
// Comment lines (:keep-alive) and blank separators are ignored.
func readSSE(r io.Reader, emit func(Chunk) error) (Reply, bool, error) {
	if emit == nil {
		emit = func(Chunk) error { return nil }
	}
	var (
		reply   Reply
		emitted bool
		// Tool-call arguments arrive as fragments keyed by index.
		toolBuf   = map[int]*ToolCall{}
		toolOrder []int
		usage     *Usage
	)

	scanner := bufio.NewScanner(r)
	// Tool arguments can be large; the default 64KB line cap is too small.
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}

		var chunk wireStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			// A single malformed frame must not kill the whole answer.
			continue
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			return Reply{}, emitted, fmt.Errorf("%w: %s", ErrUnavailable, chunk.Error.Message)
		}

		if u := chunk.RawUsage; u.PromptTokens > 0 || u.CompletionTokens > 0 {
			usage = &Usage{
				PromptTokens:       u.PromptTokens,
				CompletionTokens:   u.CompletionTokens,
				CachedPromptTokens: u.PromptTokensDetails.CachedTokens,
			}
		}

		for _, choice := range chunk.Choices {
			if d := choice.Delta.Content; d != "" {
				reply.Content += d
				emitted = true
				if err := emit(Chunk{Delta: d}); err != nil {
					return Reply{}, emitted, err
				}
			}
			for _, tc := range choice.Delta.ToolCalls {
				acc, ok := toolBuf[tc.Index]
				if !ok {
					acc = &ToolCall{}
					toolBuf[tc.Index] = acc
					toolOrder = append(toolOrder, tc.Index)
				}
				if tc.ID != "" {
					acc.ID = tc.ID
				}
				if tc.Function.Name != "" {
					acc.Name = tc.Function.Name
				}
				acc.Arguments += tc.Function.Arguments
			}
			if choice.FinishReason != "" {
				reply.FinishReason = choice.FinishReason
			}
		}
	}

	if err := scanner.Err(); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return Reply{}, emitted, err
		}
		return Reply{}, emitted, fmt.Errorf("%w: read stream: %v", ErrUnavailable, err)
	}

	for _, idx := range toolOrder {
		reply.ToolCalls = append(reply.ToolCalls, *toolBuf[idx])
	}
	if usage != nil {
		reply.Usage = *usage
	}
	if err := emit(Chunk{ToolCalls: reply.ToolCalls, FinishReason: reply.FinishReason, Usage: usage}); err != nil {
		return Reply{}, emitted, err
	}
	return reply, emitted, nil
}
