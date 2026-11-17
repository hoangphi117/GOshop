package kafka

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The metrics moved from expvar to Prometheus. These checks pin the two things
// that would break silently: the exposition format Prometheus scrapes, and the
// /healthz JSON snapshot that predates it.
func TestMetrics_PrometheusExpositionCarriesCounters(t *testing.T) {
	const topic = "goshop.metrics-exposition.v1"
	publishOK.WithLabelValues(topic).Inc()
	publishOK.WithLabelValues(topic).Inc()

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	var found bool
	for _, f := range families {
		if f.GetName() != "kafka_publish_ok_total" {
			continue
		}
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "topic" && l.GetValue() == topic {
					found = true
					assert.Equal(t, float64(2), m.GetCounter().GetValue())
				}
			}
		}
	}
	assert.True(t, found, "kafka_publish_ok_total must expose a topic label")
}

func TestMetrics_SnapshotStillReadsTheVectors(t *testing.T) {
	topic := "goshop.metrics-snapshot.v1"
	before := Metrics().PublishOK[topic]

	publishErrors.WithLabelValues(topic).Inc()

	assert.Equal(t, before, Metrics().PublishOK[topic], "unrelated topic untouched")
	assert.Equal(t, int64(1), Metrics().PublishErrors[topic], "error vector readable through the JSON snapshot")
}

// The missing-event_id ratio is the only producer-side signal that a consumer
// cannot deduplicate a stream (P7c class of bug). Both counters must move
// together on the same publish, and the denominator must count every record.
func TestMetrics_PublishCountsMissingEventID(t *testing.T) {
	topic := "goshop.order.v1"
	beforeRecords := counterValue(t, "kafka_records_published_total", topic)
	beforeMissing := counterValue(t, "kafka_records_missing_event_id_total", topic)

	b := newTestBroker(t, nil)
	require.NoError(t, b.Publish(context.Background(), "order.created", []byte(`{"order_uuid":"o-1","event_id":"has-one"}`)))
	require.NoError(t, b.Publish(context.Background(), "order.created", []byte(`{"order_uuid":"o-2"}`)))

	assert.Equal(t, beforeRecords+2, counterValue(t, "kafka_records_published_total", topic))
	assert.Equal(t, beforeMissing+1, counterValue(t, "kafka_records_missing_event_id_total", topic))
}

func counterValue(t *testing.T, name, topic string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "topic" && l.GetValue() == topic {
					return m.GetCounter().GetValue()
				}
			}
		}
	}
	return 0
}
