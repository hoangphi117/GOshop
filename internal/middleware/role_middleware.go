package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	RoleAdmin     int32 = 1
	RoleModerator int32 = 2
	RoleMember    int32 = 3
)

func RequireRole(allowedRoles ...int32) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		userRole, exists := ctx.Get("user_role")
		if !exists {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		role, ok := userRole.(int32)
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Invalid role"})
			return
		}

		for _, allowed := range allowedRoles {
			if role == allowed {
				ctx.Next()
				return
			}
		}

		ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
	}
}
