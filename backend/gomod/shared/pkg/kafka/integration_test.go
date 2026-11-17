//go:build integration

package kafka_test

// End-to-end Kafka checks against a real broker started by Testcontainers.
// These cover what unit tests can only fake: the retry chain against a live
// consumer group, DLQ landing, and replay back onto the source topic.
//
// Run with: go test -tags=integration ./shared/pkg/kafka/ -v
//
// Docker must be running. First run pulls apache/kafka:3.9.0 (~400MB).
// Set KAFKA_IT_DISABLE=1 to skip the suite on machines without Docker.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/rs/zerolog"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"goshop/shared/pkg/kafka"
	"goshop/shared/pkg/messaging"
)

const (
	itOrderTopic   = "goshop.order.v1"
	itOrderDLQ     = "goshop.order.v1.dlq"
	itPaymentTopic = "goshop.payment.v1"
	itPaymentDLQ   = "goshop.payment.v1.dlq"

	headerEventType = "event_type"
	headerEventID   = "event_id"
	headerRetry     = "retry_count"
	headerMaxRetry  = "max_retries"
	headerDLQOrigin = "dlq_original_topic"
	headerDLQGroup  = "dlq_consumer_group"
	headerDLQReason = "dlq_reason"
)

// itBroker starts one Kafka node and returns the host:port the test dials.
// Internal listeners stay on kafka:29092 for containers; the test needs a
// listener whose advertised address is reachable from the host, so a dedicated
// EXTERNAL listener is advertised on localhost with the mapped port.
// Publish EXTERNAL on a fixed host port so the advertised address is known
// before the broker boots (a mapped port is only known after start, and an
// advertised address cannot be rewritten afterwards).
const itHostPort = "19092"

func itBroker(t *testing.T) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	req := testcontainers.ContainerRequest{
		Image: "apache/kafka:3.9.0",
		Env: map[string]string{
			"KAFKA_NODE_ID":                  "1",
			"KAFKA_PROCESS_ROLES":            "broker,controller",
			"KAFKA_CONTROLLER_QUORUM_VOTERS": "1@localhost:9093",
			"KAFKA_LISTENERS":                "PLAINTEXT://0.0.0.0:29092,CONTROLLER://0.0.0.0:9093,EXTERNAL://0.0.0.0:" + itHostPort,
			// Host dials 127.0.0.1:19092 and must be able to reach every address
			// the broker advertises for that listener, so the internal listener
			// is dropped: EXTERNAL is the only client-facing one.
			"KAFKA_ADVERTISED_LISTENERS":                     "CONTROLLER://localhost:9093,EXTERNAL://localhost:" + itHostPort,
			"KAFKA_CONTROLLER_LISTENER_NAMES":                "CONTROLLER",
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP":           "CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT,EXTERNAL:PLAINTEXT",
			"KAFKA_INTER_BROKER_LISTENER_NAME":               "EXTERNAL",
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR":         "1",
			"KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR": "1",
			"KAFKA_TRANSACTION_STATE_LOG_MIN_ISR":            "1",
			"KAFKA_AUTO_CREATE_TOPICS_ENABLE":                "false",
			"KAFKA_HEAP_OPTS":                                "-Xmx512m -Xms256m",
			"CLUSTER_ID":                                     "goshop-it-kafka-0000000001",
		},
		ExposedPorts: []string{itHostPort + "/tcp"},
		HostConfigModifier: func(hc *container.HostConfig) {
			// Fixed host port: the broker advertises it, so the test must dial
			// exactly this one.
			hc.PortBindings = network.PortMap{
				network.MustParsePort(itHostPort + "/tcp"): []network.PortBinding{
					{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: itHostPort},
				},
			}
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("Kafka Server started").WithStartupTimeout(3*time.Minute),
			wait.ForListeningPort(itHostPort+"/tcp").WithStartupTimeout(3*time.Minute),
		),
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "start kafka container")
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })

	// Topics come from createTopics because auto-create is off.
	return "127.0.0.1:" + itHostPort
}

// readAll collects every record on partition 0 from the beginning. A raw
// leader connection is used instead of a Reader because StartOffset with an
// explicit Partition does not rewind reliably on a freshly created topic.
func readAll(t *testing.T, addr, topic string, idle time.Duration) []kafkago.Message {
	t.Helper()
	conn, err := kafkago.DialLeader(context.Background(), "tcp", addr, topic, 0)
	if err != nil {
		return nil
	}
	defer conn.Close()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(idle)))
	batch := conn.ReadBatch(1, 10<<20)
	msgs := make([]kafkago.Message, 0, 4)
	for {
		m, rerr := batch.ReadMessage()
		if rerr != nil {
			break
		}
		msgs = append(msgs, m)
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(idle)))
	}
	_ = batch.Close()
	return msgs
}

// createTopics asks the broker for the topics this suite needs. Auto-create is
// off, so a typo fails here instead of silently producing into an empty topic.
func createTopics(t *testing.T, addr string, topics ...string) {
	t.Helper()
	conn, err := kafkago.Dial("tcp", addr)
	require.NoError(t, err)

	ctrl, err := conn.Controller()
	require.NoError(t, err)
	_ = conn.Close()

	admin, err := kafkago.Dial("tcp", fmt.Sprintf("%s:%d", ctrl.Host, ctrl.Port))
	require.NoError(t, err)
	defer admin.Close()

	cfgs := make([]kafkago.TopicConfig, 0, len(topics))
	for _, topic := range topics {
		cfgs = append(cfgs, kafkago.TopicConfig{Topic: topic, NumPartitions: 1, ReplicationFactor: 1})
	}
	// CreateTopics is idempotent: an existing topic is left untouched.
	require.NoError(t, admin.CreateTopics(cfgs...))

	// The create returns before a leader is elected. Until one exists, producers
	// fail with UnknownTopicOrPartition and readers with Not Leader.
	for _, topic := range topics {
		topic := topic
		require.Eventually(t, func() bool {
			c, derr := kafkago.Dial("tcp", addr)
			if derr != nil {
				return false
			}
			defer c.Close()
			parts, perr := c.ReadPartitions(topic)
			return perr == nil && len(parts) == 1 && parts[0].Leader.Host != "" && parts[0].Leader.Port != 0
		}, 30*time.Second, 200*time.Millisecond, "topic %s never got a leader", topic)
	}
}

func headerOf(msg kafkago.Message, key string) string {
	for _, h := range msg.Headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

// waitForGroupJoin blocks until the consumer group has a member on the topic,
// instead of sleeping and hoping. It returns once the group reports a member;
// a record produced right after is still delivered because the reader commits
// from its own initial offset, not from a partition assignment race.
func waitForGroupJoin(t *testing.T, addr, group string) {
	waitForGroupJoinWith(t, nil, addr, group)
}

// waitForGroupJoinWith is waitForGroupJoin for a broker that needs
// authentication; a nil dialer means plaintext.
func waitForGroupJoinWith(t *testing.T, dialer *kafkago.Dialer, addr, group string) {
	t.Helper()
	client := kafkago.Client{Addr: kafkago.TCP(addr)}
	if dialer != nil {
		client.Transport = &kafkago.Transport{SASL: dialer.SASLMechanism, TLS: dialer.TLS}
	}

	require.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		resp, err := client.DescribeGroups(ctx, &kafkago.DescribeGroupsRequest{GroupIDs: []string{group}})
		if err != nil || len(resp.Groups) == 0 {
			return false
		}
		g := resp.Groups[0]
		return g.Error == nil && len(g.Members) > 0
	}, 45*time.Second, 300*time.Millisecond, "consumer group %s never joined", group)
}

// newWriter returns a producer that retries on transient broker errors:
// metadata for a freshly created topic can lag a moment behind CreateTopics,
// and the first write would otherwise fail with UnknownTopicOrPartition.
func newWriter(addr string) *kafkago.Writer {
	return &kafkago.Writer{
		Addr:         kafkago.TCP(addr),
		Balancer:     &kafkago.Hash{},
		RequiredAcks: kafkago.RequireAll,
		MaxAttempts:  10,
		// Back off between attempts so a metadata refresh can catch up; the
		// default 100ms×attempts gives up before propagation finishes.
		WriteBackoffMin: 200 * time.Millisecond,
		WriteBackoffMax: 2 * time.Second,
	}
}

func nopLog(t *testing.T) *zerolog.Logger {
	t.Helper()
	// stderr, not Nop: a consumer that silently fails to join leaves nothing to
	// debug, so failures must be visible in test output.
	l := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, NoColor: true}).Level(zerolog.DebugLevel).With().Timestamp().Logger()
	return &l
}

// ---- tests ------------------------------------------------------------------

// A handler that always fails must produce exactly one dead letter and leave no
// retry copies on the source topic. This is the regression that produced 15
// copies of one record when retries were republished onto the source topic.
func TestIntegration_PoisonRecordLandsInDLQWithoutDuplication(t *testing.T) {
	addr := itBroker(t)
	createTopics(t, addr, itOrderTopic, itOrderDLQ)
	t.Setenv("KAFKA_MAX_RETRIES", "2")
	t.Setenv("KAFKA_RETRY_BACKOFF_BASE_MS", "100")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	broker, err := kafka.NewKafkaBroker(addr, nopLog(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = broker.Close() })

	var calls int32
	poison := func(messaging.Header, []byte) error {
		atomic.AddInt32(&calls, 1)
		return errors.New("poison: handler always fails")
	}
	require.NoError(t, broker.SubscribeToExchange(ctx, "goshop-direct", "it-poison", []string{"order.created"}, poison))
	waitForGroupJoin(t, addr, "goshop.it-poison")
	// The group can report a member while the partition assignment is still
	// settling; kafka-go retries the fetch, so a short settle is enough.
	time.Sleep(5 * time.Second)

	eventID := "it-poison-1"
	w := newWriter(addr)
	require.Eventually(t, func() bool {
		return w.WriteMessages(ctx, kafkago.Message{
			Topic: itOrderTopic,
			Key:   []byte(eventID),
			Value: []byte(`{"event_id":"` + eventID + `","order_uuid":"00000000-0000-0000-0000-000000000001"}`),
			Headers: []kafkago.Header{
				{Key: headerEventType, Value: []byte("order.created")},
				{Key: headerEventID, Value: []byte(eventID)},
			},
		}) == nil
	}, 30*time.Second, 500*time.Millisecond, "poison record never accepted by the broker")
	require.NoError(t, w.Close())

	// Poll instead of sleeping: the retry chain ends with 100ms + 200ms backoff.
	var dead []kafkago.Message
	require.Eventually(t, func() bool {
		dead = readAll(t, addr, itOrderDLQ, 500*time.Millisecond)
		return len(dead) > 0
	}, 30*time.Second, 500*time.Millisecond, "poison record never reached the DLQ")

	time.Sleep(2 * time.Second) // duplicates, if any, arrive here
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls), "handler must run once per allowed attempt")

	dead = readAll(t, addr, itOrderDLQ, 2*time.Second)
	require.Len(t, dead, 1, "one poison record must produce exactly one dead letter")
	source := readAll(t, addr, itOrderTopic, 2*time.Second)
	require.Len(t, source, 1, "retry must not republish onto the source topic")
	assert.Equal(t, itOrderTopic, headerOf(dead[0], headerDLQOrigin))
	assert.Equal(t, "goshop.it-poison", headerOf(dead[0], headerDLQGroup))
	assert.Equal(t, eventID, headerOf(dead[0], headerEventID), "payload identity survives the move")
	assert.NotEmpty(t, headerOf(dead[0], headerDLQReason))

	var body struct {
		EventID string `json:"event_id"`
	}
	require.NoError(t, json.Unmarshal(dead[0].Value, &body), "the DLQ copy keeps the original body byte-for-byte")
	assert.Equal(t, eventID, body.EventID)
}

// A handler that succeeds must be called once, and nothing may reach the DLQ.
func TestIntegration_HappyPathDeliversOnceAndLeavesNoDeadLetter(t *testing.T) {
	addr := itBroker(t)
	createTopics(t, addr, itOrderTopic, itOrderDLQ)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	broker, err := kafka.NewKafkaBroker(addr, nopLog(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = broker.Close() })

	received := make(chan string, 8)
	require.NoError(t, broker.SubscribeToExchange(ctx, "goshop-direct", "it-happy", []string{"order.created"}, func(h messaging.Header, body []byte) error {
		var payload struct {
			EventID string `json:"event_id"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return err
		}
		received <- payload.EventID
		return nil
	}))
	waitForGroupJoin(t, addr, "goshop.it-happy")
	time.Sleep(5 * time.Second)

	w := newWriter(addr)
	require.Eventually(t, func() bool {
		return w.WriteMessages(ctx, kafkago.Message{
			Topic: itOrderTopic,
			Key:   []byte("it-happy-1"),
			Value: []byte(`{"event_id":"it-happy-1","order_uuid":"00000000-0000-0000-0000-000000000001"}`),
			Headers: []kafkago.Header{
				{Key: headerEventType, Value: []byte("order.created")},
				{Key: headerEventID, Value: []byte("it-happy-1")},
			},
		}) == nil
	}, 30*time.Second, 500*time.Millisecond, "happy record never accepted by the broker")
	require.NoError(t, w.Close())

	select {
	case got := <-received:
		assert.Equal(t, "it-happy-1", got)
	case <-time.After(15 * time.Second):
		t.Fatal("handler never received the order.created event")
	}

	// A second delivery would mean the manual commit did not stick.
	select {
	case dup := <-received:
		t.Fatalf("record delivered twice: %s", dup)
	case <-time.After(3 * time.Second):
	}

	assert.Empty(t, readAll(t, addr, itOrderDLQ, 2*time.Second), "a healthy record must not reach the DLQ")
}

// ReplayTopic must move a dead letter back onto its origin topic with a fresh
// retry budget and without the DLQ-only headers.
func TestIntegration_ReplayReturnsDeadLetterToSourceTopic(t *testing.T) {
	addr := itBroker(t)
	createTopics(t, addr, itPaymentTopic, itPaymentDLQ)

	// Seed one dead letter by hand: this is the shape stampDLQ produces.
	// WriteMessages is retried by the test because the very first write after
	// CreateTopics can still race the writer's metadata cache.
	w := newWriter(addr)
	seed := kafkago.Message{
		Topic: itPaymentDLQ,
		Key:   []byte("pay-1"),
		Value: []byte(`{"event_id":"pay-1","amount":100000}`),
		Headers: []kafkago.Header{
			{Key: headerEventType, Value: []byte("payment.succeeded")},
			{Key: headerEventID, Value: []byte("pay-1")},
			{Key: headerRetry, Value: []byte("2")},
			{Key: headerMaxRetry, Value: []byte("2")},
			{Key: headerDLQOrigin, Value: []byte(itPaymentTopic)},
			{Key: headerDLQGroup, Value: []byte("goshop.it.payment")},
			{Key: headerDLQReason, Value: []byte("seeded for replay")},
			{Key: "dlq_source", Value: []byte("broker")},
		},
	}
	require.Eventually(t, func() bool {
		err := w.WriteMessages(context.Background(), seed)
		return err == nil
	}, 30*time.Second, 500*time.Millisecond, "seed write never succeeded")
	require.NoError(t, w.Close())

	res, err := kafka.ReplayTopic(context.Background(), addr, itPaymentDLQ, nopLog(t))
	require.NoError(t, err)
	t.Logf("replay result: %+v", res)
	assert.Equal(t, 1, res.Replayed)
	assert.Zero(t, res.Errors)

	// The replay write is asynchronous to other connections' metadata; poll
	// until the record is readable instead of assuming it lands instantly.
	var replayed []kafkago.Message
	require.Eventually(t, func() bool {
		replayed = readAll(t, addr, itPaymentTopic, 500*time.Millisecond)
		return len(replayed) > 0
	}, 20*time.Second, 500*time.Millisecond, "the dead letter never landed back on its origin topic")
	require.Len(t, replayed, 1, "exactly one record is replayed")
	assert.Equal(t, "pay-1", headerOf(replayed[0], headerEventID))
	assert.Equal(t, "0", headerOf(replayed[0], headerRetry), "replay grants a fresh retry budget")
	assert.Empty(t, headerOf(replayed[0], headerDLQOrigin), "DLQ-only headers are stripped")
}

// TestMain fails loudly rather than reporting green when Docker is unavailable.
func TestMain(m *testing.M) {
	if os.Getenv("KAFKA_IT_DISABLE") != "" {
		fmt.Println("KAFKA_IT_DISABLE set: skipping Kafka integration suite")
		os.Exit(0)
	}
	os.Exit(m.Run())
}
