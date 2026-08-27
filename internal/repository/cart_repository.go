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

func (cr *SqlCartRepository) GetCartItems(ctx context.Context, cartID int32) ([]sqlc.GetCartItemsWithProductDetailsRow, error) {
	return cr.db.GetCartItemsWithProductDetails(ctx, cartID)
}
