package middleware

import (
	"context"
	"database/sql"
	"errors"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/utils"
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

		if jti, ok := claims["jti"].(string); ok && jti != "" {
			key := "blacklist:" + jti
			exists, err := cacheService.Exists(key)
			if err == nil && exists {
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token has been revoked"})
				return
			}
		}

		if fid, ok := claims["fid"].(string); ok && fid != "" {
			key := "blacklist:family:" + fid
			exists, err := cacheService.Exists(key)
			if err == nil && exists {
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Session has been revoked"})
				return
			}
		}

		payload, err := jwtService.DecryptAccessTokenPayload(tokenString)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		familyID := payload.FamilyID
		if fid, ok := claims["fid"].(string); ok && fid != "" {
			familyID = fid
		}
		if familyID != "" {
			ctx.Set("family_id", familyID)
		}

		if payload.UserID > 0 {
			ctx.Set("user_id", payload.UserID)
		}
		ctx.Set("user_uuid", payload.UserUUID)
		ctx.Set("user_email", payload.Email)
		ctx.Set("user_role", payload.Role)

		ctx.Next()
	}
}

func GetUserID(ctx *gin.Context) (int32, bool) {
	val, exists := ctx.Get("user_id")
	if !exists || val == nil {
		return 0, false
	}
	if id, ok := val.(int32); ok && id > 0 {
		return id, true
	}
	return 0, false
}

func MustGetUserID(ctx *gin.Context) (int32, error) {
	id, ok := GetUserID(ctx)
	if !ok || id <= 0 {
		return 0, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}
	return id, nil
}

func GetUserRole(ctx *gin.Context) (int32, bool) {
	val, exists := ctx.Get("user_role")
	if !exists || val == nil {
		return 0, false
	}
	if role, ok := val.(int32); ok {
		return role, true
	}
	return 0, false
}

func GetFamilyID(ctx *gin.Context) string {
	val, exists := ctx.Get("family_id")
	if !exists || val == nil {
		return ""
	}
	if str, ok := val.(string); ok {
		return str
	}
	return ""
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

type UserByUUIDFetcher[T any] interface {
	GetByUUID(ctx context.Context, u uuid.UUID) (T, error)
}

func GetUserFromContext[T any](ctx *gin.Context, userRepo UserByUUIDFetcher[T]) (T, error) {
	var zero T
	userUUID, err := GetUserUUID(ctx)
	if err != nil {
		return zero, err
	}

	reqCtx := ctx.Request.Context()
	user, err := userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return zero, utils.NewError(utils.ErrCodeNotFound, "User not found")
		}
		return zero, utils.NewError(utils.ErrCodeNotFound, "User not found")
	}

	return user, nil
}

