package v1dto

import "goshop/internal/db/sqlc"

type ReviewUUIDParam struct {
	UUID string `uri:"uuid" binding:"required,uuid"`
}
type ProductReviewsParam struct {
	ProductUUID string `uri:"uuid" binding:"required,uuid"`
}

type ReviewDTO struct {
	UUID         string   `json:"uuid"`
	ReviewerName string   `json:"reviewer_name"`
	Rating       int32    `json:"rating"`
	Title        *string  `json:"title,omitempty"`
	Comment      *string  `json:"comment,omitempty"`
	Images       []string `json:"images"`
	VideoURL     *string  `json:"video_url,omitempty"`
	HasMedia     bool     `json:"has_media"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

// ── Create Input ───────────────────────────────────────────────
type CreateReviewInput struct {
	ProductUUID string  `form:"product_uuid" binding:"required,uuid"`
	OrderUUID   string  `form:"order_uuid"   binding:"required,uuid"`
	Rating      int32   `form:"rating"       binding:"required,min=1,max=5"`
	Title       *string `form:"title"        binding:"omitempty,max=200"`
	Comment     *string `form:"comment"      binding:"omitempty,max=2000"`
}

// ── Query Parameters ───────────────────────────────────────────────
type GetReviewsQuery struct {
	Page     int32  `form:"page"      binding:"omitempty,gte=1"`
	Limit    int32  `form:"limit"     binding:"omitempty,gte=1,lte=100"`
	Rating   *int32 `form:"rating"    binding:"omitempty,min=1,max=5"`
	HasMedia *bool  `form:"has_media" binding:"omitempty"`
	OrderBy  string `form:"order_by"  binding:"omitempty,oneof=created_at rating"`
	Sort     string `form:"sort"      binding:"omitempty,oneof=asc desc"`
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
