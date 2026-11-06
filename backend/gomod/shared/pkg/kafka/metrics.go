package kafka

import "expvar"

// Phase 1 observability: stdlib expvar counters, no new dependency.
// kafka-exporter (compose) scrapes broker JMX; these counters track what the
// adapter itself sees: publish outcomes and DLQ moves per topic/group.
var (
	publishOK     = expvar.NewMap("kafka_publish_ok_total")
	publishErrors = expvar.NewMap("kafka_publish_errors_total")
	dlqMoved      = expvar.NewMap("kafka_dlq_total")
	consumeErrors = expvar.NewMap("kafka_consume_errors_total")
)

// MetricsSnapshot is the JSON surface health_handler merges into /healthz.
type MetricsSnapshot struct {
	PublishOK     map[string]int64 `json:"publish_ok"`
	PublishErrors map[string]int64 `json:"publish_errors"`
	DLQMoved      map[string]int64 `json:"dlq_moved"`
	ConsumeErrors map[string]int64 `json:"consume_errors"`
}

// Metrics returns a copy of current counters, safe for JSON encoding.
func Metrics() MetricsSnapshot {
	return MetricsSnapshot{
		PublishOK:     mapIntVals(publishOK),
		PublishErrors: mapIntVals(publishErrors),
		DLQMoved:      mapIntVals(dlqMoved),
		ConsumeErrors: mapIntVals(consumeErrors),
	}
}

func mapIntVals(m *expvar.Map) map[string]int64 {
	out := map[string]int64{}
	m.Do(func(kv expvar.KeyValue) {
		if v, ok := kv.Value.(*expvar.Int); ok {
			out[kv.Key] = v.Value()
		}
	})
	return out
}
