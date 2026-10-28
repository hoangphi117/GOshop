package rabbitmq

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestTraceContextRoundTripsThroughAMQPHeaders(t *testing.T) {
	otel.SetTracerProvider(sdktrace.NewTracerProvider())
	otel.SetTextMapPropagator(propagation.TraceContext{})

	ctx, span := otel.Tracer("test").Start(context.Background(), "publisher")
	defer span.End()

	headers := amqp.Table{"trace_id": "legacy-id"}
	injectTraceContext(ctx, headers)

	if got, _ := headers["traceparent"].(string); got == "" {
		t.Fatal("injectTraceContext did not write traceparent into headers")
	}
	if headers["trace_id"] != "legacy-id" {
		t.Fatalf("injectTraceContext must not clobber trace_id, got %v", headers["trace_id"])
	}

	consumerCtx := extractTraceContext(headers)
	got := trace.SpanContextFromContext(consumerCtx).TraceID().String()
	if want := span.SpanContext().TraceID().String(); got != want {
		t.Fatalf("trace id lost across the broker hop: got %q want %q", got, want)
	}
}
