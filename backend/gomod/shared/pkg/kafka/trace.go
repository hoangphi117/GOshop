package kafka

import (
	"context"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
)

// kafkaCarrier adapts Kafka record headers to the OpenTelemetry
// TextMapCarrier interface, mirroring the old RabbitMQ adapter behavior.
type kafkaCarrier []kafka.Header

func (c kafkaCarrier) Get(key string) string {
	for _, h := range c {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c kafkaCarrier) Set(key, value string) {
	for i := range c {
		if c[i].Key == key {
			c[i].Value = []byte(value)
			return
		}
	}
}

func (c kafkaCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for _, h := range c {
		keys = append(keys, h.Key)
	}
	return keys
}

// injectTraceContext writes the active trace context (W3C traceparent) into
// the record headers, next to the plain trace_id string the workers log.
func injectTraceContext(ctx context.Context, headers []kafka.Header) []kafka.Header {
	if headers == nil {
		headers = make([]kafka.Header, 0, 8)
	}
	carrier := kafkaCarrier(headers)
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return []kafka.Header(carrier)
}

// extractTraceContext recovers a record's publish-time trace context. The
// returned context carries the span; handlers that accept a context can
// continue the same trace across the broker hop.
func extractTraceContext(headers []kafka.Header) context.Context {
	if len(headers) == 0 {
		return context.Background()
	}
	return otel.GetTextMapPropagator().Extract(context.Background(), kafkaCarrier(headers))
}
