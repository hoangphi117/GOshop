package main

// DLQ replay CLI (plan phase 3). Run after fixing whatever bug poisoned the
// records:
//
//	go run ./cmd/dlqreplay -topic goshop.order.v1.dlq
//	go run ./cmd/dlqreplay -all          # every known DLQ topic
//
// Records go back to their original topic with retry counters reset; the DLQ
// itself is left intact so nothing is lost if the replay was premature.

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"

	"goshop/shared/pkg/kafka"
	"goshop/shared/pkg/utils"
)

func main() {
	var topic string
	var all bool
	var idle int
	flag.StringVar(&topic, "topic", "", "DLQ topic to replay, e.g. goshop.order.v1.dlq")
	flag.BoolVar(&all, "all", false, "replay every known DLQ topic")
	flag.IntVar(&idle, "idle", 5, "seconds of silence before stopping")
	flag.Parse()

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	brokers := utils.GetEnv("KAFKA_BROKERS", "")
	if strings.TrimSpace(brokers) == "" {
		logger.Fatal().Msg("KAFKA_BROKERS is empty")
	}

	var topics []string
	switch {
	case all:
		topics = kafka.KnownDLQTopics()
	case topic != "":
		topics = []string{topic}
	default:
		fmt.Fprintln(os.Stderr, "usage: dlqreplay -topic <dlq-topic> | -all")
		os.Exit(2)
	}

	ctx := context.Background()
	failed := false
	for _, t := range topics {
		res, err := kafka.ReplayTopic(ctx, brokers, t, &logger)
		if err != nil {
			logger.Error().Err(err).Str("topic", t).Msg("Replay failed")
			failed = true
			continue
		}
		logger.Info().
			Str("topic", t).
			Int("scanned", res.Scanned).
			Int("replayed", res.Replayed).
			Int("skipped", res.Skipped).
			Int("errors", res.Errors).
			Msg("Replay finished")
		if res.Errors > 0 {
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}
