package metrics_test

import (
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"goshop/services/assistant/internal/metrics"
)

// The collectors are scraped by name from dashboards, so assert the names and
// labels rather than just "a counter incremented".
func TestObserveLLMAndTool(t *testing.T) {
	model := "test-model-emit"
	metrics.ObserveLLM(model, 1500*time.Millisecond, 100, 40, 10)

	if n := testutil.ToFloat64(metrics.TokensTotal.WithLabelValues(model, "prompt")); n != 100 {
		t.Fatalf("prompt tokens = %v, want 100", n)
	}
	if n := testutil.ToFloat64(metrics.TokensTotal.WithLabelValues(model, "completion")); n != 40 {
		t.Fatalf("completion tokens = %v, want 40", n)
	}
	if n := testutil.ToFloat64(metrics.TokensTotal.WithLabelValues(model, "cached")); n != 10 {
		t.Fatalf("cached tokens = %v, want 10", n)
	}

	// Zero usage must not create a token series (avoids a phantom 0 line).
	metrics.ObserveLLM("test-model-zero", time.Second, 0, 0, 0)
	if got := testutil.CollectAndCount(metrics.TokensTotal); got == 0 {
		t.Fatal("no token series collected")
	}

	metrics.ObserveTool("search_products", true)
	metrics.ObserveTool("search_products", false)
	if n := testutil.ToFloat64(metrics.ToolCalls.WithLabelValues("search_products", "1")); n != 1 {
		t.Fatalf("ok tool calls = %v, want 1", n)
	}
	if n := testutil.ToFloat64(metrics.ToolCalls.WithLabelValues("search_products", "0")); n != 1 {
		t.Fatalf("failed tool calls = %v, want 1", n)
	}
}

func TestObserveRequestOutcome(t *testing.T) {
	metrics.ObserveRequest(nil, false)
	if n := testutil.ToFloat64(metrics.RequestsTotal.WithLabelValues("ok", "0")); n != 1 {
		t.Fatalf("ok requests = %v, want 1", n)
	}

	metrics.ObserveRequest(errors.New("boom"), true)
	if n := testutil.ToFloat64(metrics.RequestsTotal.WithLabelValues("error", "1")); n != 1 {
		t.Fatalf("error requests = %v, want 1", n)
	}
}

// Guard the exposed metric names: a rename here breaks the alert rule without
// breaking the build.
func TestMetricNames(t *testing.T) {
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	want := map[string]bool{
		"assistant_requests_total":       false,
		"assistant_llm_duration_seconds": false,
		"assistant_tokens_total":         false,
		"assistant_tool_calls_total":     false,
	}
	for _, f := range families {
		if _, ok := want[f.GetName()]; ok {
			want[f.GetName()] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Fatalf("metric %q not registered", name)
		}
	}
}
