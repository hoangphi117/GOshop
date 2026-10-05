package repository

import (
	"context"
	"goshop/internal/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

func (cr *SqlCartRepository) SetCartItemQuantity(ctx context.Context, params sqlc.SetCartItemQuantityParams) (sqlc.CartItem, error) {
	return cr.db.SetCartItemQuantity(ctx, params)
}

func (cr *SqlCartRepository) DeleteItem(ctx context.Context, cartID int32, itemUUID uuid.UUID) error {
	_, err := cr.db.DeleteCartItemByUUID(ctx, sqlc.DeleteCartItemByUUIDParams{
		CartItemUuid: itemUUID,
		CartID:       cartID,
	})
	return err
}

func (cr *SqlCartRepository) ClearCart(ctx context.Context, cartID int32) error {
	return cr.db.ClearCartByCartID(ctx, cartID)
}

func (cr *SqlCartRepository) DeleteCartItemsByProductIDs(ctx context.Context, tx pgx.Tx, cartID int32, productIDs []int32) error {
	var q sqlc.Querier = cr.db
	if tx != nil {
		q = sqlc.New(tx)
	}
	return q.DeleteCartItemsByProductIDs(ctx, sqlc.DeleteCartItemsByProductIDsParams{
		CartID:     cartID,
		ProductIds: productIDs,
	})
}

func (cr *SqlCartRepository) CountCartItems(ctx context.Context, cartID int32) (int64, error) {
	return cr.db.CountCartItems(ctx, cartID)
}
