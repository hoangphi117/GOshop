package repository

import (
	"context"
	"goshop/services/cart/internal/db/sqlc"

	"github.com/google/uuid"
)

type CartRepository interface {
	GetOrCreateCartByUserUUID(ctx context.Context, userUUID uuid.UUID, userID *int32) (sqlc.Cart, error)
	GetCartByUserUUID(ctx context.Context, userUUID uuid.UUID) (sqlc.Cart, error)
	GetCartByUserID(ctx context.Context, userID int32) (sqlc.Cart, error)
	GetCartByCartID(ctx context.Context, cartID int32) (sqlc.Cart, error)
	GetCartByCartUUID(ctx context.Context, cartUUID uuid.UUID) (sqlc.Cart, error)

	GetCartItems(ctx context.Context, cartID int32) ([]sqlc.CartItem, error)
	GetCartItemByUUID(ctx context.Context, itemUUID uuid.UUID, cartID int32) (sqlc.CartItem, error)
	GetCartItemQuantity(ctx context.Context, cartID int32, productUUID uuid.UUID) (int32, error)

	AddOrUpdateCartItem(ctx context.Context, arg sqlc.AddOrUpdateCartItemParams) (sqlc.CartItem, error)
	SetCartItemQuantity(ctx context.Context, arg sqlc.SetCartItemQuantityParams) (sqlc.CartItem, error)
	DeleteCartItemByUUID(ctx context.Context, itemUUID uuid.UUID, cartID int32) (sqlc.CartItem, error)

	DeleteCartItemsByProductUUIDs(ctx context.Context, cartID int32, productUUIDs []uuid.UUID) error
	DeleteCartItemsByProductIDs(ctx context.Context, cartID int32, productIDs []int32) error
	ClearCartByCartID(ctx context.Context, cartID int32) error
	CountCartItems(ctx context.Context, cartID int32) (int64, error)
}

type sqlCartRepository struct {
	q sqlc.Querier
}

func NewSqlCartRepository(q sqlc.Querier) CartRepository {
	return &sqlCartRepository{q: q}
}

func (r *sqlCartRepository) GetOrCreateCartByUserUUID(ctx context.Context, userUUID uuid.UUID, userID *int32) (sqlc.Cart, error) {
	return r.q.GetOrCreateCartByUserUUID(ctx, sqlc.GetOrCreateCartByUserUUIDParams{
		UserUuid: userUUID,
		UserID:   userID,
	})
}

func (r *sqlCartRepository) GetCartByUserUUID(ctx context.Context, userUUID uuid.UUID) (sqlc.Cart, error) {
	return r.q.GetCartByUserUUID(ctx, userUUID)
}

func (r *sqlCartRepository) GetCartByUserID(ctx context.Context, userID int32) (sqlc.Cart, error) {
	return r.q.GetCartByUserID(ctx, &userID)
}

func (r *sqlCartRepository) GetCartByCartID(ctx context.Context, cartID int32) (sqlc.Cart, error) {
	return r.q.GetCartByCartID(ctx, cartID)
}

func (r *sqlCartRepository) GetCartByCartUUID(ctx context.Context, cartUUID uuid.UUID) (sqlc.Cart, error) {
	return r.q.GetCartByCartUUID(ctx, cartUUID)
}

func (r *sqlCartRepository) GetCartItems(ctx context.Context, cartID int32) ([]sqlc.CartItem, error) {
	return r.q.GetCartItems(ctx, cartID)
}

func (r *sqlCartRepository) GetCartItemByUUID(ctx context.Context, itemUUID uuid.UUID, cartID int32) (sqlc.CartItem, error) {
	return r.q.GetCartItemByUUID(ctx, sqlc.GetCartItemByUUIDParams{
		CartItemUuid: itemUUID,
		CartID:       cartID,
	})
}

func (r *sqlCartRepository) GetCartItemQuantity(ctx context.Context, cartID int32, productUUID uuid.UUID) (int32, error) {
	return r.q.GetCartItemQuantity(ctx, sqlc.GetCartItemQuantityParams{
		CartID:      cartID,
		ProductUuid: productUUID,
	})
}

func (r *sqlCartRepository) AddOrUpdateCartItem(ctx context.Context, arg sqlc.AddOrUpdateCartItemParams) (sqlc.CartItem, error) {
	return r.q.AddOrUpdateCartItem(ctx, arg)
}

func (r *sqlCartRepository) SetCartItemQuantity(ctx context.Context, arg sqlc.SetCartItemQuantityParams) (sqlc.CartItem, error) {
	return r.q.SetCartItemQuantity(ctx, arg)
}

func (r *sqlCartRepository) DeleteCartItemByUUID(ctx context.Context, itemUUID uuid.UUID, cartID int32) (sqlc.CartItem, error) {
	return r.q.DeleteCartItemByUUID(ctx, sqlc.DeleteCartItemByUUIDParams{
		CartItemUuid: itemUUID,
		CartID:       cartID,
	})
}

func (r *sqlCartRepository) DeleteCartItemsByProductUUIDs(ctx context.Context, cartID int32, productUUIDs []uuid.UUID) error {
	return r.q.DeleteCartItemsByProductUUIDs(ctx, sqlc.DeleteCartItemsByProductUUIDsParams{
		CartID:       cartID,
		ProductUuids: productUUIDs,
	})
}

func (r *sqlCartRepository) DeleteCartItemsByProductIDs(ctx context.Context, cartID int32, productIDs []int32) error {
	return r.q.DeleteCartItemsByProductIDs(ctx, sqlc.DeleteCartItemsByProductIDsParams{
		CartID:     cartID,
		ProductIds: productIDs,
	})
}

func (r *sqlCartRepository) ClearCartByCartID(ctx context.Context, cartID int32) error {
	return r.q.ClearCartByCartID(ctx, cartID)
}

func (r *sqlCartRepository) CountCartItems(ctx context.Context, cartID int32) (int64, error) {
	return r.q.CountCartItems(ctx, cartID)
}
