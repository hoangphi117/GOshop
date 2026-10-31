package rabbitmq

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
)

// amqpCarrier adapts AMQP message headers to the OpenTelemetry TextMapCarrier
// interface. AMQP header values carry non-string types in general, but the W3C
// propagator only ever reads/writes its own string keys.
type amqpCarrier amqp.Table

func (c amqpCarrier) Get(key string) string {
	if v, ok := c[key].(string); ok {
		return v
	}
	return ""
}

func (c amqpCarrier) Set(key, value string) {
	c[key] = value
}

func (c amqpCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// injectTraceContext writes the active trace context (W3C traceparent) into
// the message headers, next to the plain trace_id string the workers log.
func injectTraceContext(ctx context.Context, headers amqp.Table) {
	if headers == nil {
		headers = amqp.Table{}
	}
	otel.GetTextMapPropagator().Inject(ctx, amqpCarrier(headers))
}

// extractTraceContext recovers an amqp message's publish-time trace context.
// The returned context carries the span; handlers that accept a context can
// continue the same trace across the broker hop.
func extractTraceContext(headers amqp.Table) context.Context {
	if headers == nil {
		return context.Background()
	}
	return otel.GetTextMapPropagator().Extract(context.Background(), amqpCarrier(headers))
}
