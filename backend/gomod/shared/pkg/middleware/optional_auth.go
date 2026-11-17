package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// OptionalAuthMiddleware attaches user identity when a valid bearer token is
// present and stays silent otherwise. Unlike AuthMiddleware it never aborts:
// public endpoints that personalize for signed-in users share one route with
// their guest path, so a missing or expired token must degrade, not 401.
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") || jwtService == nil {
			ctx.Next()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if _, _, err := jwtService.ParseToken(tokenString); err != nil {
			ctx.Next()
			return
		}

		payload, err := jwtService.DecryptAccessTokenPayload(tokenString)
		if err != nil {
			ctx.Next()
			return
		}

		if payload.UserID > 0 {
			ctx.Set("user_id", payload.UserID)
		}
		ctx.Set("user_uuid", payload.UserUUID)
		ctx.Set("user_role", payload.Role)
		ctx.Next()
	}
}
