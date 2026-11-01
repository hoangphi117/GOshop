package kafka

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goshop/shared/pkg/messaging"
)

// ---- test doubles -----------------------------------------------------------

type fakeWriter struct {
	mu   sync.Mutex
	sent []kafka.Message
	err  error
}

func (f *fakeWriter) WriteMessages(_ context.Context, msgs ...kafka.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, msgs...)
	return nil
}

func (f *fakeWriter) Close() error { return nil }

func (f *fakeWriter) messages() []kafka.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]kafka.Message(nil), f.sent...)
}

type fakeCommitter struct {
	committed []kafka.Message
}

func (f *fakeCommitter) CommitMessages(_ context.Context, msgs ...kafka.Message) error {
	f.committed = append(f.committed, msgs...)
	return nil
}

func newTestRunner(t *testing.T) (*retryRunner, *fakeWriter) {
	t.Helper()
	t.Setenv("KAFKA_RETRY_BACKOFF_BASE_MS", "1")
	t.Setenv("KAFKA_MAX_RETRIES", "2")

	w := &fakeWriter{}
	b := &kafkaBroker{brokers: []string{"127.0.0.1:1"}, logger: newNopLogger(t), writer: w, quit: make(chan struct{})}
	return newRetryRunner(b), w
}

// newNopLogger returns a discard logger wired for tests.
func newNopLogger(t *testing.T) *zerolog.Logger {
	t.Helper()
	_ = t
	nop := zerolog.Nop()
	return &nop
}

// ---- retry behaviour --------------------------------------------------------

func TestApply_SuccessCommits(t *testing.T) {
	rr, w := newTestRunner(t)
	c := &fakeCommitter{}
	msg := kafka.Message{Topic: "goshop.order.v1", Value: []byte(`{"event_type":"order.created"}`)}

	err := rr.apply(context.Background(), "g", c, msg, func(messaging.Header, []byte) error { return nil })
	require.NoError(t, err)
	assert.Len(t, c.committed, 1, "success must commit exactly once")
	assert.Empty(t, w.messages(), "success must not republish")
}

func TestApply_FailureRepublishesWithIncrementedRetryCount(t *testing.T) {
	rr, w := newTestRunner(t)
	c := &fakeCommitter{}
	msg := kafka.Message{
		Topic:   "goshop.order.v1",
		Value:   []byte(`{}`),
		Headers: []kafka.Header{{Key: headerRetryCount, Value: []byte("0")}},
	}

	err := rr.apply(context.Background(), "g", c, msg, func(messaging.Header, []byte) error {
		return errors.New("boom")
	})
	require.NoError(t, err, "retry path is not an infrastructure failure")

	sent := w.messages()
	require.Len(t, sent, 1)
	assert.Equal(t, "goshop.order.v1", sent[0].Topic)
	assert.Equal(t, "1", headerValue(sent[0].Headers, headerRetryCount), "retry count must increment")
	assert.Len(t, c.committed, 1, "original is committed only after the retry landed")
}

func TestApply_ExhaustedRetriesGoToDLQ(t *testing.T) {
	rr, w := newTestRunner(t)
	c := &fakeCommitter{}
	msg := kafka.Message{
		Topic: "goshop.payment.v1",
		Value: []byte(`{}`),
		// already at max (2) so the next failure goes straight to DLQ
		Headers: []kafka.Header{{Key: headerRetryCount, Value: []byte("2")}, {Key: headerMaxRetries, Value: []byte("2")}},
	}

	err := rr.apply(context.Background(), "g", c, msg, func(messaging.Header, []byte) error {
		return errors.New("permanent")
	})
	require.NoError(t, err)

	sent := w.messages()
	require.Len(t, sent, 1)
	assert.Equal(t, "goshop.payment.v1.dlq", sent[0].Topic)
	assert.Equal(t, msg.Value, sent[0].Value, "DLQ keeps the original payload for inspection")
	assert.Len(t, c.committed, 1, "poison message must not block the partition")
}

func TestApply_RepublishFailureDoesNotCommit(t *testing.T) {
	rr, w := newTestRunner(t)
	w.err = errors.New("broker down")
	c := &fakeCommitter{}
	msg := kafka.Message{Topic: "goshop.order.v1", Value: []byte(`{}`), Headers: []kafka.Header{{Key: headerRetryCount, Value: []byte("0")}}}

	err := rr.apply(context.Background(), "g", c, msg, func(messaging.Header, []byte) error {
		return errors.New("boom")
	})
	require.Error(t, err, "failed republish surfaces so the record is redelivered")
	assert.Empty(t, c.committed, "must never commit when the retry copy was not persisted")
}

func TestApply_ContextCancelledDuringBackoff(t *testing.T) {
	rr, w := newTestRunner(t)
	t.Setenv("KAFKA_RETRY_BACKOFF_BASE_MS", "5000")
	c := &fakeCommitter{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := rr.apply(ctx, "g", c, kafka.Message{Topic: "t", Value: []byte(`{}`)}, func(messaging.Header, []byte) error {
		return errors.New("boom")
	})
	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, c.committed)
	assert.Empty(t, w.messages())
}

func TestRetryBackoff_GrowsExponentially(t *testing.T) {
	t.Setenv("KAFKA_RETRY_BACKOFF_BASE_MS", "1000")
	assert.Equal(t, 1*time.Second, retryBackoff(0))
	assert.Equal(t, 2*time.Second, retryBackoff(1))
	assert.Equal(t, 4*time.Second, retryBackoff(2))
	assert.Equal(t, time.Second<<10, retryBackoff(50), "capped exponent keeps the shift safe")
}

// ---- header conversion & record building ------------------------------------

func TestToNeutralHeaders_DefaultsEventType(t *testing.T) {
	h := toNeutralHeaders([]kafka.Header{{Key: "x", Value: []byte("y")}})
	assert.Equal(t, "y", h.Get("x"))
	assert.Equal(t, "messages", h.Get(headerEventType), "missing event_type must never panic the filter")
}

func TestBuildRecord_EnrichesEveryEvent(t *testing.T) {
	b := &kafkaBroker{brokers: []string{"127.0.0.1:1"}, logger: newNopLogger(t), quit: make(chan struct{})}
	body := []byte(`{"event_id":"evt-1","event_type":"notification_events","trace_id":"tr-1","aggregate_id":"agg-9"}`)

	rec, err := b.buildRecord(context.Background(), "notification_events", body, 0)
	require.NoError(t, err)

	assert.Equal(t, "goshop.notification.v1", rec.Topic)
	assert.Equal(t, "agg-9", string(rec.Key), "aggregate_id becomes the partition key")
	assert.Equal(t, "evt-1", headerValue(rec.Headers, headerEventID))
	assert.Equal(t, "tr-1", headerValue(rec.Headers, headerTraceID))
	assert.Equal(t, "notification_events", headerValue(rec.Headers, headerEventType))
	assert.Equal(t, "1", headerValue(rec.Headers, headerEventVersion))
	assert.Equal(t, "0", headerValue(rec.Headers, headerRetryCount))
}

func TestBuildRecord_KeyFieldFallbackWhenAggregateIDMissing(t *testing.T) {
	b := &kafkaBroker{brokers: []string{"127.0.0.1:1"}, logger: newNopLogger(t), quit: make(chan struct{})}
	rec, err := b.buildRecord(context.Background(), "order.created", []byte(`{"order_uuid":"ord-7"}`), 0)
	require.NoError(t, err)

	assert.Equal(t, "goshop.order.v1", rec.Topic)
	assert.Equal(t, "ord-7", string(rec.Key))
}

// ---- topic routing & pattern matching ---------------------------------------

func TestTopicForEvent_KnownAndFallback(t *testing.T) {
	topic, key, err := TopicForEvent("order.created")
	require.NoError(t, err)
	assert.Equal(t, "goshop.order.v1", topic)
	assert.Equal(t, "order_uuid", key)

	topic, _, err = TopicForEvent("brand.new_event")
	require.NoError(t, err)
	assert.Equal(t, "goshop.brand.new_event.v1", topic, "unknown names get a deterministic v1 topic")

	_, _, err = TopicForEvent("  ")
	require.ErrorIs(t, err, errNameEmpty, "blank names fail fast instead of resolving to junk")
}

func TestMatchPattern(t *testing.T) {
	cases := []struct {
		pattern, key string
		want         bool
	}{
		{"order.*", "order.created", true},
		{"order.*", "order.status_changed", true},
		{"order.*", "payment.succeeded", false},
		{"*.*", "anything.here", true},
		{"order.*", "order", false},
		{"order.created", "order.created", true},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, matchPattern(c.pattern, c.key), "pattern %q vs %q", c.pattern, c.key)
	}
}

func TestTopicsForPattern_ExpandsWildcards(t *testing.T) {
	topics, err := topicsForPattern("order.*")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"goshop.order.v1"}, topics, "order.* resolves to the order domain topic")

	all, err := topicsForPattern("*.*")
	require.NoError(t, err)
	assert.Contains(t, all, "goshop.order.v1")
	assert.Contains(t, all, "goshop.payment.v1")
	assert.Contains(t, all, "goshop.user.v1")

	exact, err := topicsForPattern("payment.succeeded")
	require.NoError(t, err)
	assert.Equal(t, []string{"goshop.payment.v1"}, exact)

	fb, err := topicsForPattern("product.*")
	require.NoError(t, err)
	assert.Equal(t, []string{"goshop.product.v1"}, fb,
		"wildcard with no known keys falls back to a deterministic topic")

	_, err = topicsForPattern("  ")
	require.ErrorIs(t, err, errNameEmpty)
}

func TestFilterEventsByPatterns(t *testing.T) {
	calls := 0
	next := func(messaging.Header, []byte) error { calls++; return nil }
	filtered := filterEventsByPatterns([]string{"order.*"}, next)

	require.NoError(t, filtered(messaging.Header{headerEventType: "order.created"}, nil))
	require.NoError(t, filtered(messaging.Header{headerEventType: "payment.succeeded"}, nil))
	assert.Equal(t, 1, calls, "non-matching event types are skipped, not applied")
}

// ---- misc helpers -----------------------------------------------------------

func TestSanitize(t *testing.T) {
	assert.Equal(t, "notification_service_events-v3", sanitize("notification_service_events v3"))
}

func TestCloneRecord_PreservesPayloadAndTopic(t *testing.T) {
	in := kafka.Message{Topic: "t", Key: []byte("k"), Value: []byte("v"), Headers: []kafka.Header{
		{Key: headerRetryCount, Value: []byte("1")},
		{Key: headerMaxRetries, Value: []byte("9")},
	}}
	out := cloneRecord(in, 2, 5)

	assert.Equal(t, in.Topic, out.Topic)
	assert.Equal(t, in.Key, out.Key)
	assert.Equal(t, in.Value, out.Value)
	assert.Equal(t, "2", headerValue(out.Headers, headerRetryCount))
	assert.Equal(t, "5", headerValue(out.Headers, headerMaxRetries))
	assert.Equal(t, "1", headerValue(in.Headers, headerRetryCount), "input is never mutated")
}

func TestNewKafkaBroker_RejectsEmptyAddress(t *testing.T) {
	_, err := NewKafkaBroker("   ", newNopLogger(t))
	require.Error(t, err, "R7: no silent default to localhost or guest credentials")
}
