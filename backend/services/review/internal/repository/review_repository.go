package repository

import (
	"context"
	"fmt"
	"strings"

	pkgpgx "goshop/pkg/pgx"
	reviewsqlc "goshop/services/review/internal/db/sqlc"
	reviewdto "goshop/services/review/internal/dto"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ReviewRepository is the persistence port for reviews. Transactional method
// groups inherit their tx from the context via pkg/pgx.TxManager, matching the
// order-service pattern.
type ReviewRepository interface {
	Create(ctx context.Context, params reviewsqlc.CreateReviewParams) (reviewsqlc.ProductReview, error)
	CheckExists(ctx context.Context, userID, productID int32, orderUUID uuid.UUID) (bool, error)
	GetByProduct(ctx context.Context, productID int32, rating *int32, hasMedia *bool, orderBy, sort string, limit, offset int32) ([]reviewdto.ReviewRow, error)
	CountByProduct(ctx context.Context, productID int32, rating *int32, hasMedia *bool) (int64, error)
	GetRatingSummary(ctx context.Context, productID int32) (reviewsqlc.GetProductRatingSummaryRow, error)
	GetRatingAggregate(ctx context.Context, productID int32) (reviewsqlc.GetProductRatingAggregateRow, error)
	GetByUserID(ctx context.Context, userID int32, limit, offset int32) ([]reviewsqlc.GetReviewsByUserIDRow, error)
	CountByUserID(ctx context.Context, userID int32) (int64, error)
	GetByUUID(ctx context.Context, reviewUUID uuid.UUID) (reviewsqlc.ProductReview, error)
	Update(ctx context.Context, params reviewsqlc.UpdateReviewParams) (reviewsqlc.ProductReview, error)
	Delete(ctx context.Context, reviewID int32) error

	// Reports
	HasReported(ctx context.Context, userID, reviewID int32) (bool, error)
	CreateReport(ctx context.Context, params reviewsqlc.InsertReviewReportParams) error
	CountReports(ctx context.Context, reviewID int32) (int64, error)
	GetReportsByReviewID(ctx context.Context, reviewID int32) ([]reviewsqlc.GetReviewReportsByReviewIDRow, error)
	DeleteReportsByReviewID(ctx context.Context, reviewID int32) error

	// Helpful votes
	HasHelpfulVote(ctx context.Context, userID, reviewID int32) (bool, error)
	AddHelpfulVote(ctx context.Context, userID, reviewID int32) (int32, error)
	RemoveHelpfulVote(ctx context.Context, userID, reviewID int32) (int32, error)

	// Admin
	AdminGetAll(ctx context.Context, status *int16, rating *int32, productUUID *uuid.UUID, search *string, orderBy, sort string, limit, offset int32) ([]reviewdto.AdminReviewRow, error)
	AdminCount(ctx context.Context, status *int16, rating *int32, productUUID *uuid.UUID, search *string) (int64, error)
	UpdateStatus(ctx context.Context, reviewID int32, status int16) (reviewsqlc.ProductReview, error)
	Reply(ctx context.Context, reviewID int32, reply string) (reviewsqlc.ProductReview, error)
}

type SqlReviewRepository struct {
	db   reviewsqlc.Querier
	pool *pgxpool.Pool
}

func NewSqlReviewRepository(db reviewsqlc.Querier, pool *pgxpool.Pool) ReviewRepository {
	return &SqlReviewRepository{db: db, pool: pool}
}

// q returns an explicit-tx querier when the context carries one from
// TxManager.WithTx, otherwise the default pool querier.
func (rr *SqlReviewRepository) q(ctx context.Context) reviewsqlc.Querier {
	if tx, ok := pkgpgx.GetTx(ctx); ok {
		return reviewsqlc.New(tx)
	}
	return rr.db
}

func (rr *SqlReviewRepository) Create(ctx context.Context, params reviewsqlc.CreateReviewParams) (reviewsqlc.ProductReview, error) {
	return rr.q(ctx).CreateReview(ctx, params)
}

func (rr *SqlReviewRepository) CheckExists(ctx context.Context, userID, productID int32, orderUUID uuid.UUID) (bool, error) {
	return rr.db.CheckReviewExists(ctx, reviewsqlc.CheckReviewExistsParams{
		ReviewUserID:    userID,
		ReviewProductID: productID,
		ReviewOrderUuid: orderUUID,
	})
}

func (rr *SqlReviewRepository) GetByProduct(ctx context.Context, productID int32, rating *int32, hasMedia *bool, orderBy, sort string, limit, offset int32) ([]reviewdto.ReviewRow, error) {
	query := `SELECT
				r.review_id, r.review_uuid, r.review_rating, r.review_title,
				r.review_comment, r.review_images, r.review_video_url, r.review_has_media,
				r.review_reply, r.review_reply_at, r.review_created_at, r.review_updated_at,
				r.review_helpful_count, r.review_user_name AS reviewer_name
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
	}

	order := "DESC"
	if strings.ToLower(sort) == "asc" {
		order = "ASC"
	}

	switch orderBy {
	case "rating":
		query += fmt.Sprintf(" ORDER BY r.review_rating %s, r.review_created_at DESC", order)
	case "helpful":
		query += fmt.Sprintf(" ORDER BY r.review_helpful_count %s, r.review_created_at DESC", order)
	default:
		query += fmt.Sprintf(" ORDER BY r.review_created_at %s", order)
	}

	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := rr.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviews []reviewdto.ReviewRow
	for rows.Next() {
		var r reviewdto.ReviewRow
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
			&r.ReviewHelpfulCount,
			&r.ReviewerName,
		); err != nil {
			return nil, err
		}
		reviews = append(reviews, r)
	}

	return reviews, rows.Err()
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
	}

	var count int64
	if err := rr.pool.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (rr *SqlReviewRepository) GetRatingSummary(ctx context.Context, productID int32) (reviewsqlc.GetProductRatingSummaryRow, error) {
	return rr.db.GetProductRatingSummary(ctx, productID)
}

func (rr *SqlReviewRepository) GetRatingAggregate(ctx context.Context, productID int32) (reviewsqlc.GetProductRatingAggregateRow, error) {
	return rr.db.GetProductRatingAggregate(ctx, productID)
}

func (rr *SqlReviewRepository) GetByUserID(ctx context.Context, userID int32, limit, offset int32) ([]reviewsqlc.GetReviewsByUserIDRow, error) {
	return rr.db.GetReviewsByUserID(ctx, reviewsqlc.GetReviewsByUserIDParams{
		ReviewUserID: userID,
		Limit:        limit,
		Offset:       offset,
	})
}

func (rr *SqlReviewRepository) CountByUserID(ctx context.Context, userID int32) (int64, error) {
	return rr.db.CountReviewsByUserID(ctx, userID)
}

func (rr *SqlReviewRepository) GetByUUID(ctx context.Context, reviewUUID uuid.UUID) (reviewsqlc.ProductReview, error) {
	return rr.db.GetReviewByUUID(ctx, reviewUUID)
}

func (rr *SqlReviewRepository) Update(ctx context.Context, params reviewsqlc.UpdateReviewParams) (reviewsqlc.ProductReview, error) {
	return rr.q(ctx).UpdateReview(ctx, params)
}

func (rr *SqlReviewRepository) Delete(ctx context.Context, reviewID int32) error {
	return rr.q(ctx).DeleteReview(ctx, reviewID)
}

func (rr *SqlReviewRepository) UpdateStatus(ctx context.Context, reviewID int32, status int16) (reviewsqlc.ProductReview, error) {
	return rr.q(ctx).UpdateReviewStatus(ctx, reviewsqlc.UpdateReviewStatusParams{
		ReviewID:     reviewID,
		ReviewStatus: status,
	})
}

func (rr *SqlReviewRepository) Reply(ctx context.Context, reviewID int32, reply string) (reviewsqlc.ProductReview, error) {
	return rr.q(ctx).ReplyReview(ctx, reviewsqlc.ReplyReviewParams{
		ReviewID:    reviewID,
		ReviewReply: &reply,
	})
}

func (rr *SqlReviewRepository) AdminGetAll(ctx context.Context, status *int16, rating *int32, productUUID *uuid.UUID, search *string, orderBy, sort string, limit, offset int32) ([]reviewdto.AdminReviewRow, error) {
	query := `SELECT
				r.review_id, r.review_uuid,
				r.review_product_id,
				r.review_product_uuid AS product_uuid,
				r.review_order_uuid AS order_uuid,
				r.review_user_name AS reviewer_name, r.review_user_email AS reviewer_email,
				r.review_rating, r.review_title, r.review_comment,
				r.review_images, r.review_video_url, r.review_has_media,
				r.review_status, r.review_reply, r.review_reply_at,
				r.review_created_at, r.review_updated_at, r.review_helpful_count,
				(SELECT COUNT(*)::bigint FROM product_review_reports prr WHERE prr.review_id = r.review_id) AS report_count
			FROM product_reviews r
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
	// review_product_uuid is denormalized at write time — no cross-DB join needed.
	if productUUID != nil {
		query += fmt.Sprintf(" AND r.review_product_uuid = $%d", argIdx)
		args = append(args, *productUUID)
		argIdx++
	}
	if search != nil && *search != "" {
		query += fmt.Sprintf(" AND (unaccent(r.review_title) ILIKE '%%' || unaccent($%d) || '%%' OR unaccent(r.review_comment) ILIKE '%%' || unaccent($%d) || '%%')", argIdx, argIdx)
		args = append(args, *search)
	}
	order := "DESC"
	if strings.ToLower(sort) == "asc" {
		order = "ASC"
	}
	switch orderBy {
	case "rating":
		query += fmt.Sprintf(" ORDER BY r.review_rating %s, r.review_created_at DESC", order)
	default:
		query += fmt.Sprintf(" ORDER BY r.review_created_at %s", order)
	}
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)
	rows, err := rr.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reviews []reviewdto.AdminReviewRow
	for rows.Next() {
		var r reviewdto.AdminReviewRow
		if err := rows.Scan(
			&r.ReviewID, &r.ReviewUUID,
			&r.ProductID,
			&r.ProductUUID,
			&r.OrderUUID,
			&r.ReviewerName, &r.ReviewerEmail,
			&r.ReviewRating, &r.ReviewTitle, &r.ReviewComment,
			&r.ReviewImages, &r.ReviewVideoURL, &r.ReviewHasMedia,
			&r.ReviewStatus,
			&r.ReviewReply, &r.ReviewReplyAt,
			&r.ReviewCreatedAt, &r.ReviewUpdatedAt, &r.ReviewHelpfulCount,
			&r.ReportCount,
		); err != nil {
			return nil, err
		}
		reviews = append(reviews, r)
	}
	return reviews, rows.Err()
}

func (rr *SqlReviewRepository) AdminCount(ctx context.Context, status *int16, rating *int32, productUUID *uuid.UUID, search *string) (int64, error) {
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
	if productUUID != nil {
		query += fmt.Sprintf(" AND r.review_product_uuid = $%d", argIdx)
		args = append(args, *productUUID)
		argIdx++
	}
	if search != nil && *search != "" {
		query += fmt.Sprintf(" AND (unaccent(r.review_title) ILIKE '%%' || unaccent($%d) || '%%' OR unaccent(r.review_comment) ILIKE '%%' || unaccent($%d) || '%%')", argIdx, argIdx)
		args = append(args, *search)
	}
	var count int64
	if err := rr.pool.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (rr *SqlReviewRepository) HasHelpfulVote(ctx context.Context, userID, reviewID int32) (bool, error) {
	return rr.db.CheckUserVotedHelpful(ctx, reviewsqlc.CheckUserVotedHelpfulParams{
		UserID:   userID,
		ReviewID: reviewID,
	})
}

func (rr *SqlReviewRepository) AddHelpfulVote(ctx context.Context, userID, reviewID int32) (int32, error) {
	if err := rr.q(ctx).InsertHelpfulVote(ctx, reviewsqlc.InsertHelpfulVoteParams{
		UserID:   userID,
		ReviewID: reviewID,
	}); err != nil {
		return 0, err
	}
	return rr.q(ctx).IncrementHelpfulCount(ctx, reviewID)
}

func (rr *SqlReviewRepository) RemoveHelpfulVote(ctx context.Context, userID, reviewID int32) (int32, error) {
	if err := rr.q(ctx).DeleteHelpfulVote(ctx, reviewsqlc.DeleteHelpfulVoteParams{
		UserID:   userID,
		ReviewID: reviewID,
	}); err != nil {
		return 0, err
	}
	return rr.q(ctx).DecrementHelpfulCount(ctx, reviewID)
}

func (rr *SqlReviewRepository) HasReported(ctx context.Context, userID, reviewID int32) (bool, error) {
	return rr.db.CheckUserReportedReview(ctx, reviewsqlc.CheckUserReportedReviewParams{
		UserID:   userID,
		ReviewID: reviewID,
	})
}

func (rr *SqlReviewRepository) CreateReport(ctx context.Context, params reviewsqlc.InsertReviewReportParams) error {
	return rr.q(ctx).InsertReviewReport(ctx, params)
}

func (rr *SqlReviewRepository) CountReports(ctx context.Context, reviewID int32) (int64, error) {
	return rr.q(ctx).CountReportsByReviewID(ctx, reviewID)
}

func (rr *SqlReviewRepository) GetReportsByReviewID(ctx context.Context, reviewID int32) ([]reviewsqlc.GetReviewReportsByReviewIDRow, error) {
	return rr.db.GetReviewReportsByReviewID(ctx, reviewID)
}

func (rr *SqlReviewRepository) DeleteReportsByReviewID(ctx context.Context, reviewID int32) error {
	return rr.q(ctx).DeleteReviewReportsByReviewID(ctx, reviewID)
}
