//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
)

func TestInfrastructureDependencies(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	databaseURL := envOrDefault("INTEGRATION_DATABASE_URL", "postgres://postgres:super%40password@localhost:5432/postgres?sslmode=disable")
	conn, err := pgx.Connect(ctx, databaseURL)
	require.NoError(t, err)
	defer conn.Close(ctx)

	var result int
	require.NoError(t, conn.QueryRow(ctx, "SELECT 1").Scan(&result))
	require.Equal(t, 1, result)

	brokers := strings.Split(envOrDefault("INTEGRATION_KAFKA_BROKERS", "localhost:9092"), ",")

	topic := fmt.Sprintf("goshop.integration.%d", time.Now().UnixNano())
	group := "goshop.integration-" + uuid.NewString()

	// Consume BEFORE producing: the group would otherwise start at the latest
	// offset and miss the record written by this test.
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		GroupID:  group,
		Topic:    topic,
		MaxBytes: 1 << 20,
	})
	defer reader.Close()

	writer := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  topic,
		RequiredAcks:           kafka.RequireAll,
		AllowAutoTopicCreation: true,
	}
	defer writer.Close()

	// The first write may land on a broker that is still creating the topic and
	// answer UNKNOWN_TOPIC_OR_PARTITION; retry it rather than fail the whole
	// check on a race the broker settles on its own.
	payload := []byte("integration-check")
	var writeErr error
	for attempt := 0; attempt < 10; attempt++ {
		writeErr = writer.WriteMessages(ctx, kafka.Message{Value: payload})
		if writeErr == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.NoError(t, writeErr)

	msg, err := reader.ReadMessage(ctx)
	require.NoError(t, err, "timed out waiting for Kafka round-trip")
	require.Equal(t, payload, msg.Value)
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
