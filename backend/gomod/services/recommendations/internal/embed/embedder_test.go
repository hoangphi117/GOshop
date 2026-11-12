package embed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func vector(dims int, fill float32) []float32 {
	v := make([]float32, dims)
	for i := range v {
		v[i] = fill
	}
	return v
}

func TestEmbedReturnsVectorsInOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/embed" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var req embedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
		}
		if len(req.Texts) != 2 {
			t.Errorf("got %d texts, want 2", len(req.Texts))
		}
		_ = json.NewEncoder(w).Encode(embedResponse{
			Vectors:   [][]float32{vector(Dimension, 1), vector(Dimension, 2)},
			Model:     "all-MiniLM-L6-v2",
			Dimension: Dimension,
		})
	}))
	defer srv.Close()

	e, err := NewHTTPEmbedder(srv.URL, 5*time.Second)
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	got, err := e.Embed(context.Background(), []string{"laptop", "mouse"})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if len(got) != 2 || got[0][0] != 1 || got[1][0] != 2 {
		t.Fatalf("vectors out of order: %v", got)
	}
}

// A short vector means the sidecar served a different model; storing it would
// make every later distance meaningless.
func TestEmbedRejectsWrongDimension(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(embedResponse{
			Vectors: [][]float32{{1, 2, 3}},
		})
	}))
	defer srv.Close()

	e, _ := NewHTTPEmbedder(srv.URL, 5*time.Second)
	if _, err := e.Embed(context.Background(), []string{"x"}); err == nil {
		t.Fatal("expected an error for a 3-dim vector")
	}
}

func TestEmbedRejectsMissingVectors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(embedResponse{Vectors: [][]float32{vector(Dimension, 1)}})
	}))
	defer srv.Close()

	e, _ := NewHTTPEmbedder(srv.URL, 5*time.Second)
	if _, err := e.Embed(context.Background(), []string{"a", "b"}); err == nil {
		t.Fatal("expected an error when one vector is missing")
	}
}

func TestEmbedEnforcesBatchLimit(t *testing.T) {
	e, _ := NewHTTPEmbedder("http://127.0.0.1:1", time.Second)
	if _, err := e.Embed(context.Background(), make([]string, maxTexts+1)); err == nil {
		t.Fatal("expected an error for an oversized batch")
	}
}

func TestTruncateCountsRunes(t *testing.T) {
	// Each "ê" is two bytes; cutting by bytes would split it.
	got := truncate(strings.Repeat("ê", 10), 3)
	if len([]rune(got)) != 3 || !strings.ContainsRune(got, 'ê') {
		t.Fatalf("truncate mangled the string: %q", got)
	}
	if out := truncate("short", 100); out != "short" {
		t.Fatalf("truncate changed a short string: %q", out)
	}
}
