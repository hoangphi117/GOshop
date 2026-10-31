package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/telemetry"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
)

// TraceMiddleware opens the HTTP server span for every request and keeps the
// log-level trace_id aligned with it.
//
//   - Inbound W3C traceparent is respected, so a request that traverses gateway
//     -> gRPC -> RabbitMQ stays one trace.
//   - The response always carries X-Trace-Id (= the span's trace id) so a
//     customer's support ticket maps to an exact trace.
//   - When no collector is configured the span is non-recording and falls back
//     to a fresh UUID, preserving the historical behaviour bit for bit.
func TraceMiddleware() gin.HandlerFunc {
	propagator := otel.GetTextMapPropagator()
	tracer := otel.Tracer("goshop/http")

	return func(ctx *gin.Context) {
		requestCtx := propagator.Extract(ctx.Request.Context(), propagation.HeaderCarrier(ctx.Request.Header))

		path := ctx.FullPath()
		if path == "" {
			path = "unmatched " + ctx.Request.URL.Path
		}
		requestCtx, span := tracer.Start(requestCtx, path)
		defer span.End()
		ctx.Request = ctx.Request.WithContext(requestCtx)

		traceID := telemetry.TraceIDFromContext(requestCtx)
		if traceID == "" {
			traceID = uuid.NewString()
		}
		requestCtx = logger.WithTraceID(requestCtx, traceID)
		ctx.Request = ctx.Request.WithContext(requestCtx)

		ctx.Writer.Header().Set("X-Trace-Id", traceID)
		ctx.Set("trace_id", traceID)

		ctx.Next()

		if span.IsRecording() {
			span.SetAttributes(
				attribute.String("http.request.method", ctx.Request.Method),
				attribute.String("url.path", ctx.Request.URL.Path),
				attribute.Int("http.response.status_code", ctx.Writer.Status()),
			)
			if ctx.Writer.Status() >= 500 {
				span.SetStatus(codes.Error, "HTTP internal error")
			}
		}
	}
}
