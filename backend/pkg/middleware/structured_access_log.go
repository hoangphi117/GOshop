package middleware

import (
	"time"

	"goshop/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func StructuredAccessLog(log *zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		if log == nil {
			return
		}

		event := log.Info()
		if c.Writer.Status() >= 500 {
			event = log.Error()
		} else if c.Writer.Status() >= 400 {
			event = log.Warn()
		}

		event.
			Str("trace_id", logger.GetTraceId(c.Request.Context())).
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status_code", c.Writer.Status()).
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("HTTP request")
	}
}
