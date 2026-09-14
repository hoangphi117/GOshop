package middleware

import (
	"database/sql"
	"errors"
	"goshop/internal/db/sqlc"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	jwtService   auth.TokenService
	cacheService cache.RedisCacheService
)

func InitAuthMiddleware(service auth.TokenService, cache cache.RedisCacheService) {
	jwtService = service
	cacheService = cache
}

func AuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {

		authHeader := ctx.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing or invalid"})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		_, claims, err := jwtService.ParseToken(tokenString)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing or invalid"})
			return
		}

		if jti, ok := claims["jti"].(string); ok {
			key := "blacklist:" + jti
			exists, err := cacheService.Exists(key)
			if err == nil && exists {
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token has been revoked"})
				return
			}
		}

		payload, err := jwtService.DecryptAccessTokenPayload(tokenString)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		ctx.Set("user_uuid", payload.UserUUID)
		ctx.Set("user_email", payload.Email)
		ctx.Set("user_role", payload.Role)

		ctx.Next()
	}
}

func GetUserUUID(ctx *gin.Context) (uuid.UUID, error) {
	val, exists := ctx.Get("user_uuid")
	if !exists || val == nil {
		return uuid.Nil, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}
	userUUIDStr, ok := val.(string)
	if !ok || userUUIDStr == "" {
		return uuid.Nil, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}
	userUUID, err := uuid.Parse(userUUIDStr)
	if err != nil {
		return uuid.Nil, utils.NewError(utils.ErrCodeUnauthorized, "Invalid user UUID format")
	}
	return userUUID, nil
}

func GetUserFromContext(ctx *gin.Context, userRepo repository.UserRepository) (sqlc.User, error) {
	userUUID, err := GetUserUUID(ctx)
	if err != nil {
		return sqlc.User{}, err
	}

	reqCtx := ctx.Request.Context()
	user, err := userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.User{}, utils.NewError(utils.ErrCodeNotFound, "User not found")
		}
		return sqlc.User{}, utils.NewError(utils.ErrCodeNotFound, "User not found")
	}

	return user, nil
}
