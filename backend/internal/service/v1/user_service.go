package v1service

import (
	"database/sql"
	"errors"
	"fmt"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type userService struct {
	repo  repository.UserRepository
	cache cache.RedisCacheService
}

func NewUserService(repo repository.UserRepository, redisClient *redis.Client) UserService {
	return &userService{
		repo:  repo,
		cache: cache.NewRedisCacheService(redisClient),
	}
}

func (us *userService) GetAllUsers(ctx *gin.Context, search, orderBy, sort string, page, limit int32, deleted bool) ([]sqlc.User, int32, error) {

	context := ctx.Request.Context()

	if sort == "" {
		sort = "desc"
	}

	if orderBy == "" {
		orderBy = "user_created_at"
	}

	if page <= 0 {
		page = 1
	}

	if limit <= 0 {
		limitInt := utils.GetIntEnv("DEFAULT_LIMIT", 10)
		limit = int32(limitInt)
	}

	offset := (page - 1) * limit

	/** Cache Redis **/
	cacheKey := us.generateCacheKey(search, orderBy, sort, page, limit, deleted)
	var cacheData struct {
		Users []sqlc.User `json:"users"`
		Total int32       `json:"total"`
	}

	if err := us.cache.Get(cacheKey, &cacheData); err == nil && cacheData.Users != nil {
		return cacheData.Users, cacheData.Total, nil
	}

	listUsers, err := us.repo.GetAllV2(context, search, orderBy, sort, limit, offset, deleted)
	if err != nil {
		return []sqlc.User{}, 0, utils.WrapError(utils.ErrCodeInternal, "failed to fetch user list", err)
	}

	total, err := us.repo.CountUsers(context, search, deleted)
	if err != nil {
		return []sqlc.User{}, 0, utils.WrapError(utils.ErrCodeInternal, "failed to count users", err)
	}

	// Create cache data
	cacheData = struct {
		Users []sqlc.User `json:"users"`
		Total int32       `json:"total"`
	}{
		Users: listUsers,
		Total: int32(total),
	}
	us.cache.Set(cacheKey, cacheData, 5*time.Minute)

	return listUsers, int32(total), nil
}

func (us *userService) CreateUsers(ctx *gin.Context, input sqlc.CreateUserParams) (sqlc.User, error) {

	context := ctx.Request.Context()

	input.UserEmail = utils.NormalizeString(input.UserEmail)

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.UserPassword), bcrypt.DefaultCost)
	if err != nil {
		return sqlc.User{}, utils.WrapError(utils.ErrCodeInternal, "failed to hash password", err)
	}
	input.UserPassword = string(hashedPassword)

	user, err := us.repo.Create(context, input)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return sqlc.User{}, utils.NewError(utils.ErrCodeConflict, "email already exists")
		}
		return sqlc.User{}, utils.WrapError(utils.ErrCodeInternal, "failed to create user", err)
	}

	// Clear cache redis
	if err := us.cache.Clear("users:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear cache")
	}

	return user, nil
}

func (us *userService) GetUserByUUID(ctx *gin.Context, uuid uuid.UUID) (sqlc.User, error) {

	context := ctx.Request.Context()

	user, err := us.repo.GetByUUID(context, uuid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlc.User{}, utils.NewError(utils.ErrCodeNotFound, "user not found")
		}
		return sqlc.User{}, utils.WrapError(utils.ErrCodeInternal, "failed to get user", err)
	}

	return user, nil
}

func (us *userService) UpdateUser(ctx *gin.Context, input sqlc.UpdateUserParams) (sqlc.User, error) {
	context := ctx.Request.Context()

	if input.UserPassword != nil && *input.UserPassword != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*input.UserPassword), bcrypt.DefaultCost)
		if err != nil {
			return sqlc.User{}, utils.WrapError(utils.ErrCodeInternal, "failed to hash password", err)
		}
		hashed := string(hashedPassword)
		input.UserPassword = &hashed
	}

	updatedUser, err := us.repo.Update(context, input)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlc.User{}, utils.NewError(utils.ErrCodeNotFound, "user not found")
		}
		return sqlc.User{}, utils.WrapError(utils.ErrCodeInternal, "failed to update user", err)
	}

	// Clear cache redis
	if err := us.cache.Clear("users:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear cache")
	}

	return updatedUser, nil
}

func (us *userService) GetProfile(ctx *gin.Context) (sqlc.User, error) {
	return GetUserFromContext(ctx, us.repo)
}

func (us *userService) UpdateProfile(ctx *gin.Context, input sqlc.UpdateUserParams) (sqlc.User, error) {
	context := ctx.Request.Context()

	userUuid, err := GetUserUUIDFromContext(ctx)
	if err != nil {
		return sqlc.User{}, err
	}

	input.UserUuid = userUuid
	input.UserStatus = nil
	input.UserLevel = nil

	if input.UserPassword != nil && *input.UserPassword != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*input.UserPassword), bcrypt.DefaultCost)
		if err != nil {
			return sqlc.User{}, utils.WrapError(utils.ErrCodeInternal, "failed to hash password", err)
		}
		hashed := string(hashedPassword)
		input.UserPassword = &hashed
	}

	user, err := us.repo.Update(context, input)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlc.User{}, utils.NewError(utils.ErrCodeNotFound, "user not found")
		}
		return sqlc.User{}, utils.WrapError(utils.ErrCodeInternal, "failed to update profile", err)
	}

	if err := us.cache.Clear("users:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear cache")
	}

	return user, nil
}

func (us *userService) ChangePassword(ctx *gin.Context, input v1dto.ChangePasswordRequest) error {
	user, err := GetUserFromContext(ctx, us.repo)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.UserPassword), []byte(input.OldPassword)); err != nil {
		return utils.NewError(utils.ErrCodeUnauthorized, "old password is incorrect")
	}

	if input.NewPassword == input.OldPassword {
		return utils.NewError(utils.ErrCodeBadRequest, "new password cannot be the same as the old password")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "failed to hash new password", err)
	}

	reqCtx := ctx.Request.Context()
	updateParams := sqlc.UpdatePasswordParams{
		UserUuid:    user.UserUuid,
		NewPassword: string(hashedPassword),
	}
	if _, err := us.repo.UpdatePassword(reqCtx, updateParams); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "failed to update password", err)
	}

	_ = us.cache.Clear(fmt.Sprintf("user:%s", user.UserUuid.String()))
	_ = us.cache.Clear("users:*")
	logger.Log.Info().Str("user_uuid", user.UserUuid.String()).Msg("User password changed successfully")
	return nil
}

func (us *userService) SoftDeleteUser(ctx *gin.Context, uuid uuid.UUID) (sqlc.User, error) {
	context := ctx.Request.Context()

	user, err := us.repo.SoftDelete(context, uuid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlc.User{}, utils.NewError(utils.ErrCodeNotFound, "user not found")
		}
		return sqlc.User{}, utils.WrapError(utils.ErrCodeInternal, "failed to soft delete user", err)
	}

	// Clear cache redis
	if err := us.cache.Clear("users:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear cache")
	}

	return user, nil
}

func (us *userService) RestoreUser(ctx *gin.Context, uuid uuid.UUID) (sqlc.User, error) {
	context := ctx.Request.Context()

	user, err := us.repo.Restore(context, uuid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlc.User{}, utils.NewError(utils.ErrCodeNotFound, "user not found or not soft deleted for restoration")
		}
		return sqlc.User{}, utils.WrapError(utils.ErrCodeInternal, "failed to restore user", err)
	}

	// Clear cache redis
	if err := us.cache.Clear("users:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear cache")
	}

	return user, nil
}

func (us *userService) TrashUser(ctx *gin.Context, uuid uuid.UUID) error {
	context := ctx.Request.Context()

	_, err := us.repo.Trash(context, uuid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "user not found or not soft deleted for permanent deletion")
		}
		return utils.WrapError(utils.ErrCodeInternal, "failed to trash user", err)
	}

	// Clear cache redis
	if err := us.cache.Clear("users:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear cache")
	}

	return nil
}

func (us *userService) generateCacheKey(search, orderBy, sort string, page, limit int32, deleted bool) string {
	search = strings.TrimSpace(search)
	if search == "" {
		search = "none"
	}

	orderBy = strings.TrimSpace(orderBy)
	if orderBy == "" {
		orderBy = "user_created_at"
	}

	sort = strings.ToLower(strings.TrimSpace(sort))
	if sort == "" {
		sort = "desc"
	}
	return fmt.Sprintf("users:%s:%s:%s:%d:%d:%t", search, orderBy, sort, page, limit, deleted)
}
