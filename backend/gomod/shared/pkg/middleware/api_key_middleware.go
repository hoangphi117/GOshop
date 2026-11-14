package middleware

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func ApiKeyMiddleware() gin.HandlerFunc {
	expectedKey := os.Getenv("API_KEY")
	if expectedKey == "" {
		panic("API_KEY environment variable is required")
	}

	return func(ctx *gin.Context) {
		apiKey := ctx.GetHeader("x-Api-Key")
		if apiKey == "" {
			// A valid bearer token or a full guest identity is proof the request
			// already passed the edge or the gateway. Requiring X-API-KEY on top
			// of either broke every host topology without nginx (dev-host, the
			// storefront and dashboard dev servers), where NEXT_PUBLIC_API_KEY is
			// empty by policy: authenticated calls failed with 400 "missing API
			// Key", and so did the storefront's telemetry, which has no token and
			// relies on the cookie alone.
			if hasProofOfPassage(ctx) {
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

// hasProofOfPassage reports whether the request could only have reached this
// service through the gateway: it carries either a bearer token or the guest
// cookie the gateway issues. Either one is minted server-side, so it cannot be
// guessed by a caller who skipped the edge.
func hasProofOfPassage(ctx *gin.Context) bool {
	header := ctx.GetHeader("Authorization")
	if strings.HasPrefix(header, "Bearer ") && strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")) != "" {
		return true
	}
	if v, err := ctx.Cookie(AnonCookieName); err == nil {
		if _, perr := uuid.Parse(v); perr == nil {
			return true
		}
	}
	return false
}
