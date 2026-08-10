package middleware

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func ApiKeyMiddleware() gin.HandlerFunc {

	expectedKey := os.Getenv("API_KEY")
	if expectedKey == "" {
		expectedKey = "secret-key"
	}

	return func(ctx *gin.Context) {
		apiKey := ctx.GetHeader("x-Api-Key")
		if apiKey == "" {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing API Key"})
			return
		}

		if apiKey != expectedKey {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API Key"})
			return
		}

		ctx.Next()
	}
}
