package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"

	"goshop/shared/pkg/messaging"
	"goshop/shared/pkg/utils"
)

// retryRunner ports the RabbitMQ adapter's retry semantics to Kafka:
//
//	handler lỗi → chờ backoff → thử lại, tối đa maxRetries.
//	Hết lượt → publish bản sao vào topic DLQ sibling, commit, bỏ qua.
//
// Kafka không có Nack/requeue: một record không commit sẽ được redeliver và
// block toàn bộ partition, nên rule cứng là MỌI đường đi đều phải commit.
// Ponytail: backoff trong tiến trình block partition đang xử lý trong lúc
// sleep; nâng lên retry topic riêng khi cần backoff hàng phút.
type retryRunner struct {
	broker *kafkaBroker
}

func newRetryRunner(b *kafkaBroker) *retryRunner {
	return &retryRunner{broker: b}
}

// committerLike is the one Reader method retry needs. kafka.Reader satisfies it.
type committerLike interface {
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
}

// apply xử lý đúng 1 record: success → commit; handler lỗi → retry
// (republish với retry_count+1 rồi commit bản gốc); hết lượt → DLQ + commit.
// Chỉ trả lỗi khi bản thân commit/publish hạ tầng thất bại (khi đó coordinator
// sẽ redeliver và eventledger chặn trùng).
func (r *retryRunner) apply(ctx context.Context, group string, reader committerLike, msg kafka.Message, handler messaging.Handler) error {
	headers := toNeutralHeaders(msg.Headers)

	traceID := headers.Get(headerTraceID)
	if traceID == "" {
		traceID = string(msg.Key)
	}

	if err := handler(headers, msg.Value); err != nil {
		retryCount := headerInt(headers, headerRetryCount)
		maxRetries := headerIntOr(headers, headerMaxRetries, utils.GetIntEnv("KAFKA_MAX_RETRIES", defaultMaxRetries))

		r.broker.logger.Warn().
			Err(err).
			Str("trace_id", traceID).
			Int("retry_count", retryCount).
			Int("max_retries", maxRetries).
			Str("group", group).
			Str("topic", msg.Topic).
			Int64("offset", msg.Offset).
			Msg("Message handler failed")

		if retryCount < maxRetries {
			backoff := retryBackoff(retryCount)
			t := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				t.Stop()
				return ctx.Err()
			case <-t.C:
			}

			retryMsg := cloneRecord(msg, retryCount+1, maxRetries)
			if wErr := r.broker.writer1().WriteMessages(ctx, retryMsg); wErr != nil {
				// Republish retry thất bại → KHÔNG commit: coordinator sẽ
				// redeliver bản gốc, không mất message (song song với logic
				// Nack(requeue) của adapter cũ).
				r.broker.logger.Error().Err(wErr).Str("trace_id", traceID).Msg("Failed to republish retry record, message will be redelivered")
				return wErr
			}

			// Chỉ commit bản gốc sau khi bản retry đã được broker chấp nhận.
			return reader.CommitMessages(ctx, msg)
		}

		// Hết lượt retry → DLQ sibling, commit, đi tiếp (không block partition).
		dlqMsg := msg
		dlqMsg.Topic = DLQName(msg.Topic)
		if wErr := r.broker.writer1().WriteMessages(ctx, dlqMsg); wErr != nil {
			r.broker.logger.Error().Err(wErr).Str("trace_id", traceID).Msg("Failed to publish DLQ record, message will be redelivered")
			return wErr
		}
		r.broker.logger.Error().
			Str("trace_id", traceID).
			Str("dlq", dlqMsg.Topic).
			Int64("offset", msg.Offset).
			Msg("Message exceeded max retries, moved to DLQ")
		return reader.CommitMessages(ctx, msg)
	}

	return reader.CommitMessages(ctx, msg)
}

// retryBackoff trả 2^n * base, base từ KAFKA_RETRY_BACKOFF_BASE_MS (1s, 2s,
// 4s...), trùng nhịp với nhịp retry của hệ thống cũ.
func retryBackoff(retryCount int) time.Duration {
	base := time.Duration(utils.GetIntEnv("KAFKA_RETRY_BACKOFF_BASE_MS", defaultRetryBackoff)) * time.Millisecond
	if retryCount < 0 {
		retryCount = 0
	}
	if retryCount > 10 {
		retryCount = 10
	}
	return base * time.Duration(1<<uint(retryCount))
}

// cloneRecord copies a record back onto its own topic with an incremented
// retry counter (no read-modify hazard: new slice, key/value shared).
func cloneRecord(msg kafka.Message, retryCount, maxRetries int) kafka.Message {
	out := kafka.Message{
		Topic: msg.Topic,
		Key:   msg.Key,
		Value: msg.Value,
	}
	out.Headers = make([]kafka.Header, 0, len(msg.Headers)+1)
	seen := false
	for _, h := range msg.Headers {
		if h.Key == headerRetryCount {
			out.Headers = append(out.Headers, kafka.Header{Key: h.Key, Value: []byte(strconv.Itoa(retryCount))})
			seen = true
			continue
		}
		if h.Key == headerMaxRetries {
			out.Headers = append(out.Headers, kafka.Header{Key: h.Key, Value: []byte(strconv.Itoa(maxRetries))})
			continue
		}
		out.Headers = append(out.Headers, h)
	}
	if !seen {
		out.Headers = append(out.Headers, kafka.Header{Key: headerRetryCount, Value: []byte(strconv.Itoa(retryCount))})
	}
	return out
}

// toNeutralHeaders converts Kafka record headers into the broker-neutral
// string map handlers see. Kafka headers are []byte by design; failures to
// decode leave "" so a corrupt header never crashes a handler.
func toNeutralHeaders(src []kafka.Header) messaging.Header {
	dst := make(messaging.Header, len(src)+2)
	for _, h := range src {
		dst[h.Key] = string(h.Value)
	}
	if _, ok := dst[headerEventType]; !ok || dst[headerEventType] == "" {
		dst[headerEventType] = "messages"
	}
	return dst
}

// headerInt parses a numeric string header, 0 when absent or corrupt.
func headerInt(h messaging.Header, key string) int {
	n, _ := strconv.Atoi(h.Get(key))
	return n
}

// headerIntOr parses a numeric string header, falling back to def.
func headerIntOr(h messaging.Header, key string, def int) int {
	v := h.Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// setHeader upserts a header, allocating only when needed.
func setHeader(h []kafka.Header, key, value string) []kafka.Header {
	for i := range h {
		if h[i].Key == key {
			h[i].Value = []byte(value)
			return h
		}
	}
	return append(h, kafka.Header{Key: key, Value: []byte(value)})
}

// headerValue reads one header from a record header slice.
func headerValue(h []kafka.Header, key string) string {
	for _, x := range h {
		if x.Key == key {
			return string(x.Value)
		}
	}
	return ""
}

// extractJSONField reads a top-level string/number field from a JSON body.
// "" on any failure: key extraction is best-effort, never fatal.
func extractJSONField(body []byte, field string) string {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return ""
	}
	raw, ok := m[field]
	if !ok || len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return n.String()
	}
	return ""
}

// newEventID prefers the payload event_id (envelope đã có), fallback nano-timestamp.
func newEventID(body []byte) string {
	if id := extractJSONField(body, "event_id"); id != "" {
		return id
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
