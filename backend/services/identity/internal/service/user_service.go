package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/utils"
	"goshop/services/identity/internal/db/sqlc"
	dto "goshop/services/identity/internal/dto"
	"goshop/services/identity/internal/repository"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type userService struct {
	repo     repository.UserRepository
	cache    cache.RedisCacheService
	redis    *redis.Client
	rabbitmq rabbitmq.RabbitMQService
	outbox   repository.OutboxRepository
}

func NewUserService(repo repository.UserRepository, redisClient *redis.Client, rmq ...rabbitmq.RabbitMQService) UserService {
	var cacheService cache.RedisCacheService
	if redisClient != nil {
		cacheService = cache.NewRedisCacheService(redisClient)
	}
	var rabbitMQ rabbitmq.RabbitMQService
	if len(rmq) > 0 {
		rabbitMQ = rmq[0]
	}
	return &userService{
		repo:     repo,
		cache:    cacheService,
		redis:    redisClient,
		rabbitmq: rabbitMQ,
	}
}

func NewUserServiceWithOutbox(repo repository.UserRepository, redisClient *redis.Client, rmq rabbitmq.RabbitMQService, outbox repository.OutboxRepository) UserService {
	var cacheService cache.RedisCacheService
	if redisClient != nil {
		cacheService = cache.NewRedisCacheService(redisClient)
	}
	return &userService{
		repo:     repo,
		cache:    cacheService,
		redis:    redisClient,
		rabbitmq: rmq,
		outbox:   outbox,
	}
}

func (us *userService) publishUserSyncEvent(ctx context.Context, eventType string, user sqlc.User) {
	// 1. Invalidate Redis single profile cache (by ID and UUID)
	if us.redis != nil {
		if user.UserID > 0 {
			_ = us.redis.Del(ctx, fmt.Sprintf("user:profile:%d", user.UserID)).Err()
		}
		if user.UserUuid != uuid.Nil {
			_ = us.redis.Del(ctx, fmt.Sprintf("user:profile:uuid:%s", user.UserUuid.String())).Err()
		}
	}

	event := rabbitmq.NewUserSyncEvent(
		eventType,
		user.UserID,
		user.UserUuid.String(),
		user.UserFullname,
		user.UserEmail,
		user.UserAvatar,
		user.UserAvatarUrl,
		user.UserStatus,
		user.UserLevel,
	)

	var queueName string
	switch eventType {
	case rabbitmq.QueueUserCreated:
		queueName = rabbitmq.QueueUserCreated
	case rabbitmq.QueueUserDeleted:
		queueName = rabbitmq.QueueUserDeleted
	default:
		queueName = rabbitmq.QueueUserUpdated
	}

	// 2. Transactional Outbox Pattern: Save event to outbox table first for guaranteed delivery
	var outboxEventID int64
	if us.outbox != nil && user.UserID > 0 {
		outboxRecord, err := us.outbox.Insert(ctx, nil, "user", fmt.Sprintf("%d", user.UserID), queueName, event)
		if err != nil {
			logger.Log.Error().Err(err).Int32("user_id", user.UserID).Msg("Failed to persist user sync event to outbox")
		} else {
			outboxEventID = outboxRecord.EventID
		}
	}

	// 3. Fast-path publish to RabbitMQ if available
	if us.rabbitmq != nil {
		if err := us.rabbitmq.Publish(ctx, queueName, event); err != nil {
			logger.Log.Warn().Err(err).Str("queue", queueName).Int32("user_id", user.UserID).Msg("Fast-path RabbitMQ publish failed, will be retried by Outbox Worker")
		} else {
			logger.Log.Info().Str("queue", queueName).Int32("user_id", user.UserID).Msg("Published user sync event to RabbitMQ")
			if us.outbox != nil && outboxEventID > 0 {
				_ = us.outbox.MarkProcessed(ctx, []int64{outboxEventID})
			}
		}
	}
}

func (us *userService) GetAllUsers(ctx context.Context, search, orderBy, sort string, page, limit int32, deleted bool) ([]dto.UserDTO, int32, error) {
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

	cacheKey := us.generateCacheKey(search, orderBy, sort, page, limit, deleted)
	var cacheData struct {
		Users []dto.UserDTO `json:"users"`
		Total int32         `json:"total"`
	}

	if us.cache != nil {
		if err := us.cache.Get(cacheKey, &cacheData); err == nil && cacheData.Users != nil {
			return cacheData.Users, cacheData.Total, nil
		}
	}

	listUsers, err := us.repo.GetAllV2(ctx, search, orderBy, sort, limit, offset, deleted)
	if err != nil {
		return []dto.UserDTO{}, 0, utils.WrapError(utils.ErrCodeInternal, "failed to fetch user list", err)
	}

	total, err := us.repo.CountUsers(ctx, search, deleted)
	if err != nil {
		return []dto.UserDTO{}, 0, utils.WrapError(utils.ErrCodeInternal, "failed to count users", err)
	}

	usersDTO := make([]dto.UserDTO, len(listUsers))
	for i, u := range listUsers {
		usersDTO[i] = *mapIdentityUserToDTO(u)
	}

	if us.cache != nil {
		cacheData = struct {
			Users []dto.UserDTO `json:"users"`
			Total int32         `json:"total"`
		}{
			Users: usersDTO,
			Total: int32(total),
		}
		if err := us.cache.Set(cacheKey, cacheData, 5*time.Minute); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to cache users")
		}
	}

	return usersDTO, int32(total), nil
}

func (us *userService) CreateUsers(ctx context.Context, input dto.CreateUserInput) (*dto.UserDTO, error) {
	var userPassword *string
	if input.Password != "" {
		userPassword = &input.Password
	}

	createParams := sqlc.CreateUserParams{
		UserFullname: input.Name,
		UserEmail:    utils.NormalizeString(input.Email),
		UserPassword: userPassword,
		UserAge:      utils.ConvertToInt32Pointer(input.Age),
		UserStatus:   input.Status,
		UserLevel:    input.Level,
	}

	if createParams.UserPassword != nil && *createParams.UserPassword != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*createParams.UserPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "failed to hash password", err)
		}
		hashed := string(hashedPassword)
		createParams.UserPassword = &hashed
	}

	user, err := us.repo.Create(ctx, createParams)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, utils.NewError(utils.ErrCodeConflict, "email already exists")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to create user", err)
	}

	if us.cache != nil {
		if err := us.cache.Clear("users:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear cache")
		}
	}

	us.publishUserSyncEvent(ctx, rabbitmq.QueueUserCreated, user)

	return mapIdentityUserToDTO(user), nil
}

func (us *userService) GetUserByUUID(ctx context.Context, userUUID uuid.UUID) (*dto.UserDTO, error) {
	user, err := us.repo.GetByUUID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "user not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get user", err)
	}

	return mapIdentityUserToDTO(user), nil
}

func (us *userService) UpdateUser(ctx context.Context, userUUID uuid.UUID, input dto.UpdateUserInput) (*dto.UserDTO, error) {
	updateParams := sqlc.UpdateUserParams{
		UserUuid:     userUUID,
		UserFullname: input.Name,
		UserPassword: input.Password,
		UserAge:      input.Age,
		UserStatus:   input.Status,
		UserLevel:    input.Level,
	}
	if updateParams.UserPassword != nil && *updateParams.UserPassword != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*updateParams.UserPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "failed to hash password", err)
		}
		hashed := string(hashedPassword)
		updateParams.UserPassword = &hashed
	}

	updatedUser, err := us.repo.Update(ctx, updateParams)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "user not found")
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, utils.NewError(utils.ErrCodeConflict, "email already exists")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to update user", err)
	}

	if us.cache != nil {
		if err := us.cache.Clear("users:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear cache")
		}
	}

	us.publishUserSyncEvent(ctx, rabbitmq.QueueUserUpdated, updatedUser)

	return mapIdentityUserToDTO(updatedUser), nil
}

func (us *userService) GetProfile(ctx context.Context, userUUID uuid.UUID) (*dto.UserDTO, error) {
	user, err := us.repo.GetByUUID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "user not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get user profile", err)
	}
	return mapIdentityUserToDTO(user), nil
}

func (us *userService) UpdateProfile(ctx context.Context, userUUID uuid.UUID, input dto.UpdateProfileInput) (*dto.UserDTO, error) {
	updateParams := sqlc.UpdateUserParams{
		UserUuid:     userUUID,
		UserFullname: input.Name,
		UserPassword: input.Password,
		UserAge:      input.Age,
	}

	if updateParams.UserPassword != nil && *updateParams.UserPassword != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*updateParams.UserPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "failed to hash password", err)
		}
		hashed := string(hashedPassword)
		updateParams.UserPassword = &hashed
	}

	user, err := us.repo.Update(ctx, updateParams)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "user not found")
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, utils.NewError(utils.ErrCodeConflict, "email already exists")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to update profile", err)
	}

	if us.cache != nil {
		if err := us.cache.Clear("users:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear cache")
		}
	}

	us.publishUserSyncEvent(ctx, rabbitmq.QueueUserUpdated, user)

	return mapIdentityUserToDTO(user), nil
}

func (us *userService) ChangePassword(ctx context.Context, userUUID uuid.UUID, input dto.ChangePasswordRequest) error {
	user, err := us.repo.GetByUUID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "user not found")
		}
		return utils.WrapError(utils.ErrCodeInternal, "failed to get user", err)
	}

	if user.UserPassword == nil || *user.UserPassword == "" {
		return utils.NewError(utils.ErrCodeBadRequest, "user has no password set (e.g. social login)")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.UserPassword), []byte(input.OldPassword)); err != nil {
		return utils.NewError(utils.ErrCodeUnauthorized, "old password is incorrect")
	}

	if input.NewPassword == input.OldPassword {
		return utils.NewError(utils.ErrCodeBadRequest, "new password cannot be the same as the old password")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "failed to hash new password", err)
	}

	hashed := string(hashedPassword)
	updateParams := sqlc.UpdatePasswordParams{
		UserUuid:    user.UserUuid,
		NewPassword: &hashed,
	}
	if _, err := us.repo.UpdatePassword(ctx, updateParams); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "failed to update password", err)
	}

	if us.cache != nil {
		if err := us.cache.Clear(fmt.Sprintf("user:%s", user.UserUuid.String())); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear user cache")
		}
		if err := us.cache.Clear("users:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear users cache")
		}
	}
	logger.Log.Info().Str("user_uuid", user.UserUuid.String()).Msg("User password changed successfully")
	return nil
}

func (us *userService) SoftDeleteUser(ctx context.Context, userUUID uuid.UUID) (*dto.UserDTO, error) {
	user, err := us.repo.SoftDelete(ctx, userUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "user not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to soft delete user", err)
	}

	if us.cache != nil {
		if err := us.cache.Clear("users:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear cache")
		}
	}

	us.publishUserSyncEvent(ctx, rabbitmq.QueueUserDeleted, user)

	return mapIdentityUserToDTO(user), nil
}

func (us *userService) RestoreUser(ctx context.Context, userUUID uuid.UUID) (*dto.UserDTO, error) {
	user, err := us.repo.Restore(ctx, userUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "user not found or not soft deleted for restoration")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to restore user", err)
	}

	if us.cache != nil {
		if err := us.cache.Clear("users:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear cache")
		}
	}

	us.publishUserSyncEvent(ctx, rabbitmq.QueueUserUpdated, user)

	return mapIdentityUserToDTO(user), nil
}

func (us *userService) TrashUser(ctx context.Context, userUUID uuid.UUID) error {
	trashedUser, err := us.repo.Trash(ctx, userUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "user not found or not soft deleted for permanent deletion")
		}
		return utils.WrapError(utils.ErrCodeInternal, "failed to trash user", err)
	}

	if us.cache != nil {
		if err := us.cache.Clear("users:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear cache")
		}
	}

	if trashedUser.UserID > 0 {
		us.publishUserSyncEvent(ctx, rabbitmq.QueueUserDeleted, trashedUser)
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

func mapStatusText(status int) string {
	switch status {
	case 1:
		return "Active"
	case 2:
		return "Inactive"
	case 3:
		return "Banned"
	default:
		return "None"
	}
}

func mapLevelText(level int) string {
	switch level {
	case 1:
		return "Admin"
	case 2:
		return "Moderator"
	case 3:
		return "Member"
	default:
		return "None"
	}
}

func mapIdentityUserToDTO(u sqlc.User) *dto.UserDTO {
	dto := &dto.UserDTO{
		UUID:      u.UserUuid.String(),
		Name:      u.UserFullname,
		Email:     u.UserEmail,
		Status:    mapStatusText(int(u.UserStatus)),
		Level:     mapLevelText(int(u.UserLevel)),
		CreatedAt: u.UserCreatedAt.Format("2006-01-02 15:04:05"),
	}
	if u.UserAge != nil {
		age := int(*u.UserAge)
		dto.Age = &age
	}
	return dto
}
