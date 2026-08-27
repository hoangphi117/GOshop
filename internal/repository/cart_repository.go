package repository

import (
	"context"
	"goshop/internal/db/sqlc"
)

type SqlCartRepository struct {
	db sqlc.Querier
}

func NewSqlCartRepository(db sqlc.Querier) CartRepository {
	return &SqlCartRepository{
		db: db,
	}
}

func (cr *SqlCartRepository) GetOrCreateCart(ctx context.Context, userID int32) (sqlc.Cart, error) {
	return cr.db.GetOrCreateCartByUserID(ctx, userID)
}

func (cr *SqlCartRepository) GetCartByUserID(ctx context.Context, userID int32) (sqlc.Cart, error) {
	return cr.db.GetCartByUserID(ctx, userID)
}

func (cr *SqlCartRepository) GetCartItems(ctx context.Context, cartID int32) ([]sqlc.GetCartItemsWithProductDetailsRow, error) {
	return cr.db.GetCartItemsWithProductDetails(ctx, cartID)
}

func (cr *SqlCartRepository) AddOrUpdateItem(ctx context.Context, params sqlc.AddOrUpdateCartItemParams) (sqlc.CartItem, error) {
	return cr.db.AddOrUpdateCartItem(ctx, params)
}

func (cr *SqlCartRepository) GetCartItemQuantity(ctx context.Context, cartID, productID int32) (int32, error) {
	return cr.db.GetCartItemQuantity(ctx, sqlc.GetCartItemQuantityParams{
		CartID:    cartID,
		ProductID: productID,
	})
}
