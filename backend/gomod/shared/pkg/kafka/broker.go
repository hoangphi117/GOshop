// Package kafka is the Kafka transport adapter behind messaging.Broker.
//
// Services depend on messaging.Broker and never import this package directly;
// app.go wires it via NewKafkaBroker. Topic naming, partition keys and
// matching rules live in topics.go; headers and trace propagation in trace.go;
// retry math in retry.go.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/segmentio/kafka-go"

	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/messaging"
	"goshop/shared/pkg/utils"
)

const (
	defaultMaxRetries    = 3
	defaultConsumerCount = 3
	defaultRetryBackoff  = 1000 // ms, nhân đôi mỗi lần retry (1s, 2s, 4s...)
)

// writerLike is the subset of kafka.Writer tests and alternate transports
// depend on. kafka.Writer itself satisfies it.
type writerLike interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

// kafkaBroker is the concrete Broker. One Writer is shared by all topics
// (WriteMessages names the topic per record); each Consume/Subscribe call
// owns its Reader group so consumer groups stay independent.
type kafkaBroker struct {
	brokers []string
	logger  *zerolog.Logger

	writerMu sync.Mutex
	writer   writerLike
	quit     chan struct{}
	closeMu  sync.Mutex
	closed   bool

	// lagMu guards reader stats for /health/messaging (R3). Readers
	// register themselves here; LagStats snapshots Reader.Stats() for
	// every live consumer group.
	lagMu   sync.Mutex
	readers map[*kafka.Reader]string
}

// registerReader tracks a consumer group reader for lag reporting.
func (b *kafkaBroker) registerReader(reader *kafka.Reader, group string) {
	b.lagMu.Lock()
	defer b.lagMu.Unlock()
	if b.readers == nil {
		b.readers = make(map[*kafka.Reader]string)
	}
	b.readers[reader] = group
}

// unregisterReader drops a closed reader from lag reporting.
func (b *kafkaBroker) unregisterReader(reader *kafka.Reader) {
	b.lagMu.Lock()
	defer b.lagMu.Unlock()
	delete(b.readers, reader)
}

// LagInfo returns one lag number per live consumer group. It is the
// health-handler surface for R3 (lagReporter); LagStats is the full
// kafka.ReaderStats detail for debugging.
func (b *kafkaBroker) LagStats() map[string]kafka.ReaderStats {
	b.lagMu.Lock()
	copies := make(map[*kafka.Reader]string, len(b.readers))
	for r, g := range b.readers {
		copies[r] = g
	}
	b.lagMu.Unlock()

	out := make(map[string]kafka.ReaderStats, len(copies))
	for r, g := range copies {
		out[g] = r.Stats()
	}
	return out
}

// NewKafkaBroker connects to the Kafka cluster and returns a messaging.Broker.
// Signature mirrors NewRabbitMQService so wiring a service is a one-line swap.
func NewKafkaBroker(brokers string, log *zerolog.Logger) (messaging.Broker, error) {
	if strings.TrimSpace(brokers) == "" {
		return nil, fmt.Errorf("kafka: KAFKA_BROKERS is empty, refusing default")
	}

	b := &kafkaBroker{
		brokers: strings.Split(strings.ReplaceAll(brokers, " ", ""), ","),
		logger:  log,
		quit:    make(chan struct{}),
	}
	return b, nil
}

// writer lazily builds the shared producer. kafka-go Writer is safe for
// concurrent use, so one instance serves every Publish call.
func (b *kafkaBroker) writer1() writerLike {
	b.writerMu.Lock()
	defer b.writerMu.Unlock()
	if b.writer != nil {
		return b.writer
	}
	b.writer = &kafka.Writer{
		Addr:         kafka.TCP(b.brokers...),
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireAll,
		MaxAttempts:  3,
		BatchTimeout: 10 * time.Millisecond,
		Async:        false,
	}
	return b.writer
}

// marshalBody matches the RabbitMQ adapter: raw bytes pass through,
// everything else is JSON-encoded.
func marshalBody(message any) ([]byte, error) {
	switch m := message.(type) {
	case []byte:
		return m, nil
	case json.RawMessage:
		return m, nil
	default:
		return json.Marshal(m)
	}
}

// buildRecord assembles one Kafka record: domain topic from the routing key,
// partition key from the aggregate_id, W3C trace headers.
func (b *kafkaBroker) buildRecord(ctx context.Context, name string, body []byte, retryCount int) (kafka.Message, error) {
	topic, keyField, err := TopicForEvent(name)
	if err != nil {
		return kafka.Message{}, err
	}
	name = strings.TrimSpace(name)

	headers := []kafka.Header{
		{Key: headerEventID, Value: []byte(newEventID(body))},
		{Key: headerEventType, Value: []byte(name)},
		{Key: headerEventVersion, Value: []byte(defaultEventVersion)},
		{Key: headerRetryCount, Value: []byte(fmt.Sprintf("%d", retryCount))},
		{Key: headerMaxRetries, Value: []byte(fmt.Sprintf("%d", utils.GetIntEnv("KAFKA_MAX_RETRIES", defaultMaxRetries)))},
	}

	// Prefer the envelope's canonical aggregate_id, fall back to the
	// per-topic key field. Runs for every event, keyed or not.
	agg := extractJSONField(body, "aggregate_id")
	if agg == "" && keyField != "" {
		agg = extractJSONField(body, keyField)
	}
	if v := extractJSONField(body, "event_id"); v != "" {
		headers = setHeader(headers, headerEventID, v)
	}
	if v := extractJSONField(body, "event_version"); v != "" {
		headers = setHeader(headers, headerEventVersion, v)
	}
	if v := extractJSONField(body, "trace_id"); v != "" {
		headers = setHeader(headers, headerTraceID, v)
	} else if traceID := logger.GetTraceId(ctx); traceID != "" {
		headers = setHeader(headers, headerTraceID, traceID)
	} else {
		headers = setHeader(headers, headerTraceID, fmt.Sprintf("%d", time.Now().UnixNano()))
	}
	if agg != "" {
		headers = setHeader(headers, headerAggregateID, agg)
	}
	headers = injectTraceContext(ctx, headers)

	rec := kafka.Message{Topic: topic, Value: body, Headers: headers}
	if agg != "" {
		rec.Key = []byte(agg)
	}
	return rec, nil
}

// Publish sends one message to the domain topic resolved from queue.
// Semantics mirror the RabbitMQ adapter: persistent, synchronous, error on
// failure.
func (b *kafkaBroker) Publish(ctx context.Context, queue string, message any) error {
	if strings.TrimSpace(queue) == "" {
		return errNameEmpty
	}
	body, err := marshalBody(message)
	if err != nil {
		return fmt.Errorf("kafka: failed to marshal message: %w", err)
	}

	rec, err := b.buildRecord(ctx, queue, body, 0)
	if err != nil {
		return err
	}
	if err := b.writer1().WriteMessages(ctx, rec); err != nil {
		return fmt.Errorf("kafka: failed to publish to %s: %w", rec.Topic, err)
	}
	return nil
}

// PublishTopic sends one message to the domain topic resolved from routingKey.
func (b *kafkaBroker) PublishTopic(ctx context.Context, routingKey string, message any) error {
	return b.Publish(ctx, routingKey, message)
}

// PublishToExchange maps the exchange name to a topic prefix. The project
// only uses one topic exchange (goshop.events), so exchange is ignored and
// routingKey alone decides the domain topic; the signature exists for
// interface parity.
func (b *kafkaBroker) PublishToExchange(ctx context.Context, exchange, routingKey string, message any) error {
	return b.Publish(ctx, routingKey, message)
}

// newGroupReader opens a consumer-group reader over one topic with manual
// offset commits (at-least-once: eventledger dedupes replays).
//
// Topic, not GroupTopics: kafka-go's GroupTopics path never joins the classic
// consumer-group protocol, so the broker assigns no partitions and lag stays
// N/A (seen Nov 2026). One reader per topic, all sharing GroupID, is the
// supported way to consume several topics in one group.
func (b *kafkaBroker) newGroupReader(group, topic string) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:        b.brokers,
		GroupID:        group,
		Topic:          topic,
		MinBytes:       1,
		MaxBytes:       1 << 20,
		CommitInterval: 0, // manual CommitMessages only
	})
}

// startGroupConsumer runs consumerCount FetchMessage loops on reader. Each
// loop is sequential per instance so partition order is preserved in-process;
// scale comes from more instances, not more goroutines sharing one reader.
func (b *kafkaBroker) startGroupConsumer(ctx context.Context, group string, reader *kafka.Reader, rr *retryRunner, handler messaging.Handler) {
	b.registerReader(reader, group)
	go func() {
		defer b.unregisterReader(reader)
		defer reader.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case <-b.quit:
				return
			default:
			}

			msg, err := reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				b.logger.Error().Err(err).Str("group", group).Msg("Failed to fetch message, retrying")
				time.Sleep(time.Second)
				continue
			}

			if err := rr.apply(ctx, group, reader, msg, handler); err != nil {
				b.logger.Error().Err(err).Str("group", group).Msg("Failed to apply message")
			}
		}
	}()
}

// filterEventsByPatterns wraps handler: records whose event_type matches none
// of patterns still get committed (skip) but no business logic runs. This is
// the Kafka replacement for the AMQP topic exchange: subscribe to the domain
// topics, filter event types in-app.
func filterEventsByPatterns(patterns []string, handler messaging.Handler) messaging.Handler {
	return func(headers messaging.Header, payload []byte) error {
		eventType := headers.Get(headerEventType)
		for _, p := range patterns {
			if matchPattern(p, eventType) {
				return handler(headers, payload)
			}
		}
		return nil
	}
}

// Consume registers group on the topic resolved from queue and starts
// KAFKA_CONSUMER_COUNT parallel member instances. Each member joins the same
// consumer group; partitions are owned by exactly one member, so one group =
// each message applied once.
func (b *kafkaBroker) Consume(ctx context.Context, queue string, handler messaging.Handler) error {
	if strings.TrimSpace(queue) == "" {
		return errNameEmpty
	}
	topic, _, err := TopicForEvent(queue)
	if err != nil {
		return err
	}
	group := "goshop." + sanitize(queue)

	consumerCount := utils.GetIntEnv("KAFKA_CONSUMER_COUNT", defaultConsumerCount)
	if consumerCount < 1 {
		consumerCount = 1
	}

	rr := newRetryRunner(b)
	for i := 0; i < consumerCount; i++ {
		reader := b.newGroupReader(group, topic)
		handler2 := filterEventsByPatterns([]string{queue}, handler)
		b.startGroupConsumer(ctx, group, reader, rr, handler2)
	}

	b.logger.Info().Str("group", group).Str("topic", topic).Msg("Kafka consumer registered")
	return nil
}

// SubscribeToExchange registers group on all topics matching routingKeys
// (exact keys or wildcards like "order.*") and filters event types in-app.
func (b *kafkaBroker) SubscribeToExchange(ctx context.Context, exchange, queue string, routingKeys []string, handler messaging.Handler) error {
	if strings.TrimSpace(queue) == "" {
		return errNameEmpty
	}

	seen := make(map[string]struct{}, 4)
	topics := make([]string, 0, 4)
	patterns := make([]string, 0, len(routingKeys))
	for _, rk := range routingKeys {
		patterns = append(patterns, rk)
		resolved, err := topicsForPattern(rk)
		if err != nil {
			return fmt.Errorf("kafka: invalid routing key %q: %w", rk, err)
		}
		for _, t := range resolved {
			if _, dup := seen[t]; dup {
				continue
			}
			seen[t] = struct{}{}
			topics = append(topics, t)
		}
	}
	if len(topics) == 0 {
		return fmt.Errorf("kafka: no topics resolved from routing keys %v", routingKeys)
	}

	group := "goshop." + sanitize(queue)
	consumerCount := utils.GetIntEnv("KAFKA_CONSUMER_COUNT", defaultConsumerCount)
	if consumerCount < 1 {
		consumerCount = 1
	}

	filtered := filterEventsByPatterns(patterns, handler)
	rr := newRetryRunner(b)
	// One reader per topic sharing GroupID: see newGroupReader. Multi-topic
	// groups fan out here; consumption stays exactly-once per group.
	for _, t := range topics {
		for i := 0; i < consumerCount; i++ {
			reader := b.newGroupReader(group, t)
			b.startGroupConsumer(ctx, group, reader, rr, filtered)
		}
	}

	b.logger.Info().Str("group", group).Strs("topics", topics).Msg("Kafka topic consumer registered")
	return nil
}

// SubscribeTopic resolves the default topic exchange for parity with the
// RabbitMQ adapter, then delegates to SubscribeToExchange.
func (b *kafkaBroker) SubscribeTopic(ctx context.Context, queue string, routingKeys []string, handler messaging.Handler) error {
	return b.SubscribeToExchange(ctx, "", queue, routingKeys, handler)
}

// Close shuts down the shared writer. Readers close with their consume loops.
func (b *kafkaBroker) Close() error {
	b.closeMu.Lock()
	defer b.closeMu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	close(b.quit)

	b.writerMu.Lock()
	defer b.writerMu.Unlock()
	if b.writer != nil {
		return b.writer.Close()
	}
	return nil
}

// IsHealthy dials the cluster leader. Cheap enough for /health polling.
func (b *kafkaBroker) IsHealthy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := kafka.DialContext(ctx, "tcp", b.brokers[0])
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// sanitize turns a queue name into a legal Kafka consumer group suffix.
func sanitize(name string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, strings.TrimSpace(name))
}

// LagInfo returns one lag number per live consumer group. It is the
// health-handler surface for R3 (lagReporter); LagStats is the full
// kafka.ReaderStats detail for debugging.
func (b *kafkaBroker) LagInfo() map[string]int64 {
	out := make(map[string]int64)
	for group, stats := range b.LagStats() {
		out[group] = stats.Lag
	}
	return out
}
