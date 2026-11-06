package kafka

import (
	"errors"
	"strings"
	"testing"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStampDLQ_AttachesTriageHeaders(t *testing.T) {
	msg := kafkago.Message{
		Topic:   "goshop.order.v1",
		Key:     []byte("order-1"),
		Value:   []byte(`{"event_id":"e1"}`),
		Offset:  42,
		Headers: []kafkago.Header{{Key: headerEventType, Value: []byte("order.created")}},
	}

	out := stampDLQ(msg, "goshop.analytics.order_events", errors.New("boom"))

	assert.Equal(t, "goshop.order.v1.dlq", out.Topic, "lands on the DLQ sibling")
	assert.Equal(t, msg.Key, out.Key, "partition key preserved for ordering")
	assert.Equal(t, msg.Value, out.Value, "payload preserved byte-for-byte for replay")
	assert.Equal(t, "goshop.order.v1", headerValue(out.Headers, headerDLQTopic))
	assert.Equal(t, "goshop.analytics.order_events", headerValue(out.Headers, headerDLQGroup))
	assert.Equal(t, "42", headerValue(out.Headers, headerDLQOffset))
	assert.Equal(t, "boom", headerValue(out.Headers, headerDLQReason))
	assert.Equal(t, "broker", headerValue(out.Headers, headerDLQSource))
	assert.Equal(t, "order.created", headerValue(out.Headers, headerEventType), "original headers survive")
}

func TestStampDLQ_ReasonIsTruncated(t *testing.T) {
	long := strings.Repeat("x", dlqReasonMaxLen*2)
	out := stampDLQ(kafkago.Message{Topic: "goshop.order.v1"}, "g", errors.New(long))
	assert.Len(t, headerValue(out.Headers, headerDLQReason), dlqReasonMaxLen)
}

func TestStampDLQ_NilCauseStillStamped(t *testing.T) {
	out := stampDLQ(kafkago.Message{Topic: "goshop.order.v1"}, "g", nil)
	assert.Equal(t, "unknown", headerValue(out.Headers, headerDLQReason))
}

func TestBuildReplayMessage_ResetsRetryAndDropsDLQHeaders(t *testing.T) {
	msg := kafkago.Message{
		Topic: "goshop.order.v1.dlq",
		Key:   []byte("k"),
		Value: []byte("v"),
		Headers: []kafkago.Header{
			{Key: headerEventType, Value: []byte("order.created")},
			{Key: headerRetryCount, Value: []byte("5")},
			{Key: headerDLQTopic, Value: []byte("goshop.order.v1")},
			{Key: headerDLQGroup, Value: []byte("g")},
			{Key: headerDLQOffset, Value: []byte("42")},
			{Key: headerDLQReason, Value: []byte("boom")},
			{Key: headerDLQTime, Value: []byte("2026-01-01T00:00:00Z")},
			{Key: headerDLQSource, Value: []byte("broker")},
		},
	}

	out := buildReplayMessage(msg, "goshop.order.v1")

	assert.Equal(t, "goshop.order.v1", out.Topic)
	assert.Equal(t, "0", headerValue(out.Headers, headerRetryCount), "fresh retry budget")
	assert.Equal(t, "order.created", headerValue(out.Headers, headerEventType))
	for _, dropped := range []string{headerDLQTopic, headerDLQGroup, headerDLQOffset, headerDLQReason, headerDLQTime, headerDLQSource} {
		assert.Empty(t, headerValue(out.Headers, dropped), "replay must not leak %s", dropped)
	}
}

func TestKnownDLQTopics_CoversEveryDomainTopicOnce(t *testing.T) {
	topics := KnownDLQTopics()
	require.NotEmpty(t, topics)

	seen := map[string]bool{}
	for _, tp := range topics {
		assert.True(t, strings.HasSuffix(tp, dlqSuffix), "%s must end in %s", tp, dlqSuffix)
		assert.False(t, seen[tp], "duplicate DLQ topic %s", tp)
		seen[tp] = true
	}

	// Every domain topic used by a subscriber must have a DLQ sibling.
	for _, spec := range domainTopics {
		assert.True(t, seen[DLQName(spec.topic)], "missing DLQ for %s", spec.topic)
	}
}
