package telemetry

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// The distributed trace is only as good as the context that survives each hop.
// These checks pin the two bridges the system relies on: gRPC metadata and the
// logger trace_id, both of which silently degrade to a fresh id when broken.
func TestTraceContextSurvivesGRPCHop(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)))
	otel.SetTextMapPropagator(propagation.TraceContext{})

	ctx, parent := Tracer("test").Start(context.Background(), "parent")
	defer parent.End()

	// Client side: the interceptor injects the active context into metadata.
	var outMD metadata.MD
	clientInterceptor := UnaryClientInterceptor()
	err := clientInterceptor(ctx, "/test.Service/Call", nil, nil, nil,
		func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			outMD, _ = metadata.FromOutgoingContext(ctx)
			return nil
		})
	if err != nil {
		t.Fatalf("client interceptor: %v", err)
	}
	if vals := outMD.Get("traceparent"); len(vals) == 0 || vals[0] == "" {
		t.Fatal("client interceptor did not inject traceparent into outgoing metadata")
	}

	// Server side: the interceptor extracts it and opens a child span.
	serverCtx := metadata.NewIncomingContext(context.Background(), outMD)
	var handlerTraceID string
	_, err = UnaryServerInterceptor()(serverCtx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Call"},
		func(ctx context.Context, _ any) (any, error) {
			handlerTraceID = TraceIDFromContext(ctx)
			return nil, nil
		})
	if err != nil {
		t.Fatalf("server interceptor: %v", err)
	}

	if handlerTraceID != parent.SpanContext().TraceID().String() {
		t.Fatalf("trace id did not survive the gRPC hop: got %q want %q",
			handlerTraceID, parent.SpanContext().TraceID().String())
	}

	spans := recorder.Ended()
	childFound := false
	for _, s := range spans {
		if s.Name() == "/test.Service/Call" && s.Parent().SpanID() == parent.SpanContext().SpanID() {
			childFound = true
		}
	}
	if !childFound {
		t.Fatalf("server span is not a child of the client span; ended spans: %+v", spanNames(spans))
	}
}

func spanNames(spans []sdktrace.ReadOnlySpan) []string {
	names := make([]string, len(spans))
	for i, s := range spans {
		names[i] = s.Name()
	}
	return names
}

func TestInitTracerWithoutEndpointIsNoop(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	shutdown, err := InitTracer(context.Background(), "goshop-test", "0.0.0")
	if err != nil {
		t.Fatalf("InitTracer with no endpoint must not fail: %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown must never be nil")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("no-op shutdown returned an error: %v", err)
	}
}
