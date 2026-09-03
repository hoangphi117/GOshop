package repository

import (
	"context"
	"goshop/internal/db/sqlc"

	"github.com/jackc/pgx/v5"
)

type SqlReviewRepository struct {
	db sqlc.Querier
}

func NewSqlReviewRepository(db sqlc.Querier) ReviewRepository {
	return &SqlReviewRepository{
		db: db,
	}
}

func (rr *SqlReviewRepository) Create(ctx context.Context, tx pgx.Tx, params sqlc.CreateReviewParams) (sqlc.ProductReview, error) {
	q := sqlc.New(tx)
	return q.CreateReview(ctx, params)
}

func (rr *SqlReviewRepository) CheckPurchased(ctx context.Context, orderID, userID, productID int32) (bool, error) {
	return rr.db.CheckUserPurchasedProduct(ctx, sqlc.CheckUserPurchasedProductParams{
		OrderID:            orderID,
		OrderUserID:        userID,
		OrderItemProductID: productID,
	})
}

func (rr *SqlReviewRepository) CheckExists(ctx context.Context, userID, productID, orderID int32) (bool, error) {
	return rr.db.CheckReviewExists(ctx, sqlc.CheckReviewExistsParams{
		ReviewUserID:    userID,
		ReviewProductID: productID,
		ReviewOrderID:   orderID,
	})
}

func (rr *SqlReviewRepository) RecalcRating(ctx context.Context, tx pgx.Tx, productID int32) error {
	q := sqlc.New(tx)
	return q.RecalcProductRating(ctx, productID)
}
