package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	defaultAllowed := map[string]bool{
		"http://localhost:5173": true,
		"http://127.0.0.1:5173": true,
		"http://localhost:3000": true,
		"http://127.0.0.1:3000": true,
		"http://localhost:3001": true,
		"http://127.0.0.1:3001": true,
		"http://localhost:5500": true,
		"http://127.0.0.1:5500": true,
	}

	envOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if envOrigins != "" {
		for _, o := range strings.Split(envOrigins, ",") {
			trimmed := strings.TrimSpace(o)
			if trimmed != "" {
				defaultAllowed[trimmed] = true
			}
		}
	}

	return func(ctx *gin.Context) {
		origin := ctx.Request.Header.Get("Origin")

		if origin != "" {
			if defaultAllowed[origin] || os.Getenv("APP_ENV") != "production" {
				ctx.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			} else {
				ctx.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			}
		} else {
			ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		}

		ctx.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		ctx.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, x-api-key, X-API-KEY, X-Idempotency-Key, x-idempotency-key, Idempotency-Key, X-Trace-Id, X-Client-App, x-client-app")
		ctx.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		ctx.Writer.Header().Set("Access-Control-Max-Age", "86400")

		if ctx.Request.Method == "OPTIONS" {
			ctx.AbortWithStatus(http.StatusOK)
			return
		}

		ctx.Next()
	}
}
