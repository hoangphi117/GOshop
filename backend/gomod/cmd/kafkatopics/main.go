package main

// Kafka topic bootstrap. Redpanda Cloud does not auto-create topics, and a
// publish to a missing topic fails in the publisher rather than the consumer,
// so the symptom shows up far from the cause as
//
//	[3] Unknown Topic Or Partition
//
// This creates every topic the services publish to, plus its .dlq sibling.
// Names come from kafka.DomainTopics(), which reads the same table the
// producers do, so this cannot drift from the code.
//
//	go run ./cmd/kafkatopics          # create (idempotent)
//	go run ./cmd/kafkatopics -list    # print the names and exit
//
// Safe to re-run: an existing topic is reported as such, not as an error.

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	segmentio "github.com/segmentio/kafka-go"

	"goshop/shared/pkg/kafka"
	"goshop/shared/pkg/utils"
)

func main() {
	var list bool
	var partitions int
	flag.BoolVar(&list, "list", false, "print the topics and exit")
	flag.IntVar(&partitions, "partitions", 1, "partitions per topic")
	flag.Parse()

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()

	topics := make([]string, 0, 2*len(kafka.DomainTopics()))
	for _, t := range kafka.DomainTopics() {
		topics = append(topics, t, kafka.DLQName(t))
	}

	if list {
		for _, t := range topics {
			fmt.Println(t)
		}
		return
	}

	brokers := strings.TrimSpace(utils.GetEnv("KAFKA_BROKERS", ""))
	if brokers == "" {
		logger.Fatal().Msg("KAFKA_BROKERS is empty")
	}

	dialer, err := kafka.DialerFromEnv()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to configure Kafka transport")
	}
	if dialer == nil {
		dialer = &segmentio.Dialer{Timeout: 10 * time.Second, DualStack: true}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// CreateTopics is only accepted by the controller, so find it first.
	bootstrap := strings.Split(brokers, ",")[0]
	conn, err := dialer.DialContext(ctx, "tcp", bootstrap)
	if err != nil {
		logger.Fatal().Err(err).Str("broker", bootstrap).Msg("Failed to dial broker")
	}
	controller, err := conn.Controller()
	_ = conn.Close()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to find controller")
	}

	ctrl, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to dial controller")
	}
	defer func() { _ = ctrl.Close() }()

	configs := make([]segmentio.TopicConfig, 0, len(topics))
	for _, t := range topics {
		configs = append(configs, segmentio.TopicConfig{
			Topic:             t,
			NumPartitions:     partitions,
			ReplicationFactor: 1,
		})
	}

	if err := ctrl.CreateTopics(configs...); err != nil {
		// Partial failure is reported as an error containing every topic that
		// could not be created; "already exists" is the idempotent re-run case.
		if strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "Unknown") {
			logger.Info().Msg("all topics already exist")
			return
		}
		logger.Fatal().Err(err).Msg("CreateTopics failed")
	}

	logger.Info().Int("total", len(topics)).Msg("kafka topics ready")
}
