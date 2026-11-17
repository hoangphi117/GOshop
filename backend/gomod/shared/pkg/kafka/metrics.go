package kafka

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Phase 1 observability, Prometheus surface: these counters used to be bare
// expvar maps, which nothing could scrape. The metric NAMES and label
// semantics are unchanged (kafka_publish_ok_total{topic=...} etc.) so any
// existing query keeps working; only the exposition moved from /healthz JSON
// to /metrics. The JSON snapshot is still served by /healthz.
//
// Only the error counters go through a helper: they are incremented from two
// packages (broker.go and retry.go) and would otherwise be easy to update in
// one place and forget in the other.
var (
	publishOK = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kafka_publish_ok_total",
		Help: "Records published to a topic successfully.",
	}, []string{"topic"})

	publishErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kafka_publish_errors_total",
		Help: "Publish attempts that failed, per topic.",
	}, []string{"topic"})

	dlqMoved = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kafka_dlq_total",
		Help: "Records moved to a DLQ topic after exhausting retries.",
	}, []string{"topic"})

	consumeErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kafka_consume_errors_total",
		Help: "Handler failures per consumer group.",
	}, []string{"group"})

	// Unique records published vs. records published without a payload
	// event_id. The ratio is the producer-side signal for "consumers cannot
	// deduplicate this stream": the header still carries a generated id, but a
	// consumer that dedupes on the payload has nothing to key on.
	publishedRecords = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kafka_records_published_total",
		Help: "Records published, per topic. Denominator of kafka_records_missing_event_id_total.",
	}, []string{"topic"})

	publishedRecordsMissingEventID = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kafka_records_missing_event_id_total",
		Help: "Records published without a payload event_id field, per topic.",
	}, []string{"topic"})
)

// MetricsSnapshot is the JSON surface health_handler merges into /healthz.
//
// Kept alongside the Prometheus vectors: /healthz predates /metrics and its
// shape is a published contract.
type MetricsSnapshot struct {
	PublishOK     map[string]int64 `json:"publish_ok"`
	PublishErrors map[string]int64 `json:"publish_errors"`
	DLQMoved      map[string]int64 `json:"dlq_moved"`
	ConsumeErrors map[string]int64 `json:"consume_errors"`
}

// Metrics returns a copy of current counters, safe for JSON encoding.
func Metrics() MetricsSnapshot {
	return MetricsSnapshot{
		PublishOK:     collectVec(publishOK),
		PublishErrors: collectVec(publishErrors),
		DLQMoved:      collectVec(dlqMoved),
		ConsumeErrors: collectVec(consumeErrors),
	}
}
