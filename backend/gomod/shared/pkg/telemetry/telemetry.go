// Package telemetry wires OpenTelemetry tracing for every GOshop service.
//
// Tracing is opt-in at runtime: when OTEL_EXPORTER_OTLP_ENDPOINT is empty the
// provider is a no-op, so a service runs without a collector. The provider is
// global, so every otel.Tracer(...) call in the process reports to it.
package telemetry

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"goshop/shared/pkg/logger"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// InitTracer installs the global tracer provider and text-map propagator.
//
// The returned shutdown function flushes pending spans; call it on exit. When
// tracing is disabled it is a no-op. InitTracer never fails the process: a
// collector that is down must not stop a service from serving requests.
func InitTracer(ctx context.Context, serviceName, version string) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if endpoint == "" {
		// No collector configured: keep the no-op provider, but the
		// propagator still injects traceparent so a later hop can be traced.
		return func(context.Context) error { return nil }, nil
	}

	// Dev collectors are plain http:// (jaeger:4318); Grafana Cloud Tempo is
	// https:// and must keep TLS — WithInsecure on an https URL would send
	// cleartext to port 443 and fail.
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpointURL(strings.TrimRight(endpoint, "/") + "/v1/traces"),
	}
	if !strings.HasPrefix(endpoint, "https://") {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version),
			semconv.DeploymentEnvironment(os.Getenv("APP_ENV")),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build trace resource: %w", err)
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(2*time.Second)),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRatio()))),
	)
	otel.SetTracerProvider(provider)
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.Log.Warn().Err(err).Msg("OpenTelemetry error")
	}))

	logger.Log.Info().Str("service", serviceName).Str("endpoint", endpoint).Msg("Tracing enabled")

	return provider.Shutdown, nil
}

// sampleRatio reads OTEL_TRACES_SAMPLER_ARG (default 1.0 = trace everything in
// dev). Lower it in production where only a fraction of spans is needed.
func sampleRatio() float64 {
	raw := os.Getenv("OTEL_TRACES_SAMPLER_ARG")
	if raw == "" {
		return 1.0
	}
	ratio, err := strconv.ParseFloat(raw, 64)
	if err != nil || ratio < 0 || ratio > 1 {
		return 1.0
	}
	return ratio
}

// Tracer returns a named tracer for the global provider.
func Tracer(name string) trace.Tracer { return otel.Tracer(name) }

// TraceIDFromContext returns the active span's trace id in hex, or "" when the
// context carries no span. Used to keep the log-level trace_id aligned with the
// distributed trace.
func TraceIDFromContext(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.HasTraceID() {
		return ""
	}
	return sc.TraceID().String()
}
