package v1dto

import "goshop/internal/db/sqlc"

type ReviewUUIDParam struct {
	UUID string `uri:"uuid" binding:"required,uuid"`
}
type ProductReviewsParam struct {
	ProductUUID string `uri:"product_uuid" binding:"required,uuid"`
}

// ── Create Input ───────────────────────────────────────────────
type CreateReviewInput struct {
	ProductUUID string  `form:"product_uuid" binding:"required,uuid"`
	OrderUUID   string  `form:"order_uuid"   binding:"required,uuid"`
	Rating      int32   `form:"rating"       binding:"required,min=1,max=5"`
	Title       *string `form:"title"        binding:"omitempty,max=200"`
	Comment     *string `form:"comment"      binding:"omitempty,max=2000"`
}

// ── Response DTO ─────────────────────────────────────────────
type ReviewResponseDTO struct {
	UUID         string   `json:"uuid"`
	ProductUUID  string   `json:"product_uuid"`
	OrderUUID    string   `json:"order_uuid"`
	ReviewerName string   `json:"reviewer_name"`
	Rating       int32    `json:"rating"`
	Title        *string  `json:"title,omitempty"`
	Comment      *string  `json:"comment,omitempty"`
	ImageURLs    []string `json:"images"`
	VideoURL     *string  `json:"video_url,omitempty"`
	CreatedAt    string   `json:"created_at"`
}

// ── Mapping Functions ─────────────────────────────────────────────

func MapCreatedReviewToDTO(r sqlc.ProductReview, reviewerName, productUUID, orderUUID string) ReviewResponseDTO {

	images := r.ReviewImages
	if r.ReviewImages == nil {
		images = []string{}
	}

	return ReviewResponseDTO{
		UUID:         r.ReviewUuid.String(),
		ProductUUID:  productUUID,
		OrderUUID:    orderUUID,
		ReviewerName: reviewerName,
		Rating:       int32(r.ReviewRating),
		Title:        r.ReviewTitle,
		Comment:      r.ReviewComment,
		ImageURLs:    images,
		VideoURL:     r.ReviewVideoUrl,
		CreatedAt:    r.ReviewCreatedAt.Format("2006-01-02 15:04:05"),
	}
}
