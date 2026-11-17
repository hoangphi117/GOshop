//go:build integration

package kafka_test

// SASL checks: the adapter must authenticate with SCRAM and refuse a wrong
// password. These start a dedicated broker with a SASL listener, because the
// plaintext suite in integration_test.go must stay runnable on hosts without a
// JAAS setup.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/rs/zerolog"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/scram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"goshop/shared/pkg/kafka"
	"goshop/shared/pkg/messaging"
)

const (
	saslHostPort  = "19093"
	saslAdminUser = "admin"
	saslAdminPass = "admin-secret"
	saslTestTopic = "goshop.order.v1"

	// JAAS for the broker itself: it authenticates to its own SASL listener for
	// inter-broker requests, so a super user must exist before the first client
	// connects. The listener below leaves one PLAINTEXT port open at start so
	// that super user can be created; see saslBroker.
	jaasConfig = `KafkaServer {
    org.apache.kafka.common.security.scram.ScramLoginModule required
    username="admin"
    password="admin-secret";
};`
)

// saslBroker starts a broker with a SCRAM-SHA-512 listener and returns its
// host address plus a dialer that is already authenticated as the admin user.
func saslBroker(t *testing.T) (string, *kafkago.Dialer) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	// jaas.conf has to exist before the process starts, so it is written to a
	// temp file and bind mounted read-only.
	jaasFile := t.TempDir() + "/jaas.conf"
	require.NoError(t, writeFile(jaasFile, jaasConfig))

	req := testcontainers.ContainerRequest{
		Image: "apache/kafka:3.9.0",
		Env: map[string]string{
			"KAFKA_NODE_ID":                          "1",
			"KAFKA_PROCESS_ROLES":                    "broker,controller",
			"KAFKA_CONTROLLER_QUORUM_VOTERS":         "1@127.0.0.1:9093",
			"KAFKA_OPTS":                             "-Djava.security.auth.login.config=/tmp/jaas.conf",
			"KAFKA_LISTENERS":                        "PLAINTEXT://0.0.0.0:29092,CONTROLLER://0.0.0.0:9093,EXTERNAL://0.0.0.0:" + saslHostPort,
			"KAFKA_ADVERTISED_LISTENERS":             "PLAINTEXT://127.0.0.1:29092,CONTROLLER://127.0.0.1:9093,EXTERNAL://127.0.0.1:" + saslHostPort,
			"KAFKA_CONTROLLER_LISTENER_NAMES":        "CONTROLLER",
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP":   "CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT,EXTERNAL:SASL_PLAINTEXT",
			"KAFKA_INTER_BROKER_LISTENER_NAME":       "PLAINTEXT",
			"KAFKA_SASL_ENABLED_MECHANISMS":          "SCRAM-SHA-512",
			"KAFKA_SUPER_USERS":                      "User:" + saslAdminUser,
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR": "1",
			"KAFKA_AUTO_CREATE_TOPICS_ENABLE":        "false",
			"KAFKA_HEAP_OPTS":                        "-Xmx512m -Xms256m",
			"CLUSTER_ID":                             "goshop-sasl-kafka-000000001",
		},
		ExposedPorts: []string{saslHostPort + "/tcp"},
		// A file mount, not a raw bind string: a Windows path passed as
		// "C:\...:C:\..." is rejected by the daemon, while Mounts converts it.
		Mounts: testcontainers.Mounts(testcontainers.ContainerMount{
			Source:   testcontainers.GenericBindMountSource{HostPath: jaasFile},
			Target:   "/tmp/jaas.conf",
			ReadOnly: true,
		}),
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.PortBindings = network.PortMap{
				network.MustParsePort(saslHostPort + "/tcp"): []network.PortBinding{
					{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: saslHostPort},
				},
			}
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("Kafka Server started").WithStartupTimeout(3*time.Minute),
			wait.ForListeningPort(saslHostPort+"/tcp").WithStartupTimeout(3*time.Minute),
		),
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "start sasl kafka container")
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })

	// Bootstrap the super user over the PLAINTEXT listener: SCRAM users live in
	// the cluster metadata, so the very first one cannot be created through an
	// authenticated connection.
	code, out, cerr := c.Exec(ctx, []string{"/opt/kafka/bin/kafka-configs.sh",
		"--bootstrap-server", "localhost:29092",
		"--alter", "--entity-type", "users", "--entity-name", saslAdminUser,
		"--add-config", "SCRAM-SHA-512=[password=" + saslAdminPass + "]",
	})
	require.NoError(t, cerr)
	var output string
	if out != nil {
		b, _ := io.ReadAll(out)
		output = string(b)
	}
	require.Zero(t, code, "create scram user: %s", output)

	m, err := scram.Mechanism(scram.SHA512, saslAdminUser, saslAdminPass)
	require.NoError(t, err)
	return "127.0.0.1:" + saslHostPort, &kafkago.Dialer{Timeout: 15 * time.Second, SASLMechanism: m}
}

// ---- tests ------------------------------------------------------------------

// The adapter must connect, publish and consume through a SCRAM broker when the
// credentials come from the environment.
func TestIntegration_SASLPublishesAndConsumes(t *testing.T) {
	addr, dialer := saslBroker(t)
	t.Setenv("KAFKA_SASL_USER", saslAdminUser)
	t.Setenv("KAFKA_SASL_PASSWORD", saslAdminPass)

	createTopicWith(t, dialer, addr, saslTestTopic)

	broker, err := kafka.NewKafkaBroker(addr, nopLog(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = broker.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	received := make(chan string, 4)
	require.NoError(t, broker.SubscribeToExchange(ctx, "goshop-direct", "sasl-check", []string{"order.created"},
		func(_ messaging.Header, body []byte) error {
			var payload struct {
				EventID string `json:"event_id"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				return err
			}
			received <- payload.EventID
			return nil
		}))
	waitForGroupJoinWith(t, dialer, addr, "goshop.sasl-check")
	time.Sleep(4 * time.Second)

	require.NoError(t, broker.Publish(ctx, "order.created", map[string]any{"event_id": "sasl-1", "order_uuid": "u1"}))

	select {
	case got := <-received:
		assert.Equal(t, "sasl-1", got)
	case <-time.After(20 * time.Second):
		t.Fatal("no record consumed over SASL")
	}
}

// A wrong password must fail loudly at construction/publish time, never fall
// back to an unauthenticated connection.
func TestIntegration_SASLRejectsWrongPassword(t *testing.T) {
	addr, dialer := saslBroker(t)
	createTopicWith(t, dialer, addr, saslTestTopic)

	t.Setenv("KAFKA_SASL_USER", saslAdminUser)
	t.Setenv("KAFKA_SASL_PASSWORD", "definitely-not-the-password")

	broker, err := kafka.NewKafkaBroker(addr, nopLog(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = broker.Close() })

	err = broker.Publish(context.Background(), "order.created", map[string]any{"event_id": "denied"})
	require.Error(t, err, "publishing with a bad password must fail")
	assert.NotContains(t, strings.ToLower(err.Error()), "success")
}

// PLAIN without TLS must be refused: it would send the password in clear text.
func TestIntegration_SASLPlainRequiresTLS(t *testing.T) {
	t.Setenv("KAFKA_SASL_USER", "someone")
	t.Setenv("KAFKA_SASL_PASSWORD", "secret")
	t.Setenv("KAFKA_SASL_MECHANISM", "PLAIN")

	_, err := kafka.NewKafkaBroker("127.0.0.1:1", nopLog(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "KAFKA_TLS_ENABLED")
}

// Half a credential is a deployment mistake, not a reason to connect anonymously.
func TestIntegration_SASLPasswordWithoutUserFails(t *testing.T) {
	t.Setenv("KAFKA_SASL_USER", "")
	t.Setenv("KAFKA_SASL_PASSWORD", "orphan-password")

	_, err := kafka.NewKafkaBroker("127.0.0.1:1", nopLog(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "KAFKA_SASL_USER")
}

// createTopicWith creates one topic through an authenticated connection.
func createTopicWith(t *testing.T, dialer *kafkago.Dialer, addr, topic string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	require.NoError(t, err)
	ctrl, err := conn.Controller()
	require.NoError(t, err)
	_ = conn.Close()

	admin, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", ctrl.Host, ctrl.Port))
	require.NoError(t, err)
	defer admin.Close()

	require.NoError(t, admin.CreateTopics(kafkago.TopicConfig{Topic: topic, NumPartitions: 1, ReplicationFactor: 1}))
	require.Eventually(t, func() bool {
		c, derr := dialer.DialContext(ctx, "tcp", addr)
		if derr != nil {
			return false
		}
		defer c.Close()
		parts, perr := c.ReadPartitions(topic)
		return perr == nil && len(parts) == 1 && parts[0].Leader.Host != ""
	}, 30*time.Second, 200*time.Millisecond, "topic %s never got a leader", topic)
}

// nopLog is a discard logger; kept so this file does not depend on the helper
// order in integration_test.go.
func saslNopLog() *zerolog.Logger {
	l := zerolog.Nop()
	return &l
}

// writeFile is a one-line os.WriteFile without importing os twice.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}
