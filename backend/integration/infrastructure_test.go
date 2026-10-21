//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
)

func TestInfrastructureDependencies(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	databaseURL := envOrDefault("INTEGRATION_DATABASE_URL", "postgres://postgres:super%40password@localhost:5432/shopping?sslmode=disable")
	conn, err := pgx.Connect(ctx, databaseURL)
	require.NoError(t, err)
	defer conn.Close(ctx)

	var result int
	require.NoError(t, conn.QueryRow(ctx, "SELECT 1").Scan(&result))
	require.Equal(t, 1, result)

	rabbitURL := envOrDefault("INTEGRATION_RABBITMQ_URL", "amqp://goshop:goshop@localhost:5672/")
	rabbitConn, err := amqp.Dial(rabbitURL)
	require.NoError(t, err)
	defer rabbitConn.Close()

	channel, err := rabbitConn.Channel()
	require.NoError(t, err)
	defer channel.Close()

	queueName := fmt.Sprintf("goshop.integration.%d", time.Now().UnixNano())
	queue, err := channel.QueueDeclare(queueName, false, true, true, false, nil)
	require.NoError(t, err)

	message := []byte("integration-check")
	require.NoError(t, channel.PublishWithContext(ctx, "", queue.Name, false, false, amqp.Publishing{Body: message}))

	messages, err := channel.Consume(queue.Name, "", true, true, false, false, nil)
	require.NoError(t, err)

	select {
	case received := <-messages:
		require.Equal(t, message, received.Body)
	case <-ctx.Done():
		t.Fatal("timed out waiting for RabbitMQ message")
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
