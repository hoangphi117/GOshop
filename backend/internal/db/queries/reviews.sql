-- name: CreateReview :one
INSERT INTO product_reviews (
    review_user_id, review_product_id, review_order_id,
    review_rating, review_title, review_comment, review_images, review_video_url, review_has_media
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: CheckUserPurchasedProduct :one
SELECT EXISTS (
    SELECT 1 FROM order_items oi
    JOIN orders o ON o.order_id = oi.order_item_order_id
    WHERE o.order_id = $1
      AND o.order_user_id = $2
      AND oi.order_item_product_id = $3
      AND o.order_status = 4  -- Delivered
) AS purchased;

-- name: CheckReviewExists :one
SELECT EXISTS (
    SELECT 1 FROM product_reviews
    WHERE review_user_id = $1
      AND review_product_id = $2
      AND review_order_id = $3
) AS exists;

-- name: RecalcProductRating :exec
UPDATE products SET
    product_avg_rating = COALESCE(
        (SELECT ROUND(AVG(pr.review_rating)::numeric, 1)
         FROM product_reviews AS pr
         WHERE pr.review_product_id = $1 AND pr.review_status = 1),
        0.0
    ),
    product_review_count = (
        SELECT COUNT(*)
        FROM product_reviews AS pr
        WHERE pr.review_product_id = $1 AND pr.review_status = 1
    )
WHERE product_id = $1;

-- name: GetProductRatingSummary :one
SELECT
    COUNT(*)::bigint AS total_reviews,
    COALESCE(ROUND(AVG(review_rating)::numeric, 1), 0.0)::float8 AS avg_rating,
    COUNT(*) FILTER (WHERE review_has_media = TRUE)::bigint AS with_media_count,
    COUNT(*) FILTER (WHERE review_rating = 5)::bigint AS five_star,
    COUNT(*) FILTER (WHERE review_rating = 4)::bigint AS four_star,
    COUNT(*) FILTER (WHERE review_rating = 3)::bigint AS three_star,
    COUNT(*) FILTER (WHERE review_rating = 2)::bigint AS two_star,
    COUNT(*) FILTER (WHERE review_rating = 1)::bigint AS one_star
FROM product_reviews
WHERE review_product_id = $1 AND review_status = 1;

-- name: GetReviewsByUserID :many
SELECT
    r.review_id,
    r.review_uuid,
    r.review_rating,
    r.review_title,
    r.review_comment,
    r.review_images,
    r.review_video_url,
    r.review_has_media,
    r.review_edit_count,
    r.review_created_at,
    p.product_uuid,
    p.product_name,
    p.product_slug,
    p.product_image_url
FROM product_reviews r
JOIN products p ON p.product_id = r.review_product_id
WHERE r.review_user_id = $1
ORDER BY r.review_created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountReviewsByUserID :one
SELECT COUNT(*)::bigint FROM product_reviews
WHERE review_user_id = $1;

-- name: GetReviewByUUID :one
SELECT 
    r.*,
    p.product_uuid
FROM product_reviews r
JOIN products p ON p.product_id = r.review_product_id
WHERE r.review_uuid = $1;

-- name: UpdateReview :one
UPDATE product_reviews SET
    review_rating = $2,
    review_title = $3,
    review_comment = $4,
    review_edit_count = review_edit_count + 1
WHERE review_id = $1
RETURNING *;

-- name: DeleteReview :exec
DELETE FROM product_reviews
WHERE review_id = $1;

-- name: UpdateReviewStatus :one
UPDATE product_reviews SET
    review_status = $2
WHERE review_id = $1
RETURNING *;

-- name: ReplyReview :one
UPDATE product_reviews SET
    review_reply = $2,
    review_reply_at = now()
WHERE review_id = $1
RETURNING *;

-- Helpful Votes Queries

-- name: CheckUserVotedHelpful :one
SELECT EXISTS(
    SELECT 1 FROM product_review_helpful_votes 
    WHERE review_id = $1 AND user_id = $2
) AS has_voted;

-- name: InsertHelpfulVote :exec
INSERT INTO product_review_helpful_votes (review_id, user_id)
VALUES ($1, $2);

-- name: DeleteHelpfulVote :exec
DELETE FROM product_review_helpful_votes
WHERE review_id = $1 AND user_id = $2;

-- name: IncrementHelpfulCount :one
UPDATE product_reviews SET
    review_helpful_count = review_helpful_count + 1
WHERE review_id = $1
RETURNING review_helpful_count;

-- name: DecrementHelpfulCount :one
UPDATE product_reviews SET
    review_helpful_count = GREATEST(review_helpful_count - 1, 0)
WHERE review_id = $1
RETURNING review_helpful_count;

-- Review Reports Queries

-- name: CheckUserReportedReview :one
SELECT EXISTS(
    SELECT 1 FROM product_review_reports 
    WHERE review_id = $1 AND user_id = $2
) AS has_reported;

-- name: InsertReviewReport :exec
INSERT INTO product_review_reports (
    review_id, user_id, reason, details
) VALUES ($1, $2, $3, $4);

-- name: CountReportsByReviewID :one
SELECT COUNT(*)::bigint FROM product_review_reports
WHERE review_id = $1;

-- name: GetReviewReportsByReviewID :many
SELECT 
    prr.report_id,
    prr.user_id,
    prr.user_name,
    prr.user_email,
    prr.reason,
    prr.details,
    prr.created_at
FROM product_review_reports prr
WHERE prr.review_id = $1
ORDER BY prr.created_at DESC;

-- name: DeleteReviewReportsByReviewID :exec
DELETE FROM product_review_reports
WHERE review_id = $1;