package kafka

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
	kafkago "github.com/segmentio/kafka-go"
)

// Phase 3: a dead letter is only useful if an operator can act on it. A bare
// record on `<topic>.dlq` loses why it died and where it came from, so every
// DLQ write is stamped with those headers and a replay tool can put the record
// back on its original topic once the bug is fixed.

// Headers added only on the DLQ copy.
const (
	headerDLQTopic  = "dlq_original_topic"
	headerDLQGroup  = "dlq_consumer_group"
	headerDLQOffset = "dlq_original_offset"
	headerDLQReason = "dlq_reason"
	headerDLQTime   = "dlq_failed_at"
	headerDLQSource = "dlq_source" // broker-produced copy vs. replayed manual copy
)

// dlqReasonMaxLen caps stored error text: a stack-trace-sized reason bloats
// every DLQ record and every kafka-ui preview.
const dlqReasonMaxLen = 512

// stampDLQ copies msg onto its DLQ sibling with the failure context attached.
// The original value and key are preserved byte-for-byte so replay is lossless.
func stampDLQ(msg kafkago.Message, group string, cause error) kafkago.Message {
	out := kafkago.Message{
		Topic: DLQName(msg.Topic),
		Key:   msg.Key,
		Value: msg.Value,
	}
	out.Headers = make([]kafkago.Header, 0, len(msg.Headers)+6)
	out.Headers = append(out.Headers, msg.Headers...)
	out.Headers = setHeader(out.Headers, headerDLQTopic, msg.Topic)
	out.Headers = setHeader(out.Headers, headerDLQGroup, group)
	out.Headers = setHeader(out.Headers, headerDLQOffset, fmt.Sprintf("%d", msg.Offset))
	out.Headers = setHeader(out.Headers, headerDLQTime, time.Now().UTC().Format(time.RFC3339))
	out.Headers = setHeader(out.Headers, headerDLQSource, "broker")
	reason := "unknown"
	if cause != nil {
		reason = cause.Error()
		if len(reason) > dlqReasonMaxLen {
			reason = reason[:dlqReasonMaxLen]
		}
	}
	out.Headers = setHeader(out.Headers, headerDLQReason, reason)
	return out
}

// ReplayResult summarises one replay run so a CLI can report without guessing.
type ReplayResult struct {
	Scanned  int
	Replayed int
	Skipped  int
	Errors   int
}

// ReplayTopic moves every record currently on a DLQ topic back to the topic it
// originally came from, resetting the retry counters so the consumer gets the
// full retry budget again. It returns when the DLQ is drained or ctx ends.
//
// This is the manual recovery path: run it after fixing the bug that poisoned
// the records. It is safe to run twice because the ledger-backed consumers skip
// events they already applied; a replay of a partially applied event is still
// possible, so replay deliberately does NOT delete from the DLQ source — the
// operator inspects the DLQ afterwards.
//
// ponytail: no DLQ compaction or auto-retry timer. Add a scheduler when
// failures become routine rather than exceptional.
func ReplayTopic(ctx context.Context, brokers, dlqTopic string, log *zerolog.Logger) (ReplayResult, error) {
	var res ReplayResult
	if dlqTopic == "" {
		return res, errors.New("kafka: dlq topic is required")
	}
	if len(brokers) == 0 {
		return res, errors.New("kafka: no brokers configured")
	}
	addrs := splitBrokers(brokers)

	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        addrs,
		Topic:          dlqTopic,
		GroupID:        "goshop.dlq.replay." + dlqTopic,
		MinBytes:       1,
		MaxBytes:       10 << 20,
		CommitInterval: 0,
	})
	defer reader.Close()

	writer := &kafkago.Writer{
		Addr:         kafkago.TCP(addrs...),
		Balancer:     &kafkago.Hash{},
		RequiredAcks: kafkago.RequireAll,
		MaxAttempts:  3,
		Async:        false,
	}
	defer writer.Close()

	// Bounded drain: FetchMessage blocks until a record arrives, so stop on
	// idle instead of hanging forever once the DLQ is empty.
	idle := 5 * time.Second
	for {
		fetchCtx, cancel := context.WithTimeout(ctx, idle)
		msg, err := reader.FetchMessage(fetchCtx)
		cancel()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) {
				break
			}
			return res, fmt.Errorf("read dlq %s: %w", dlqTopic, err)
		}
		res.Scanned++

		target := headerValue(msg.Headers, headerDLQTopic)
		if target == "" {
			res.Skipped++
			if log != nil {
				log.Warn().Str("dlq", dlqTopic).Int64("offset", msg.Offset).Msg("DLQ record has no original topic header, skipping")
			}
		} else {
			if err := writer.WriteMessages(ctx, buildReplayMessage(msg, target)); err != nil {
				res.Errors++
				if log != nil {
					log.Error().Err(err).Str("dlq", dlqTopic).Int64("offset", msg.Offset).Str("target", target).Msg("Failed to replay DLQ record")
				}
				// Do not commit a failed record: leave it on the DLQ.
				continue
			}
			res.Replayed++
			if log != nil {
				log.Info().Str("dlq", dlqTopic).Str("target", target).Int64("offset", msg.Offset).Msg("Replayed DLQ record")
			}
		}

		if err := reader.CommitMessages(ctx, msg); err != nil {
			res.Errors++
			if log != nil {
				log.Error().Err(err).Str("dlq", dlqTopic).Int64("offset", msg.Offset).Msg("Failed to commit replayed DLQ record")
			}
		}
	}
	return res, nil
}

// buildReplayMessage resets the retry counters and drops DLQ-only headers so
// the consumer treats the record as a fresh delivery on its original topic.
func buildReplayMessage(msg kafkago.Message, target string) kafkago.Message {
	out := kafkago.Message{Topic: target, Key: msg.Key, Value: msg.Value}
	out.Headers = make([]kafkago.Header, 0, len(msg.Headers))
	for _, h := range msg.Headers {
		switch h.Key {
		case headerDLQTopic, headerDLQGroup, headerDLQOffset, headerDLQReason, headerDLQTime, headerDLQSource:
			continue
		case headerRetryCount:
			out.Headers = append(out.Headers, kafkago.Header{Key: h.Key, Value: []byte("0")})
		default:
			out.Headers = append(out.Headers, h)
		}
	}
	return out
}

func splitBrokers(list string) []string {
	out := make([]string, 0, 2)
	for _, part := range strings.Split(list, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
