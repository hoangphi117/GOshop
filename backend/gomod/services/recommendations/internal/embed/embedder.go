// Package embed talks to the embedding sidecar.
//
// The model itself lives in a separate container (docker/embedder): a torch
// process needs several hundred MB resident, and every Go service here runs
// under a 160m cap. Keeping the model out of this process also means a model
// crash takes down one shelf, not the whole recommendations API.
package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Dimension is the width of all-MiniLM-L6-v2, and of product_embedding.embedding
// (the migration pins vector(384)). A response of any other width is rejected
// rather than stored, because pgvector would reject it anyway.
const Dimension = 384

// maxTexts mirrors the sidecar's request cap; sending more fails the whole
// batch there, so batch here instead.
const maxTexts = 256

// maxTextRunes is applied client-side too: the sidecar truncates, but sending
// a 200 KB description only to have it truncated wastes the request.
const maxTextRunes = 2000

// Embedder is the piece the recommendation service depends on.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// HTTPEmbedder calls POST /embed on the sidecar.
type HTTPEmbedder struct {
	baseURL string
	client  *http.Client
}

func NewHTTPEmbedder(baseURL string, timeout time.Duration) (*HTTPEmbedder, error) {
	if baseURL == "" {
		return nil, errors.New("embedder URL is empty")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &HTTPEmbedder{
		baseURL: baseURL,
		client:  &http.Client{Timeout: timeout},
	}, nil
}

type embedRequest struct {
	Texts []string `json:"texts"`
}

type embedResponse struct {
	Vectors   [][]float32 `json:"vectors"`
	Model     string      `json:"model"`
	Dimension int         `json:"dimension"`
}

// Embed returns one unit-length vector per text, in the same order.
//
// Embeddings are only useful relative to each other, so a partial batch is an
// error: silently dropping a text would shift every later index.
func (e *HTTPEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if len(texts) > maxTexts {
		return nil, fmt.Errorf("embed: %d texts exceeds the %d batch limit", len(texts), maxTexts)
	}

	trimmed := make([]string, len(texts))
	for i, t := range texts {
		trimmed[i] = truncate(t, maxTextRunes)
	}

	body, err := json.Marshal(embedRequest{Texts: trimmed})
	if err != nil {
		return nil, fmt.Errorf("embed: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embed: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed: call sidecar: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embed: sidecar returned %d", res.StatusCode)
	}

	var payload embedResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("embed: decode response: %w", err)
	}
	if len(payload.Vectors) != len(texts) {
		return nil, fmt.Errorf("embed: expected %d vectors, got %d", len(texts), len(payload.Vectors))
	}
	for i, v := range payload.Vectors {
		if len(v) != Dimension {
			return nil, fmt.Errorf("embed: vector %d has %d dims, expected %d", i, len(v), Dimension)
		}
	}
	return payload.Vectors, nil
}

// truncate counts runes, not bytes: slicing a multi-byte character in half
// produces invalid UTF-8, which the tokenizer mangles into garbage tokens.
func truncate(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes])
}
