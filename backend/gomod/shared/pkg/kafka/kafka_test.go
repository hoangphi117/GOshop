package kafka

// Producer-side and lifecycle tests. Retry-consumer semantics live in
// retry_test.go; this file covers everything the producer touches plus the
// full produce→consume header round-trip against fake writer/committer.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goshop/shared/pkg/messaging"
	"goshop/shared/pkg/utils"
)

// ---- producer path --------------------------------------------------------

func newTestBroker(t *testing.T, w writerLike) *kafkaBroker {
	t.Helper()
	if w == nil {
		w = &fakeWriter{}
	}
	return &kafkaBroker{brokers: []string{"127.0.0.1:1"}, logger: newNopLogger(t), writer: w, quit: make(chan struct{})}
}

func TestPublish_RoutesToDomainTopicWithKey(t *testing.T) {
	w := &fakeWriter{}
	b := newTestBroker(t, w)

	body := `{"order_uuid":"ord-9","event_id":"evt-9","event_type":"order.created","trace_id":"tr-9"}`
	require.NoError(t, b.Publish(context.Background(), "order.created", []byte(body)))

	sent := w.messages()
	require.Len(t, sent, 1)
	assert.Equal(t, "goshop.order.v1", sent[0].Topic)
	assert.Equal(t, "ord-9", string(sent[0].Key), "partition key = aggregate, ordering per order")
	assert.Equal(t, "evt-9", headerValue(sent[0].Headers, headerEventID))
	assert.Equal(t, "order.created", headerValue(sent[0].Headers, headerEventType))
	assert.Equal(t, "tr-9", headerValue(sent[0].Headers, headerTraceID))
}

func TestPublish_AcceptsStructPayload(t *testing.T) {
	w := &fakeWriter{}
	b := newTestBroker(t, w)

	type orderPayload struct {
		OrderUUID string `json:"order_uuid"`
		Total     int    `json:"total"`
	}
	require.NoError(t, b.Publish(context.Background(), "order.created", orderPayload{OrderUUID: "ord-1", Total: 5}))

	sent := w.messages()
	require.Len(t, sent, 1)
	assert.JSONEq(t, `{"order_uuid":"ord-1","total":5}`, string(sent[0].Value))
	assert.Equal(t, "ord-1", string(sent[0].Key))
}

func TestPublish_RejectsEmptyName(t *testing.T) {
	b := newTestBroker(t, nil)
	assert.ErrorIs(t, b.Publish(context.Background(), "  ", []byte(`{}`)), errNameEmpty)
	assert.ErrorIs(t, b.Consume(context.Background(), "", func(messaging.Header, []byte) error { return nil }), errNameEmpty)
}

func TestPublish_RejectsUnmarshallablePayload(t *testing.T) {
	b := newTestBroker(t, nil)
	assert.Error(t, b.Publish(context.Background(), "order.created", make(chan int)),
		"channels cannot be JSON-encoded, must fail before touching the wire")
}

func TestPublishTopic_MapsRoutingKey(t *testing.T) {
	w := &fakeWriter{}
	b := newTestBroker(t, w)

	require.NoError(t, b.PublishTopic(context.Background(), "payment.succeeded", []byte(`{"order_uuid":"ord-3"}`)))
	sent := w.messages()
	require.Len(t, sent, 1)
	assert.Equal(t, "goshop.payment.v1", sent[0].Topic)
	assert.Equal(t, "payment.succeeded", headerValue(sent[0].Headers, headerEventType))
}

func TestPublishToExchange_IgnoresExchangeName(t *testing.T) {
	w := &fakeWriter{}
	b := newTestBroker(t, w)

	require.NoError(t, b.PublishToExchange(context.Background(), "goshop.events", "user.created", []byte(`{"user_uuid":"u-1"}`)))
	sent := w.messages()
	require.Len(t, sent, 1)
	assert.Equal(t, "goshop.user.v1", sent[0].Topic)
}

func TestPublish_PropagatesWriterError(t *testing.T) {
	w := &fakeWriter{err: assert.AnError}
	b := newTestBroker(t, w)
	assert.Error(t, b.Publish(context.Background(), "order.created", []byte(`{}`)))
}

func TestWriter1_SharesOneInstance(t *testing.T) {
	b := newTestBroker(t, nil)
	b.writer = nil // force lazy build
	assert.Same(t, b.writer1(), b.writer1(), "single Writer across all topics, like the old shared channel")
	require.NoError(t, b.Close())
}

// ---- full round-trip: producer headers → consumer handler ------------------

func TestRoundTrip_ProducerHeadersReachConsumerHandler(t *testing.T) {
	t.Setenv("KAFKA_MAX_RETRIES", "3")
	w := &fakeWriter{}
	b := newTestBroker(t, w)

	body := `{"event_id":"evt-rt","event_type":"order.status_changed","trace_id":"tr-rt","aggregate_id":"ord-rt","order_uuid":"ord-rt"}`
	require.NoError(t, b.Publish(context.Background(), "order.status_changed", []byte(body)))

	sent := w.messages()
	require.Len(t, sent, 1)

	// Consumer side: what toNeutralHeaders + filter + handler see.
	got := make(messaging.Header)
	rr := newRetryRunner(b)
	c := &fakeCommitter{}
	err := rr.apply(context.Background(), "g", c, sent[0], func(h messaging.Header, _ []byte) error {
		got = h
		return nil
	})
	require.NoError(t, err)

	assert.Equal(t, "evt-rt", got.Get(headerEventID))
	assert.Equal(t, "order.status_changed", got.Get(headerEventType))
	assert.Equal(t, "tr-rt", got.Get(headerTraceID))
	assert.Equal(t, "ord-rt", got.Get(headerAggregateID))
	assert.Len(t, c.committed, 1)
}

func TestSubscribeToExchange_RejectsEmptyPatterns(t *testing.T) {
	b := newTestBroker(t, nil)
	err := b.SubscribeToExchange(context.Background(), "", "some-queue", []string{"   "}, func(messaging.Header, []byte) error { return nil })
	require.Error(t, err, "blank patterns resolve to nothing, must fail fast")
}

// ---- lifecycle --------------------------------------------------------------

func TestNewKafkaBroker_OK(t *testing.T) {
	mb, err := NewKafkaBroker("kafka:29092, localhost:9092", newNopLogger(t))
	require.NoError(t, err)
	require.NotNil(t, mb)

	kb, ok := mb.(*kafkaBroker)
	require.True(t, ok)
	assert.Equal(t, []string{"kafka:29092", "localhost:9092"}, kb.brokers, "spaces trimmed, order kept")

	require.NoError(t, mb.Close(), "close with unbuilt writer")
	require.NoError(t, mb.Close(), "close is idempotent")

	assert.NotPanics(t, func() { _ = utils.GetEnv("KAFKA_BROKERS", "") })
}

func TestConsume_CancelledContextExits(t *testing.T) {
	b := newTestBroker(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, b.Consume(ctx, "some.queue", func(messaging.Header, []byte) error { return nil }),
		"registration must not dial; loops exit on cancelled context")
}
