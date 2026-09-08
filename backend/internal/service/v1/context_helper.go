package v1service

import (
	"database/sql"
	"errors"
	"goshop/internal/db/sqlc"
	"goshop/internal/repository"
	"goshop/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// GetUserUUIDFromContext extracts and parses user_uuid from gin.Context.
// Returns an Unauthorized error if the key is missing or is not a valid UUID string.
func GetUserUUIDFromContext(ctx *gin.Context) (uuid.UUID, error) {
	userUUIDVal, exists := ctx.Get("user_uuid")
	if !exists || userUUIDVal == nil {
		return uuid.Nil, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}

	userUUIDStr, ok := userUUIDVal.(string)
	if !ok || userUUIDStr == "" {
		return uuid.Nil, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}

	userUUID, err := uuid.Parse(userUUIDStr)
	if err != nil {
		return uuid.Nil, utils.NewError(utils.ErrCodeUnauthorized, "Invalid user UUID format")
	}

	return userUUID, nil
}

// GetUserFromContext extracts user_uuid from gin.Context and queries the User entity from repository.
// Returns an Unauthorized error on invalid/missing context UUID, or NotFound if the user doesn't exist.
func GetUserFromContext(ctx *gin.Context, userRepo repository.UserRepository) (sqlc.User, error) {
	userUUID, err := GetUserUUIDFromContext(ctx)
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
