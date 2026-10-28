package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/utils"
	"goshop/services/cart/internal/db/sqlc"
	"goshop/services/cart/internal/dto"
	"goshop/services/cart/internal/repository"
	catalogcontract "goshop/services/catalog/contract"

	"github.com/google/uuid"
)

type CartService interface {
	GetMyCart(ctx context.Context, userUUID uuid.UUID, userID *int32) (dto.CartDTO, error)
	AddItemToCart(ctx context.Context, userUUID uuid.UUID, userID *int32, input dto.AddCartItemInput) (dto.CartDTO, error)
	UpdateCartItemQuantity(ctx context.Context, userUUID uuid.UUID, userID *int32, itemUUID uuid.UUID, input dto.UpdateCartItemQuantityInput) (dto.CartDTO, error)
	RemoveItemFromCart(ctx context.Context, userUUID uuid.UUID, userID *int32, itemUUID uuid.UUID) (dto.CartDTO, error)
	ClearMyCart(ctx context.Context, userUUID uuid.UUID, userID *int32) error
	RemovePurchasedItems(ctx context.Context, userUUID *uuid.UUID, userID *int32, productUUIDs []uuid.UUID, productIDs []int32) error
}

type cartService struct {
	repo          repository.CartRepository
	catalogClient catalogcontract.CatalogClient
	cache         cache.RedisCacheService
}

func NewCartService(
	repo repository.CartRepository,
	catalogClient catalogcontract.CatalogClient,
	cache cache.RedisCacheService,
) CartService {
	return &cartService{
		repo:          repo,
		catalogClient: catalogClient,
		cache:         cache,
	}
}

func roundFloat(val float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}

func (cs *cartService) invalidateCartCache(userUUID uuid.UUID) {
	if cs.cache != nil {
		cacheKey := fmt.Sprintf("cart:user:%s", userUUID.String())
		if err := cs.cache.Clear(cacheKey); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear cart cache")
		}
	}
}

func (cs *cartService) getOrCreateCart(ctx context.Context, userUUID uuid.UUID, userID *int32) (sqlc.Cart, error) {
	cart, err := cs.repo.GetOrCreateCartByUserUUID(ctx, userUUID, userID)
	if err != nil {
		return sqlc.Cart{}, utils.WrapError(utils.ErrCodeInternal, "failed to retrieve or create cart", err)
	}
	return cart, nil
}

func (cs *cartService) loadCartDTO(ctx context.Context, userUUID uuid.UUID, cart sqlc.Cart) (dto.CartDTO, error) {
	items, err := cs.repo.GetCartItems(ctx, cart.CartID)
	if err != nil {
		return dto.CartDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to retrieve cart items", err)
	}

	if len(items) == 0 {
		return dto.CartDTO{
			UUID:        cart.CartUuid.String(),
			TotalItems:  0,
			TotalAmount: 0,
			Items:       []dto.CartItemDTO{},
		}, nil
	}

	// Fetch product details via CatalogClient
	productUUIDs := make([]string, len(items))
	for i, item := range items {
		productUUIDs[i] = item.ProductUuid.String()
	}

	productMap := make(map[string]catalogcontract.ProductItemInfo)
	if cs.catalogClient != nil {
		products, err := cs.catalogClient.GetProductsBatch(ctx, productUUIDs, nil)
		if err == nil {
			for _, p := range products {
				productMap[p.ProductUUID] = p
			}
		} else {
			logger.Log.Warn().Err(err).Msg("Failed to fetch product batch from catalog service")
		}
	}

	cartItems := make([]dto.CartItemDTO, 0, len(items))
	var totalAmount float64

	for _, item := range items {
		pInfo, found := productMap[item.ProductUuid.String()]
		price := pInfo.ProductPrice
		subtotal := roundFloat(price*float64(item.Quantity), 2)
		totalAmount = roundFloat(totalAmount+subtotal, 2)

		isAvailable := found &&
			pInfo.ProductStatus == dto.ProductStatusActive &&
			pInfo.ProductStock >= item.Quantity

		prodID := pInfo.ProductID
		if prodID == 0 && item.ProductID != nil {
			prodID = *item.ProductID
		}

		cartItems = append(cartItems, dto.CartItemDTO{
			UUID:           item.CartItemUuid.String(),
			ProductUUID:    item.ProductUuid.String(),
			ProductID:      prodID,
			ProductName:    pInfo.ProductName,
			ProductSlug:    pInfo.ProductSlug,
			ProductImage:   pInfo.ProductImageURL,
			Price:          price,
			Quantity:       item.Quantity,
			Subtotal:       subtotal,
			StockAvailable: pInfo.ProductStock,
			IsAvailable:    isAvailable,
		})
	}

	cartDTO := dto.CartDTO{
		UUID:        cart.CartUuid.String(),
		TotalItems:  int32(len(cartItems)),
		TotalAmount: totalAmount,
		Items:       cartItems,
	}

	if cs.cache != nil {
		cacheKey := fmt.Sprintf("cart:user:%s", userUUID.String())
		if err := cs.cache.Set(cacheKey, cartDTO, 5*time.Minute); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to set cart cache")
		}
	}

	return cartDTO, nil
}

func (cs *cartService) GetMyCart(ctx context.Context, userUUID uuid.UUID, userID *int32) (dto.CartDTO, error) {
	cacheKey := fmt.Sprintf("cart:user:%s", userUUID.String())
	if cs.cache != nil {
		var cached dto.CartDTO
		if err := cs.cache.Get(cacheKey, &cached); err == nil && cached.UUID != "" {
			return cached, nil
		}
	}

	cart, err := cs.getOrCreateCart(ctx, userUUID, userID)
	if err != nil {
		return dto.CartDTO{}, err
	}

	return cs.loadCartDTO(ctx, userUUID, cart)
}

func (cs *cartService) AddItemToCart(ctx context.Context, userUUID uuid.UUID, userID *int32, input dto.AddCartItemInput) (dto.CartDTO, error) {
	productUUID, err := uuid.Parse(input.ProductUUID)
	if err != nil {
		return dto.CartDTO{}, utils.NewError(utils.ErrCodeBadRequest, "invalid product UUID")
	}

	cart, err := cs.getOrCreateCart(ctx, userUUID, userID)
	if err != nil {
		return dto.CartDTO{}, err
	}

	var productID *int32
	if cs.catalogClient != nil {
		p, err := cs.catalogClient.GetProductByUUID(ctx, input.ProductUUID)
		if err != nil {
			return dto.CartDTO{}, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		if p.ProductStatus != dto.ProductStatusActive {
			return dto.CartDTO{}, utils.NewError(utils.ErrCodeBadRequest, "product is unavailable for purchase")
		}
		existingQty, _ := cs.repo.GetCartItemQuantity(ctx, cart.CartID, productUUID)
		totalQty := existingQty + input.Quantity
		if p.ProductStock < totalQty {
			return dto.CartDTO{}, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("insufficient stock: available %d, requested total %d", p.ProductStock, totalQty))
		}
		productID = &p.ProductID
	}

	_, err = cs.repo.AddOrUpdateCartItem(ctx, sqlc.AddOrUpdateCartItemParams{
		CartID:      cart.CartID,
		ProductUuid: productUUID,
		ProductID:   productID,
		Quantity:    input.Quantity,
	})
	if err != nil {
		return dto.CartDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to add item to cart", err)
	}

	cs.invalidateCartCache(userUUID)
	return cs.loadCartDTO(ctx, userUUID, cart)
}

func (cs *cartService) UpdateCartItemQuantity(ctx context.Context, userUUID uuid.UUID, userID *int32, itemUUID uuid.UUID, input dto.UpdateCartItemQuantityInput) (dto.CartDTO, error) {
	if input.Quantity == nil || *input.Quantity <= 0 {
		return cs.RemoveItemFromCart(ctx, userUUID, userID, itemUUID)
	}

	cart, err := cs.getOrCreateCart(ctx, userUUID, userID)
	if err != nil {
		return dto.CartDTO{}, err
	}

	item, err := cs.repo.GetCartItemByUUID(ctx, itemUUID, cart.CartID)
	if err != nil {
		return dto.CartDTO{}, utils.NewError(utils.ErrCodeNotFound, "cart item not found")
	}

	newQty := *input.Quantity

	if cs.catalogClient != nil {
		p, err := cs.catalogClient.GetProductByUUID(ctx, item.ProductUuid.String())
		if err == nil {
			if p.ProductStatus != dto.ProductStatusActive {
				return dto.CartDTO{}, utils.NewError(utils.ErrCodeBadRequest, "product is unavailable for purchase")
			}
			if p.ProductStock < newQty {
				return dto.CartDTO{}, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("insufficient stock: available %d, requested %d", p.ProductStock, newQty))
			}
		}
	}

	_, err = cs.repo.SetCartItemQuantity(ctx, sqlc.SetCartItemQuantityParams{
		Quantity:     newQty,
		CartItemUuid: itemUUID,
		CartID:       cart.CartID,
	})
	if err != nil {
		return dto.CartDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to update cart item quantity", err)
	}

	cs.invalidateCartCache(userUUID)
	return cs.loadCartDTO(ctx, userUUID, cart)
}

func (cs *cartService) RemoveItemFromCart(ctx context.Context, userUUID uuid.UUID, userID *int32, itemUUID uuid.UUID) (dto.CartDTO, error) {
	cart, err := cs.getOrCreateCart(ctx, userUUID, userID)
	if err != nil {
		return dto.CartDTO{}, err
	}

	_, err = cs.repo.DeleteCartItemByUUID(ctx, itemUUID, cart.CartID)
	if err != nil {
		return dto.CartDTO{}, utils.NewError(utils.ErrCodeNotFound, "cart item not found")
	}

	cs.invalidateCartCache(userUUID)
	return cs.loadCartDTO(ctx, userUUID, cart)
}

func (cs *cartService) ClearMyCart(ctx context.Context, userUUID uuid.UUID, userID *int32) error {
	cart, err := cs.getOrCreateCart(ctx, userUUID, userID)
	if err != nil {
		return err
	}

	if err := cs.repo.ClearCartByCartID(ctx, cart.CartID); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "failed to clear cart", err)
	}

	cs.invalidateCartCache(userUUID)
	return nil
}

func (cs *cartService) RemovePurchasedItems(ctx context.Context, userUUID *uuid.UUID, userID *int32, productUUIDs []uuid.UUID, productIDs []int32) error {
	var cart sqlc.Cart
	var err error

	if userUUID != nil {
		cart, err = cs.repo.GetCartByUserUUID(ctx, *userUUID)
	} else if userID != nil {
		cart, err = cs.repo.GetCartByUserID(ctx, *userID)
	} else {
		return utils.NewError(utils.ErrCodeBadRequest, "user identification required")
	}

	if err != nil {
		// If cart doesn't exist, no items to remove
		return nil
	}

	if len(productUUIDs) > 0 {
		if err := cs.repo.DeleteCartItemsByProductUUIDs(ctx, cart.CartID, productUUIDs); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "failed to delete purchased items by uuid", err)
		}
	}

	if len(productIDs) > 0 {
		if err := cs.repo.DeleteCartItemsByProductIDs(ctx, cart.CartID, productIDs); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "failed to delete purchased items by id", err)
		}
	}

	if userUUID != nil {
		cs.invalidateCartCache(*userUUID)
	} else {
		if cs.cache != nil {
			_ = cs.cache.Clear("cart:user:*")
		}
	}

	return nil
}
