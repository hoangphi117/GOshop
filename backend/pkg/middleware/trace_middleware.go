package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"goshop/pkg/logger"
)

func TraceMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		traceID := ctx.GetHeader("X-Trace-Id")
		if traceID == "" {
			traceID = uuid.NewString()
		}

		contextValue := logger.WithTraceID(ctx.Request.Context(), traceID)
		contextValue = logger.WithTraceID(contextValue, traceID)
		ctx.Request = ctx.Request.WithContext(contextValue)

		ctx.Writer.Header().Set("X-Trace-Id", traceID)

		ctx.Set("trace_id", traceID)

		ctx.Next()
	}
}
