package v1service

import (
	"fmt"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type cartService struct {
	repo     repository.CartRepository
	userRepo repository.UserRepository
	cache    cache.RedisCacheService
}

func NewCartService(repo repository.CartRepository, userRepo repository.UserRepository, redisClient *redis.Client) CartService {
	return &cartService{
		repo:     repo,
		userRepo: userRepo,
		cache:    cache.NewRedisCacheService(redisClient),
	}
}

func (cs *cartService) GetMyCart(ctx *gin.Context) (v1dto.CartDTO, error) {

	reqCtx := ctx.Request.Context()
	userUUIDStr, exists := ctx.Get("user_uuid")
	if !exists {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeNotFound, "user not found")
	}

	cacheKey := fmt.Sprintf("cart:user:%s", userUUIDStr)

	var cached v1dto.CartDTO
	if err := cs.cache.Get(cacheKey, &cached); err == nil {
		return cached, nil
	}

	userUUID, err := uuid.Parse(userUUIDStr.(string))
	if err != nil {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeBadRequest, "invalid user UUID")
	}

	user, err := cs.userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeNotFound, "user not found")
	}

	cart, err := cs.repo.GetOrCreateCart(reqCtx, user.UserID)
	if err != nil {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeInternal, "failed to retrieve or create cart")
	}

	row, err := cs.repo.GetCartItems(reqCtx, cart.CartID)
	if err != nil {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeInternal, "failed to retrieve cart items")
	}

	cartDTO := v1dto.MapCartItemsToDTO(cart.CartUuid, row)

	if err := cs.cache.Set(cacheKey, cartDTO, 5*time.Minute); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to set cart cache")
	}

	return cartDTO, nil
}
