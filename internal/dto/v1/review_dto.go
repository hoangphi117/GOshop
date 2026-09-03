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
	ProductUUID string  `json:"product_uuid" binding:"required,uuid"`
	OrderUUID   string  `json:"order_uuid"   binding:"required,uuid"`
	Rating      int32   `json:"rating"       binding:"required,min=1,max=5"`
	Title       *string `json:"title"        binding:"omitempty,max=200"`
	Comment     *string `json:"comment"      binding:"omitempty,max=2000"`
}

// ── Response DTO ─────────────────────────────────────────────
type ReviewResponseDTO struct {
	UUID         string  `json:"uuid"`
	ProductUUID  string  `json:"product_uuid"`
	OrderUUID    string  `json:"order_uuid"`
	ReviewerName string  `json:"reviewer_name"`
	Rating       int32   `json:"rating"`
	Title        *string `json:"title,omitempty"`
	Comment      *string `json:"comment,omitempty"`
	CreatedAt    string  `json:"created_at"`
}

// ── Mapping Functions ─────────────────────────────────────────────
func (input *CreateReviewInput) MapToCreateParams(userID, productID, orderID int32) sqlc.CreateReviewParams {
	return sqlc.CreateReviewParams{
		ReviewUserID:    userID,
		ReviewProductID: productID,
		ReviewOrderID:   orderID,
		ReviewRating:    int16(input.Rating),
		ReviewTitle:     input.Title,
		ReviewComment:   input.Comment,
	}
}

func MapCreatedReviewToDTO(r sqlc.ProductReview, reviewerName, productUUID, orderUUID string) ReviewResponseDTO {
	return ReviewResponseDTO{
		UUID:         r.ReviewUuid.String(),
		ProductUUID:  productUUID,
		OrderUUID:    orderUUID,
		ReviewerName: reviewerName,
		Rating:       int32(r.ReviewRating),
		Title:        r.ReviewTitle,
		Comment:      r.ReviewComment,
		CreatedAt:    r.ReviewCreatedAt.Format("2006-01-02 15:04:05"),
	}
}
