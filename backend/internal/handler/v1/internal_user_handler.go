package v1handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// InternalUserHandler provides endpoints for inter-service communication (REST IPC)
type InternalUserHandler struct {
	userRepo repository.UserRepository
	redis    *redis.Client
}

func NewInternalUserHandler(userRepo repository.UserRepository, redisClient *redis.Client) *InternalUserHandler {
	return &InternalUserHandler{
		userRepo: userRepo,
		redis:    redisClient,
	}
}

// GetBatchUsers handles POST /internal/v1/users/batch
func (h *InternalUserHandler) GetBatchUsers(c *gin.Context) {
	var req v1dto.InternalBatchUsersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.UserIDs) == 0 {
		c.JSON(http.StatusOK, v1dto.InternalBatchUsersResponse{
			Users: make(map[int32]v1dto.InternalUserDTO),
		})
		return
	}

	rows, err := h.userRepo.GetByIDs(c.Request.Context(), req.UserIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	res := make(map[int32]v1dto.InternalUserDTO, len(rows))
	for _, row := range rows {
		dto := v1dto.InternalUserDTO{
			UserID:        row.UserID,
			UserUUID:      row.UserUuid,
			UserFullname:  row.UserFullname,
			UserEmail:     row.UserEmail,
			UserAvatar:    row.UserAvatar,
			UserAvatarUrl: row.UserAvatarUrl,
			UserStatus:    row.UserStatus,
			UserLevel:     row.UserLevel,
		}
		res[row.UserID] = dto

		// Cache to Redis (TTL 1 hour)
		if h.redis != nil {
			if data, err := json.Marshal(dto); err != nil {
				logger.Log.Warn().Err(err).Int32("user_id", dto.UserID).Msg("Failed to marshal user dto for caching")
			} else if err := h.redis.Set(c.Request.Context(), fmt.Sprintf("user:profile:%d", dto.UserID), data, 1*time.Hour).Err(); err != nil {
				logger.Log.Warn().Err(err).Int32("user_id", dto.UserID).Msg("Failed to cache user profile in redis")
			}
		}
	}

	c.JSON(http.StatusOK, v1dto.InternalBatchUsersResponse{
		Users: res,
	})
}

// GetUserByID handles GET /internal/v1/users/:id
func (h *InternalUserHandler) GetUserByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), int32(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	dto := v1dto.InternalUserDTO{
		UserID:        user.UserID,
		UserUUID:      user.UserUuid,
		UserFullname:  user.UserFullname,
		UserEmail:     user.UserEmail,
		UserAvatar:    user.UserAvatar,
		UserAvatarUrl: user.UserAvatarUrl,
		UserStatus:    user.UserStatus,
		UserLevel:     user.UserLevel,
	}

	// Cache to Redis (TTL 1 hour)
	if h.redis != nil {
		if data, err := json.Marshal(dto); err != nil {
			logger.Log.Warn().Err(err).Int32("user_id", dto.UserID).Msg("Failed to marshal user dto for caching")
		} else if err := h.redis.Set(c.Request.Context(), fmt.Sprintf("user:profile:%d", dto.UserID), data, 1*time.Hour).Err(); err != nil {
			logger.Log.Warn().Err(err).Int32("user_id", dto.UserID).Msg("Failed to cache user profile in redis")
		}
	}

	c.JSON(http.StatusOK, dto)
}
