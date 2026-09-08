package middleware

import (
	"crypto/subtle"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func ApiKeyMiddleware() gin.HandlerFunc {
	expectedKey := os.Getenv("API_KEY")
	if expectedKey == "" {
		panic("API_KEY environment variable is required")
	}

	return func(ctx *gin.Context) {
		apiKey := ctx.GetHeader("x-Api-Key")
		if apiKey == "" {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing API Key"})
			return
		}

		if subtle.ConstantTimeCompare([]byte(apiKey), []byte(expectedKey)) != 1 {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API Key"})
			return
		}

		ctx.Next()
	}
}
