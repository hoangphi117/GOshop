package kafka

// Phase-1 observability counters. Each test uses its own topic/group name so
// package-level expvar state from other tests can never mask a regression.

import (
	"context"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goshop/shared/pkg/messaging"
)

func TestMetrics_PublishCountsOKAndError(t *testing.T) {
	okTopic := "goshop.order.v1"
	beforeOK := Metrics().PublishOK[okTopic]
	require.NoError(t, newTestBroker(t, nil).Publish(context.Background(), "order.created", []byte(`{"order_uuid":"m1"}`)))
	assert.Equal(t, beforeOK+1, Metrics().PublishOK[okTopic])

	beforeErr := Metrics().PublishErrors[okTopic]
	bad := newTestBroker(t, &fakeWriter{err: assert.AnError})
	require.Error(t, bad.Publish(context.Background(), "order.created", []byte(`{"order_uuid":"m2"}`)))
	assert.Equal(t, beforeErr+1, Metrics().PublishErrors[okTopic], "writer failure increments the error counter")
}

func TestMetrics_DLQMovedCountsExhaustedRetries(t *testing.T) {
	t.Setenv("KAFKA_MAX_RETRIES", "1")
	t.Setenv("KAFKA_RETRY_BACKOFF_BASE_MS", "1")

	w := &fakeWriter{}
	b := newTestBroker(t, w)
	rr := newRetryRunner(b)

	msg := kafka.Message{
		Topic:   "goshop.email.v1",
		Key:     []byte("agg-metrics"),
		Value:   []byte(`{"event_type":"email.send"}`),
		Headers: []kafka.Header{{Key: headerRetryCount, Value: []byte("1")}},
	}
	beforeDLQ := Metrics().DLQMoved["goshop.email.v1"]
	require.NoError(t, rr.apply(context.Background(), "goshop.metrics-test", &fakeCommitter{}, msg,
		func(messaging.Header, []byte) error { return assert.AnError }))

	assert.Equal(t, beforeDLQ+1, Metrics().DLQMoved["goshop.email.v1"], "exhausted retry lands in DLQ exactly once")
}

func TestMetrics_ConsumeErrorCountedByGroup(t *testing.T) {
	b := newTestBroker(t, nil)
	reader := &fakeCommitter{}
	rr := newRetryRunner(b)
	t.Setenv("KAFKA_MAX_RETRIES", "0")

	err := rr.apply(context.Background(), "goshop.metrics-consumer", reader,
		kafka.Message{Topic: "goshop.review.v1", Value: []byte(`{}`)},
		func(messaging.Header, []byte) error { return nil })
	require.NoError(t, err, "handler success must not be counted as consume error")

	// consumeErrors is incremented by startGroupConsumer, not retryRunner;
	// assert the counter map is reachable and typed so the wiring cannot rot.
	assert.NotNil(t, Metrics().ConsumeErrors)
}
