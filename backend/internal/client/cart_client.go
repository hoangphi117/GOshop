package client

import (
	"context"
	"fmt"

	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/services/cart/contract"

	"github.com/google/uuid"
)

type (
	CartClient     = contract.CartClient
	GRPCCartClient = contract.GRPCCartClient
)

var (
	NewGRPCCartClient = contract.NewGRPCCartClient
)

var (
	_ v1service.CartService     = (*CartServiceAdapter)(nil)
	_ v1service.OrderCartPort   = (*CartServiceAdapter)(nil)
)

// CartServiceAdapter bridges the standalone Cart gRPC microservice into Monolith services & workers
type CartServiceAdapter struct {
	client contract.CartClient
}

func NewCartServiceAdapter(client contract.CartClient) *CartServiceAdapter {
	return &CartServiceAdapter{client: client}
}

func (a *CartServiceAdapter) GetMyCart(ctx context.Context, userUUID uuid.UUID) (v1dto.CartDTO, error) {
	if a.client == nil {
		return v1dto.CartDTO{}, fmt.Errorf("cart gRPC client is not initialized")
	}

	cart, err := a.client.GetCart(ctx, userUUID.String(), 0)
	if err != nil {
		return v1dto.CartDTO{}, err
	}
	return mapCartInfoToDTO(cart), nil
}

func (a *CartServiceAdapter) AddItemToCart(ctx context.Context, userUUID uuid.UUID, input v1dto.AddCartItemInput) (v1dto.CartDTO, error) {
	if a.client == nil {
		return v1dto.CartDTO{}, fmt.Errorf("cart gRPC client is not initialized")
	}

	cart, err := a.client.AddItem(ctx, userUUID.String(), 0, input.ProductUUID, input.Quantity)
	if err != nil {
		return v1dto.CartDTO{}, err
	}
	return mapCartInfoToDTO(cart), nil
}

func (a *CartServiceAdapter) UpdateCartItemQuantity(ctx context.Context, userUUID uuid.UUID, itemUUID uuid.UUID, input v1dto.UpdateCartItemQuantityInput) (v1dto.CartDTO, error) {
	if a.client == nil {
		return v1dto.CartDTO{}, fmt.Errorf("cart gRPC client is not initialized")
	}

	var qty int32
	if input.Quantity != nil {
		qty = *input.Quantity
	}

	cart, err := a.client.UpdateItemQuantity(ctx, userUUID.String(), 0, itemUUID.String(), qty)
	if err != nil {
		return v1dto.CartDTO{}, err
	}
	return mapCartInfoToDTO(cart), nil
}

func (a *CartServiceAdapter) RemoveItemFromCart(ctx context.Context, userUUID uuid.UUID, itemUUID uuid.UUID) (v1dto.CartDTO, error) {
	if a.client == nil {
		return v1dto.CartDTO{}, fmt.Errorf("cart gRPC client is not initialized")
	}

	cart, err := a.client.RemoveItem(ctx, userUUID.String(), 0, itemUUID.String())
	if err != nil {
		return v1dto.CartDTO{}, err
	}
	return mapCartInfoToDTO(cart), nil
}

func (a *CartServiceAdapter) ClearMyCart(ctx context.Context, userUUID uuid.UUID) error {
	if a.client == nil {
		return fmt.Errorf("cart gRPC client is not initialized")
	}

	_, err := a.client.ClearCart(ctx, userUUID.String(), 0)
	return err
}

func (a *CartServiceAdapter) RemovePurchasedItems(ctx context.Context, userID int32, productIDs []int32) error {
	if a.client == nil {
		return fmt.Errorf("cart gRPC client is not initialized")
	}

	_, err := a.client.RemovePurchasedItems(ctx, "", userID, nil, productIDs)
	return err
}

func mapCartInfoToDTO(cart *contract.CartInfo) v1dto.CartDTO {
	if cart == nil {
		return v1dto.CartDTO{}
	}

	items := make([]v1dto.CartItemDTO, len(cart.Items))
	for i, it := range cart.Items {
		items[i] = v1dto.CartItemDTO{
			UUID:           it.ItemUUID,
			ProductUUID:    it.ProductUUID,
			ProductName:    it.ProductName,
			ProductSlug:    it.ProductSlug,
			ProductImage:   it.ProductImageURL,
			Price:          it.Price,
			Quantity:       it.Quantity,
			Subtotal:       it.Subtotal,
			StockAvailable: it.StockAvailable,
			IsAvailable:    it.IsAvailable,
		}
	}

	return v1dto.CartDTO{
		UUID:        cart.CartUUID,
		TotalItems:  cart.TotalItems,
		TotalAmount: cart.TotalAmount,
		Items:       items,
	}
}
