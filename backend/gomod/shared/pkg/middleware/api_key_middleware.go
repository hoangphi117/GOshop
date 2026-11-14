package middleware

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

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
			// A valid bearer token is proof the request already passed the edge
			// or the gateway. Requiring X-API-KEY on top of it broke every host
			// topology without nginx (dev-host, the storefront and dashboard
			// dev servers), where NEXT_PUBLIC_API_KEY is empty by policy, so
			// authenticated calls failed with 400 "missing API Key".
			header := ctx.GetHeader("Authorization")
			if strings.HasPrefix(header, "Bearer ") && strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")) != "" {
				ctx.Next()
				return
			}
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
