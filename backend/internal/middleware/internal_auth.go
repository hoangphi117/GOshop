package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
)

// InternalAuthMiddleware verifies the X-Internal-Token header using constant-time comparison
// to prevent timing attacks. Only callers with the matching internal token can access internal routes.
func InternalAuthMiddleware(expectedSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Internal-Token")
		if token == "" || expectedSecret == "" || subtle.ConstantTimeCompare([]byte(token), []byte(expectedSecret)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized internal request",
			})
			return
		}
		c.Next()
	}
}
