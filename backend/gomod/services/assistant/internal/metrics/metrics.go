// Package metrics holds the assistant service's Prometheus collectors.
//
// They register on the default registry, so the shared metrics.Handler(nil)
// route serves them with no extra wiring. Names are namespaced `assistant_` to
// match the alert rules in system/prometheus.
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestsTotal counts finished HTTP turns. status is the HTTP class
	// ("ok", "error"); fallback is "1" when the answer handed off to a human.
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "assistant",
		Name:      "requests_total",
		Help:      "Assistant chat requests by outcome.",
	}, []string{"status", "fallback"})

	// LLMDuration observes one provider round trip (not the whole tool loop).
	LLMDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "assistant",
		Name:      "llm_duration_seconds",
		Help:      "Provider round-trip latency.",
		Buckets:   []float64{0.5, 1, 2, 5, 10, 30, 60, 120},
	}, []string{"model"})

	// TokensTotal counts billed tokens; kind is "prompt" | "completion" | "cached".
	TokensTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "assistant",
		Name:      "tokens_total",
		Help:      "Provider tokens billed, by kind.",
	}, []string{"model", "kind"})

	// ToolCalls counts tool executions; ok is "1" when the tool returned data.
	ToolCalls = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "assistant",
		Name:      "tool_calls_total",
		Help:      "Tool executions by name and outcome.",
	}, []string{"tool", "ok"})
)

// ObserveLLM records one provider call and its usage. model is the configured
// model id; it is a label, so keep it stable (never the raw request).
func ObserveLLM(model string, d time.Duration, prompt, completion, cached int) {
	LLMDuration.WithLabelValues(model).Observe(d.Seconds())
	if prompt > 0 {
		TokensTotal.WithLabelValues(model, "prompt").Add(float64(prompt))
	}
	if completion > 0 {
		TokensTotal.WithLabelValues(model, "completion").Add(float64(completion))
	}
	if cached > 0 {
		TokensTotal.WithLabelValues(model, "cached").Add(float64(cached))
	}
}

// ObserveTool records one tool execution.
func ObserveTool(name string, ok bool) {
	label := "0"
	if ok {
		label = "1"
	}
	ToolCalls.WithLabelValues(name, label).Inc()
}

// ObserveRequest records the outcome of one finished chat turn.
func ObserveRequest(err error, fallbackToHuman bool) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	fallback := "0"
	if fallbackToHuman {
		fallback = "1"
	}
	RequestsTotal.WithLabelValues(status, fallback).Inc()
}
