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
