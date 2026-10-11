package v1handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// InternalAddressHandler provides address endpoints for inter-service communication (REST IPC)
type InternalAddressHandler struct {
	addressRepo repository.AddressRepository
	redis       *redis.Client
}

func NewInternalAddressHandler(addressRepo repository.AddressRepository, redisClient *redis.Client) *InternalAddressHandler {
	return &InternalAddressHandler{
		addressRepo: addressRepo,
		redis:       redisClient,
	}
}

// GetAddressesByUserID handles GET /internal/v1/users/:id/addresses
func (h *InternalAddressHandler) GetAddressesByUserID(c *gin.Context) {
	idStr := c.Param("id")
	userID, err := strconv.Atoi(idStr)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	cacheKey := fmt.Sprintf("user:addresses:%d", userID)
	if h.redis != nil {
		if val, err := h.redis.Get(c.Request.Context(), cacheKey).Result(); err == nil && val != "" {
			var dtos []v1dto.AddressDTO
			if err := json.Unmarshal([]byte(val), &dtos); err == nil {
				c.JSON(http.StatusOK, gin.H{"addresses": dtos})
				return
			}
		}
	}

	rows, err := h.addressRepo.GetByUserID(c.Request.Context(), int32(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	dtos := make([]v1dto.AddressDTO, len(rows))
	for i, row := range rows {
		dtos[i] = v1dto.MapToAddressDTO(row)
	}

	if h.redis != nil {
		if data, err := json.Marshal(dtos); err == nil {
			_ = h.redis.Set(c.Request.Context(), cacheKey, data, 15*time.Minute).Err()
		}
	}

	c.JSON(http.StatusOK, gin.H{"addresses": dtos})
}

// GetAddressByUUID handles GET /internal/v1/addresses/:uuid
func (h *InternalAddressHandler) GetAddressByUUID(c *gin.Context) {
	uuidStr := c.Param("uuid")
	addrUUID, err := uuid.Parse(uuidStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid address uuid"})
		return
	}

	userIDStr := c.Query("user_id")
	userID, _ := strconv.Atoi(userIDStr)

	cacheKey := fmt.Sprintf("user:address:%s", addrUUID.String())
	if h.redis != nil {
		if val, err := h.redis.Get(c.Request.Context(), cacheKey).Result(); err == nil && val != "" {
			var dto v1dto.AddressDTO
			if err := json.Unmarshal([]byte(val), &dto); err == nil {
				c.JSON(http.StatusOK, dto)
				return
			}
		}
	}

	row, err := h.addressRepo.GetByUUID(c.Request.Context(), int32(userID), addrUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "address not found"})
		return
	}

	dto := v1dto.MapToAddressDTO(row)

	if h.redis != nil {
		if data, err := json.Marshal(dto); err == nil {
			_ = h.redis.Set(c.Request.Context(), cacheKey, data, 30*time.Minute).Err()
		}
	}

	c.JSON(http.StatusOK, dto)
}
