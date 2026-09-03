package v1dto

import (
	"goshop/internal/db/sqlc"
	"time"

	"github.com/google/uuid"
)

type ReviewRow struct {
	ReviewID        int32
	ReviewUUID      uuid.UUID
	ReviewRating    int16
	ReviewTitle     *string
	ReviewComment   *string
	ReviewImages    []string
	ReviewVideoURL  *string
	ReviewHasMedia  bool
	ReviewCreatedAt time.Time
	ReviewUpdatedAt time.Time
	ReviewerName    string
}

type ReviewUUIDParam struct {
	UUID string `uri:"uuid" binding:"required,uuid"`
}
type ProductReviewsParam struct {
	ProductUUID string `uri:"uuid" binding:"required,uuid"`
}

type ReviewDTO struct {
	UUID         string   `json:"uuid"`
	ProductUUID  string   `json:"product_uuid,omitempty"`
	OrderUUID    string   `json:"order_uuid,omitempty"`
	ReviewerName string   `json:"reviewer_name"`
	Rating       int32    `json:"rating"`
	Title        *string  `json:"title,omitempty"`
	Comment      *string  `json:"comment,omitempty"`
	Images       []string `json:"images"`
	VideoURL     *string  `json:"video_url,omitempty"`
	HasMedia     bool     `json:"has_media"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at,omitempty"`
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

type GetMyReviewsQuery struct {
	Page  int32 `form:"page"      binding:"omitempty,gte=1"`
	Limit int32 `form:"limit"     binding:"omitempty,gte=1,lte=50"`
}

type MyReviewDTO struct {
	UUID            string   `json:"uuid"`
	ProductUUID     string   `json:"product_uuid"`
	ProductName     string   `json:"product_name"`
	ProductSlug     string   `json:"product_slug"`
	ProductImageURL *string  `json:"product_image_url,omitempty"`
	Rating          int32    `json:"rating"`
	Title           *string  `json:"title,omitempty"`
	Comment         *string  `json:"comment,omitempty"`
	Images          []string `json:"images"`
	VideoURL        *string  `json:"video_url,omitempty"`
	HasMedia        bool     `json:"has_media"`
	CreatedAt       string   `json:"created_at"`
}

type RatingSummaryRow struct {
	TotalReviews   int64
	AvgRating      float64
	WithMediaCount int64
	FiveStar       int64
	FourStar       int64
	ThreeStar      int64
	TwoStar        int64
	OneStar        int64
}

// ── Mapping Functions ─────────────────────────────────────────────

func MapCreatedReviewToDTO(r sqlc.ProductReview, reviewerName, productUUID, orderUUID string) ReviewDTO {
	images := r.ReviewImages
	if r.ReviewImages == nil {
		images = []string{}
	}

	return ReviewDTO{
		UUID:         r.ReviewUuid.String(),
		ProductUUID:  productUUID,
		OrderUUID:    orderUUID,
		ReviewerName: reviewerName,
		Rating:       int32(r.ReviewRating),
		Title:        r.ReviewTitle,
		Comment:      r.ReviewComment,
		Images:       images,
		VideoURL:     r.ReviewVideoUrl,
		HasMedia:     r.ReviewHasMedia,
		CreatedAt:    r.ReviewCreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:    r.ReviewUpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func MapToMyReviewDTO(r sqlc.GetReviewsByUserIDRow) MyReviewDTO {
	images := r.ReviewImages
	if images == nil {
		images = []string{}
	}
	return MyReviewDTO{
		UUID:            r.ReviewUuid.String(),
		ProductUUID:     r.ProductUuid.String(),
		ProductName:     r.ProductName,
		ProductSlug:     r.ProductSlug,
		ProductImageURL: r.ProductImageUrl,
		Rating:          int32(r.ReviewRating),
		Title:           r.ReviewTitle,
		Comment:         r.ReviewComment,
		Images:          images,
		VideoURL:        r.ReviewVideoUrl,
		HasMedia:        r.ReviewHasMedia,
		CreatedAt:       r.ReviewCreatedAt.Format("2006-01-02 15:04:05"),
	}
}

func MapToMyReviewDTOs(rows []sqlc.GetReviewsByUserIDRow) []MyReviewDTO {
	result := make([]MyReviewDTO, len(rows))
	for i, r := range rows {
		result[i] = MapToMyReviewDTO(r)
	}
	return result
}
