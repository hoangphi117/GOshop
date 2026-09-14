package v1service

import (
	"context"
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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

type cartService struct {
	repo        repository.CartRepository
	userRepo    repository.UserRepository
	productRepo repository.ProductRepository
	cache       cache.RedisCacheService
}

func NewCartService(repo repository.CartRepository, userRepo repository.UserRepository, productRepo repository.ProductRepository, redisClient *redis.Client) CartService {
	var cacheService cache.RedisCacheService
	if redisClient != nil {
		cacheService = cache.NewRedisCacheService(redisClient)
	}
	return &cartService{
		repo:        repo,
		userRepo:    userRepo,
		productRepo: productRepo,
		cache:       cacheService,
	}
}

// -------------------------------------------------------------------------
// Single Responsibility Helpers
// -------------------------------------------------------------------------

func (cs *cartService) getUser(ctx context.Context, userUUID uuid.UUID) (sqlc.User, error) {
	user, err := cs.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.User{}, utils.NewError(utils.ErrCodeNotFound, "user not found")
		}
		return sqlc.User{}, utils.WrapError(utils.ErrCodeInternal, "failed to get user", err)
	}
	return user, nil
}

func (cs *cartService) getOrCreateCart(ctx context.Context, userID int32) (sqlc.Cart, error) {
	cart, err := cs.repo.GetOrCreateCart(ctx, userID)
	if err != nil {
		return sqlc.Cart{}, utils.WrapError(utils.ErrCodeInternal, "failed to retrieve or create cart", err)
	}
	return cart, nil
}

func (cs *cartService) loadCartDTO(ctx context.Context, userUUID uuid.UUID, cartID int32, cartUUID uuid.UUID) (v1dto.CartDTO, error) {
	rows, err := cs.repo.GetCartItems(ctx, cartID)
	if err != nil {
		return v1dto.CartDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to retrieve cart items", err)
	}

	cartDTO := v1dto.MapCartItemsToDTO(cartUUID, rows)

	if cs.cache != nil {
		cacheKey := fmt.Sprintf("cart:user:%s", userUUID.String())
		if err := cs.cache.Set(cacheKey, cartDTO, 5*time.Minute); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to set cart cache")
		}
	}

	return cartDTO, nil
}

func (cs *cartService) invalidateCartCache(userUUID uuid.UUID) {
	if cs.cache != nil {
		cacheKey := fmt.Sprintf("cart:user:%s", userUUID.String())
		if err := cs.cache.Clear(cacheKey); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear cart cache")
		}
	}
}

func (cs *cartService) validateProductForCart(ctx context.Context, productUUID uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
	product, err := cs.productRepo.GetByUUID(ctx, productUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.GetProductByUUIDRow{}, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return sqlc.GetProductByUUIDRow{}, utils.WrapError(utils.ErrCodeInternal, "failed to get product", err)
	}

	if product.ProductStatus != ProductStatusActive {
		return sqlc.GetProductByUUIDRow{}, utils.NewError(utils.ErrCodeBadRequest, "product is unavailable for purchase")
	}

	return product, nil
}

// -------------------------------------------------------------------------
// CartService Implementation
// -------------------------------------------------------------------------

func (cs *cartService) GetMyCart(ctx context.Context, userUUID uuid.UUID) (v1dto.CartDTO, error) {
	cacheKey := fmt.Sprintf("cart:user:%s", userUUID.String())

	if cs.cache != nil {
		var cached v1dto.CartDTO
		if err := cs.cache.Get(cacheKey, &cached); err == nil && cached.UUID != "" {
			return cached, nil
		}
	}

	user, err := cs.getUser(ctx, userUUID)
	if err != nil {
		return v1dto.CartDTO{}, err
	}

	cart, err := cs.getOrCreateCart(ctx, user.UserID)
	if err != nil {
		return v1dto.CartDTO{}, err
	}

	return cs.loadCartDTO(ctx, userUUID, cart.CartID, cart.CartUuid)
}

func (cs *cartService) AddItemToCart(ctx context.Context, userUUID uuid.UUID, input v1dto.AddCartItemInput) (v1dto.CartDTO, error) {
	productUUID, err := uuid.Parse(input.ProductUUID)
	if err != nil {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeBadRequest, "invalid product UUID")
	}

	user, err := cs.getUser(ctx, userUUID)
	if err != nil {
		return v1dto.CartDTO{}, err
	}

	cart, err := cs.getOrCreateCart(ctx, user.UserID)
	if err != nil {
		return v1dto.CartDTO{}, err
	}

	product, err := cs.validateProductForCart(ctx, productUUID)
	if err != nil {
		return v1dto.CartDTO{}, err
	}

	existingQty, _ := cs.repo.GetCartItemQuantity(ctx, cart.CartID, product.ProductID)
	totalQty := existingQty + input.Quantity
	if product.ProductStock != nil && *product.ProductStock < totalQty {
		return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("insufficient stock: available %d, requested total %d", *product.ProductStock, totalQty))
	}

	_, err = cs.repo.AddOrUpdateItem(ctx, sqlc.AddOrUpdateCartItemParams{
		CartID:    cart.CartID,
		ProductID: product.ProductID,
		Quantity:  input.Quantity,
	})
	if err != nil {
		return v1dto.CartDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to add item to cart", err)
	}

	cs.invalidateCartCache(userUUID)

	return cs.loadCartDTO(ctx, userUUID, cart.CartID, cart.CartUuid)
}

func (cs *cartService) UpdateCartItemQuantity(ctx context.Context, userUUID uuid.UUID, itemUUID uuid.UUID, input v1dto.UpdateCartItemQuantityInput) (v1dto.CartDTO, error) {
	if input.Quantity == nil || *input.Quantity <= 0 {
		return cs.RemoveItemFromCart(ctx, userUUID, itemUUID)
	}

	user, err := cs.getUser(ctx, userUUID)
	if err != nil {
		return v1dto.CartDTO{}, err
	}

	cart, err := cs.getOrCreateCart(ctx, user.UserID)
	if err != nil {
		return v1dto.CartDTO{}, err
	}

	newQty := *input.Quantity

	items, err := cs.repo.GetCartItems(ctx, cart.CartID)
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

	_, err = cs.repo.SetCartItemQuantity(ctx, sqlc.SetCartItemQuantityParams{
		Quantity:     newQty,
		CartItemUuid: itemUUID,
		CartID:       cart.CartID,
	})
	if err != nil {
		return v1dto.CartDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to update cart item quantity", err)
	}

	cs.invalidateCartCache(userUUID)

	return cs.loadCartDTO(ctx, userUUID, cart.CartID, cart.CartUuid)
}

func (cs *cartService) RemoveItemFromCart(ctx context.Context, userUUID uuid.UUID, itemUUID uuid.UUID) (v1dto.CartDTO, error) {
	user, err := cs.getUser(ctx, userUUID)
	if err != nil {
		return v1dto.CartDTO{}, err
	}

	cart, err := cs.getOrCreateCart(ctx, user.UserID)
	if err != nil {
		return v1dto.CartDTO{}, err
	}

	err = cs.repo.DeleteItem(ctx, cart.CartID, itemUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return v1dto.CartDTO{}, utils.NewError(utils.ErrCodeNotFound, "cart item not found")
		}
		return v1dto.CartDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to remove item from cart", err)
	}

	cs.invalidateCartCache(userUUID)

	return cs.loadCartDTO(ctx, userUUID, cart.CartID, cart.CartUuid)
}

func (cs *cartService) ClearMyCart(ctx context.Context, userUUID uuid.UUID) error {
	user, err := cs.getUser(ctx, userUUID)
	if err != nil {
		return err
	}

	cart, err := cs.getOrCreateCart(ctx, user.UserID)
	if err != nil {
		return err
	}

	err = cs.repo.ClearCart(ctx, cart.CartID)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "failed to clear cart", err)
	}

	cs.invalidateCartCache(userUUID)

	return nil
}
