package repository

import (
	"context"
	"fmt"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"strings"

	"github.com/google/uuid"
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

func (rr *SqlReviewRepository) GetByProduct(ctx context.Context, productID int32, rating *int32, hasMedia *bool, orderBy, sort string, limit, offset int32) ([]v1dto.ReviewRow, error) {
	query := `SELECT 
				r.review_id, r.review_uuid, r.review_rating, r.review_title, 
				r.review_comment, r.review_images, r.review_video_url, r.review_has_media,
				r.review_reply, r.review_reply_at, r.review_created_at, r.review_updated_at,
				u.user_fullname AS reviewer_name
			FROM product_reviews r
			JOIN users u ON u.user_id = r.review_user_id
			WHERE r.review_product_id = $1 AND r.review_status = 1`

	args := []any{productID}
	argIdx := 2

	if rating != nil {
		query += fmt.Sprintf(" AND r.review_rating = $%d", argIdx)
		args = append(args, *rating)
		argIdx++
	}

	if hasMedia != nil {
		query += fmt.Sprintf(" AND r.review_has_media = $%d", argIdx)
		args = append(args, *hasMedia)
		argIdx++
	}

	order := "DESC"
	if strings.ToLower(sort) == "asc" {
		order = "ASC"
	}

	switch orderBy {
	case "rating":
		query += fmt.Sprintf(" ORDER BY r.review_rating %s, r.review_created_at DESC", order)
	case "created_at":
		query += fmt.Sprintf(" ORDER BY r.review_created_at %s", order)
	default:
		query += fmt.Sprintf(" ORDER BY r.review_created_at %s", order)
	}

	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := db.DBPool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviews []v1dto.ReviewRow
	for rows.Next() {
		var r v1dto.ReviewRow
		if err := rows.Scan(
			&r.ReviewID,
			&r.ReviewUUID,
			&r.ReviewRating,
			&r.ReviewTitle,
			&r.ReviewComment,
			&r.ReviewImages,
			&r.ReviewVideoURL,
			&r.ReviewHasMedia,
			&r.ReviewReply,
			&r.ReviewReplyAt,
			&r.ReviewCreatedAt,
			&r.ReviewUpdatedAt,
			&r.ReviewerName,
		); err != nil {
			return nil, err
		}
		reviews = append(reviews, r)
	}

	return reviews, nil
}

func (rr *SqlReviewRepository) CountByProduct(ctx context.Context, productID int32, rating *int32, hasMedia *bool) (int64, error) {
	query := `SELECT COUNT(*) 
				FROM product_reviews r 
				WHERE r.review_product_id = $1 AND r.review_status = 1`

	args := []any{productID}
	argIdx := 2

	if rating != nil {
		query += fmt.Sprintf(" AND r.review_rating = $%d", argIdx)
		args = append(args, *rating)
		argIdx++
	}

	if hasMedia != nil {
		query += fmt.Sprintf(" AND r.review_has_media = $%d", argIdx)
		args = append(args, *hasMedia)
		argIdx++
	}

	var count int64
	if err := db.DBPool.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (rr *SqlReviewRepository) GetRatingSummary(ctx context.Context, productID int32) (sqlc.GetProductRatingSummaryRow, error) {
	return rr.db.GetProductRatingSummary(ctx, productID)
}

func (rr *SqlReviewRepository) GetByUserID(ctx context.Context, userID int32, limit, offset int32) ([]sqlc.GetReviewsByUserIDRow, error) {
	return rr.db.GetReviewsByUserID(ctx, sqlc.GetReviewsByUserIDParams{
		ReviewUserID: userID,
		Limit:        limit,
		Offset:       offset,
	})
}

func (rr *SqlReviewRepository) CountByUserID(ctx context.Context, userID int32) (int64, error) {
	return rr.db.CountReviewsByUserID(ctx, userID)
}

func (rr *SqlReviewRepository) GetByUUID(ctx context.Context, reviewUUID uuid.UUID) (sqlc.GetReviewByUUIDRow, error) {
	return rr.db.GetReviewByUUID(ctx, reviewUUID)
}

func (rr *SqlReviewRepository) Update(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReviewParams) (sqlc.ProductReview, error) {
	q := sqlc.New(tx)
	return q.UpdateReview(ctx, params)
}

func (rr *SqlReviewRepository) Delete(ctx context.Context, tx pgx.Tx, reviewID int32) error {
	q := sqlc.New(tx)
	return q.DeleteReview(ctx, reviewID)
}

func (rr *SqlReviewRepository) UpdateStatus(ctx context.Context, tx pgx.Tx, reviewID int32, status int16) (sqlc.ProductReview, error) {
	q := sqlc.New(tx)
	return q.UpdateReviewStatus(ctx, sqlc.UpdateReviewStatusParams{
		ReviewID:     reviewID,
		ReviewStatus: status,
	})
}

func (rr *SqlReviewRepository) AdminGetAll(ctx context.Context, status *int16, rating *int32, productUUID, search *string, orderBy, sort string, limit, offset int32) ([]v1dto.AdminReviewRow, error) {
	query := `SELECT 
				r.review_id, r.review_uuid, 
				p.product_uuid, p.product_name,
				o.order_uuid,
				u.user_fullname AS reviewer_name, u.user_email AS reviewer_email,
				r.review_rating, r.review_title, r.review_comment,
				r.review_images, r.review_video_url, r.review_has_media,
				r.review_status, r.review_reply, r.review_reply_at,
				r.review_created_at, r.review_updated_at,
			FROM product_reviews r
			JOIN products p ON p.product_id = r.review_product_id
			JOIN orders o ON o.order_id = r.review_order_id
			JOIN users u ON u.user_id = r.review_user_id
			WHERE 1=1`

	args := []any{}
	argIdx := 1

	if status != nil {
		query += fmt.Sprintf(" AND r.review_status = $%d", argIdx)
		args = append(args, *status)
		argIdx++
	}

	if rating != nil {
		query += fmt.Sprintf(" AND r.review_rating = $%d", argIdx)
		args = append(args, *rating)
		argIdx++
	}

	if productUUID != nil {
		query += fmt.Sprintf(" AND p.product_uuid = $%d", argIdx)
		args = append(args, *productUUID)
		argIdx++
	}

	if search != nil && *search != "" {
		query += fmt.Sprintf(" AND (unaccent(r.review_title) ILIKE '%%' || unaccent($%d) || '%%' OR unaccent(r.review_comment) ILIKE '%%' || unaccent($%d) || '%%')", argIdx, argIdx)
		args = append(args, *search)
		argIdx++
	}

	order := "DESC"
	if strings.ToLower(sort) == "asc" {
		order = "ASC"
	}

	switch orderBy {
	case "rating":
		query += fmt.Sprintf(" ORDER BY r.review_rating %s, r.review_created_at DESC", order)
	case "created_at":
		query += fmt.Sprintf(" ORDER BY r.review_created_at %s", order)
	default:
		query += fmt.Sprintf(" ORDER BY r.review_created_at %s", order)
	}

	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := db.DBPool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviews []v1dto.AdminReviewRow
	for rows.Next() {
		var r v1dto.AdminReviewRow
		if err := rows.Scan(
			&r.ReviewID, &r.ReviewUUID,
			&r.ProductUUID, &r.ProductName,
			&r.OrderUUID,
			&r.ReviewerName, &r.ReviewerEmail,
			&r.ReviewRating, &r.ReviewTitle, &r.ReviewComment,
			&r.ReviewImages, &r.ReviewVideoURL, &r.ReviewHasMedia,
			&r.ReviewStatus,
			&r.ReviewReply, &r.ReviewReplyAt,
			&r.ReviewCreatedAt, &r.ReviewUpdatedAt,
		); err != nil {
			return nil, err
		}
		reviews = append(reviews, r)
	}

	return reviews, nil
}

func (rr *SqlReviewRepository) AdminCount(ctx context.Context, status *int16, rating *int32, productID *int32, search *string) (int64, error) {
	query := `SELECT COUNT(*) FROM product_reviews r WHERE 1=1`
	args := []any{}
	argIdx := 1
	if status != nil {
		query += fmt.Sprintf(" AND r.review_status = $%d", argIdx)
		args = append(args, *status)
		argIdx++
	}
	if rating != nil {
		query += fmt.Sprintf(" AND r.review_rating = $%d", argIdx)
		args = append(args, *rating)
		argIdx++
	}
	if productID != nil {
		query += fmt.Sprintf(" AND r.review_product_id = $%d", argIdx)
		args = append(args, *productID)
		argIdx++
	}
	if search != nil && *search != "" {
		query += fmt.Sprintf(" AND (unaccent(r.review_title) ILIKE '%%' || unaccent($%d) || '%%' OR unaccent(r.review_comment) ILIKE '%%' || unaccent($%d) || '%%')", argIdx, argIdx)
		args = append(args, *search)
		argIdx++
	}
	var count int64
	if err := db.DBPool.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (rr *SqlReviewRepository) Reply(ctx context.Context, tx pgx.Tx, reviewID int32, reply string) (sqlc.ProductReview, error) {
	q := sqlc.New(tx)
	return q.ReplyReview(ctx, sqlc.ReplyReviewParams{
		ReviewID:    reviewID,
		ReviewReply: &reply,
	})
}
