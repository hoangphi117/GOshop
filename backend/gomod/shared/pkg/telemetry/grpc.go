package telemetry

import (
	"context"

	"goshop/shared/pkg/logger"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// metadataCarrier adapts gRPC metadata to the OpenTelemetry TextMapCarrier
// interface. gRPC lowercases keys, so the canonical W3C "traceparent" key
// round-trips unchanged.
type metadataCarrier metadata.MD

func (c metadataCarrier) Get(key string) string {
	if vals := metadata.MD(c).Get(key); len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func (c metadataCarrier) Set(key, value string) {
	metadata.MD(c).Set(key, value)
}

func (c metadataCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// UnaryClientInterceptor propagates the active trace context to downstream
// services by injecting W3C headers into outgoing metadata. It deliberately
// does NOT create client spans: the server side already records a span per
// hop, and child links stay connected through the propagated context.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, request, reply any,
		conn *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
	) error {
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.MD{}
		} else {
			md = md.Copy()
		}
		otel.GetTextMapPropagator().Inject(ctx, metadataCarrier(md))
		return invoker(metadata.NewOutgoingContext(ctx, md), method, request, reply, conn, opts...)
	}
}

// UnaryServerInterceptor extracts the incoming trace context and opens a span
// per RPC, named after the full method. The logger-level trace_id is bridged
// from the span so handlers keep logging the same correlation id as before.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		ctx = otel.GetTextMapPropagator().Extract(ctx, metadataCarrier(md))

		ctx, span := Tracer("goshop/grpc").Start(ctx, info.FullMethod)
		defer span.End()

		if traceID := TraceIDFromContext(ctx); traceID != "" {
			ctx = logger.WithTraceID(ctx, traceID)
		}

		resp, err := handler(ctx, req)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
		}
		return resp, err
	}
}

// ServerOptions returns the gRPC server options that enable tracing. Services
// append them to their grpc.NewServer call, e.g.
//
//	grpc.NewServer(telemetry.ServerOptions(), ...)
func ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{grpc.ChainUnaryInterceptor(UnaryServerInterceptor())}
}
