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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type cartService struct {
	repo        repository.CartRepository
	userRepo    repository.UserRepository
	productRepo repository.ProductRepository
	cache       cache.RedisCacheService
}

func NewCartService(repo repository.CartRepository, userRepo repository.UserRepository, productRepo repository.ProductRepository, redisClient *redis.Client) CartService {
	return &cartService{
		repo:        repo,
		userRepo:    userRepo,
		productRepo: productRepo,
		cache:       cache.NewRedisCacheService(redisClient),
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

func (cs *cartService) AddItemToCart(ctx *gin.Context, input v1dto.AddCartItemInput) (v1dto.CartDTO, error) {
	reqCtx := ctx.Request.Context()

	userUUIDStr, exists := ctx.Get("user_uuid")
	if !exists || userUUIDStr == "" {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeUnauthorized, "unauthorized")
	}

	userUUID, err := uuid.Parse(userUUIDStr.(string))
	if err != nil {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeUnauthorized, "invalid user UUID format")
	}

	user, err := cs.userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeNotFound, "user not found")
	}

	productUUID, err := uuid.Parse(input.ProductUUID)
	if err != nil {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeBadRequest, "invalid product UUID")
	}

	product, err := cs.productRepo.GetByUUID(reqCtx, productUUID)
	if err != nil {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeNotFound, "product not found")
	}

	if product.ProductStatus != ProductStatusActive {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeBadRequest, "product is unavailable for purchase")
	}

	cart, err := cs.repo.GetOrCreateCart(reqCtx, user.UserID)
	if err != nil {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeInternal, "failed to retrieve or create cart")
	}

	// Soft check stock
	existingQty, _ := cs.repo.GetCartItemQuantity(reqCtx, cart.CartID, product.ProductID)
	totalQty := existingQty + input.Quantity

	if product.ProductStock != nil && *product.ProductStock < totalQty {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("insufficient stock: available %d, requested total %d", *product.ProductStock, totalQty))
	}

	_, err = cs.repo.AddOrUpdateItem(reqCtx, sqlc.AddOrUpdateCartItemParams{
		CartID:    cart.CartID,
		ProductID: product.ProductID,
		Quantity:  input.Quantity,
	})
	if err != nil {
		return v1dto.CartDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to add item to cart", err)
	}

	cacheKey := fmt.Sprintf("cart:user:%s", userUUIDStr)
	if err := cs.cache.Clear(cacheKey); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear cart cache")
	}

	return cs.GetMyCart(ctx)
}

func (cs *cartService) UpdateCartItemQuantity(ctx *gin.Context, itemUUID uuid.UUID, input v1dto.UpdateCartItemQuantityInput) (v1dto.CartDTO, error) {
	reqCtx := ctx.Request.Context()

	userUUIDStr, exists := ctx.Get("user_uuid")
	if !exists || userUUIDStr == "" {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeUnauthorized, "unauthorized")
	}

	userUUID, err := uuid.Parse(userUUIDStr.(string))
	if err != nil {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeUnauthorized, "invalid user UUID format")
	}

	user, err := cs.userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeNotFound, "user not found")
	}

	cart, err := cs.repo.GetOrCreateCart(reqCtx, user.UserID)
	if err != nil {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeInternal, "failed to retrieve or create cart")
	}

	if input.Quantity == nil || *input.Quantity <= 0 {
		return cs.RemoveItemFromCart(ctx, itemUUID)
	}

	newQty := *input.Quantity

	items, err := cs.repo.GetCartItems(reqCtx, cart.CartID)
	if err != nil {
		return v1dto.CartDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to retrieve cart items", err)
	}

	var targetItem *sqlc.GetCartItemsWithProductDetailsRow
	for _, item := range items {
		if item.CartItemUuid == itemUUID {
			targetItem = &item
			break
		}
	}

	if targetItem == nil {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeNotFound, "cart item not found")
	}

	if targetItem.ProductStatus != ProductStatusActive {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeBadRequest, "product is unavailable for purchase")
	}

	if targetItem.ProductStock != nil && *targetItem.ProductStock < newQty {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("insufficient stock: available %d, requested %d", *targetItem.ProductStock, newQty))
	}

	_, err = cs.repo.SetCartItemQuantity(reqCtx, sqlc.SetCartItemQuantityParams{
		Quantity:     newQty,
		CartItemUuid: itemUUID,
		CartID:       cart.CartID,
	})
	if err != nil {
		return v1dto.CartDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to update cart item quantity", err)
	}

	cacheKey := fmt.Sprintf("cart:user:%s", userUUIDStr)
	if err := cs.cache.Clear(cacheKey); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear cart cache")
	}

	return cs.GetMyCart(ctx)
}

func (cs *cartService) RemoveItemFromCart(ctx *gin.Context, itemUUID uuid.UUID) (v1dto.CartDTO, error) {
	reqCtx := ctx.Request.Context()

	userUUIDStr, exists := ctx.Get("user_uuid")
	if !exists || userUUIDStr == "" {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeUnauthorized, "unauthorized")
	}

	userUUID, err := uuid.Parse(userUUIDStr.(string))
	if err != nil {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeUnauthorized, "invalid user UUID format")
	}

	user, err := cs.userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeNotFound, "user not found")
	}

	cart, err := cs.repo.GetOrCreateCart(reqCtx, user.UserID)
	if err != nil {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeInternal, "failed to retrieve or create cart")
	}

	err = cs.repo.DeleteItem(reqCtx, cart.CartID, itemUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeNotFound, "cart item not found")
		}
		return v1dto.CartDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to remove item from cart", err)
	}

	cacheKey := fmt.Sprintf("cart:user:%s", userUUIDStr)
	if err := cs.cache.Clear(cacheKey); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear cart cache")
	}

	return cs.GetMyCart(ctx)
}
