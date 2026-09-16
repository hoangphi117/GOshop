package v1dto

import (
	"goshop/internal/db/sqlc"
	"time"

	"github.com/google/uuid"
)

type ReviewRow struct {
	ReviewID           int32
	ReviewUUID         uuid.UUID
	ReviewRating       int16
	ReviewTitle        *string
	ReviewComment      *string
	ReviewImages       []string
	ReviewVideoURL     *string
	ReviewHasMedia     bool
	ReviewerName       string
	ReviewReply        *string
	ReviewReplyAt      *time.Time
	ReviewHelpfulCount int32
	ReviewCreatedAt    time.Time
	ReviewUpdatedAt    time.Time
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
	CanEdit      bool     `json:"can_edit"`
	Reply        *string  `json:"reply,omitempty"`
	ReplyAt      *string  `json:"reply_at,omitempty"`
	HelpfulCount int32    `json:"helpful_count"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at,omitempty"`
}

type AdminReviewRow struct {
	ReviewID           int32
	ReviewUUID         uuid.UUID
	ProductUUID        uuid.UUID
	ProductName        string
	OrderUUID          uuid.UUID
	ReviewerName       string
	ReviewerEmail      string
	ReviewRating       int16
	ReviewTitle        *string
	ReviewComment      *string
	ReviewImages       []string
	ReviewVideoURL     *string
	ReviewHasMedia     bool
	ReviewReply        *string
	ReviewReplyAt      *time.Time
	ReviewStatus       int16
	ReviewCreatedAt    time.Time
	ReviewUpdatedAt    time.Time
	ReviewHelpfulCount int32
	ReportCount        int64
}

type AdminReviewDTO struct {
	UUID          string   `json:"uuid"`
	ProductUUID   string   `json:"product_uuid"`
	ProductName   string   `json:"product_name"`
	OrderUUID     string   `json:"order_uuid"`
	ReviewerName  string   `json:"reviewer_name"`
	ReviewerEmail string   `json:"reviewer_email"`
	Rating        int32    `json:"rating"`
	Title         *string  `json:"title,omitempty"`
	Comment       *string  `json:"comment,omitempty"`
	Images        []string `json:"images"`
	VideoURL      *string  `json:"video_url,omitempty"`
	HasMedia      bool     `json:"has_media"`
	Status        int16    `json:"status"` // 1: Visible, 2: Hidden, 3: Reported
	Reply         *string  `json:"reply,omitempty"`
	ReplyAt       *string  `json:"reply_at,omitempty"`
	HelpfulCount  int32    `json:"helpful_count"`
	ReportCount   int64    `json:"report_count"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at,omitempty"`
}

type ReviewReportDTO struct {
	ReportID  int32   `json:"report_id"`
	UserID    int32   `json:"user_id"`
	UserName  string  `json:"user_name"`
	UserEmail string  `json:"user_email"`
	Reason    string  `json:"reason"` // spam, inappropriate, fake, other
	Details   *string `json:"details,omitempty"`
	CreatedAt string  `json:"created_at"`
}

type ToggleHelpfulResponseDTO struct {
	Voted        bool  `json:"voted"`         // true: đã vote, false: đã hủy vote
	HelpfulCount int32 `json:"helpful_count"` // số lượng vote mới nhất
}

// ── Input ───────────────────────────────────────────────
type CreateReviewInput struct {
	ProductUUID string  `form:"product_uuid" binding:"required,uuid"`
	OrderUUID   string  `form:"order_uuid"   binding:"required,uuid"`
	Rating      int32   `form:"rating"       binding:"required,min=1,max=5"`
	Title       *string `form:"title"        binding:"omitempty,max=200"`
	Comment     *string `form:"comment"      binding:"omitempty,max=2000"`
}

type UpdateReviewInput struct {
	Rating  int32   `json:"rating"  binding:"required,min=1,max=5"`
	Title   *string `json:"title"   binding:"omitempty,max=200"`
	Comment *string `json:"comment" binding:"omitempty,max=2000"`
}

type UpdateReviewStatusInput struct {
	Status int16 `json:"status" binding:"required,oneof=1 2 3"` // 1: Visible, 2: Hidden, 3: Reported
}

type ReplyReviewInput struct {
	Reply string `json:"reply" binding:"required,min=1,max=2000"`
}

type ReportReviewInput struct {
	Reason  string  `json:"reason"  binding:"required,oneof=spam inappropriate fake other"`
	Details *string `json:"details" binding:"omitempty,max=500"`
}

// ── Query Parameters ───────────────────────────────────────────────
type GetReviewsQuery struct {
	Page     int32  `form:"page"      binding:"omitempty,gte=1"`
	Limit    int32  `form:"limit"     binding:"omitempty,gte=1,lte=100"`
	Rating   *int32 `form:"rating"    binding:"omitempty,min=1,max=5"`
	HasMedia *bool  `form:"has_media" binding:"omitempty"`
	OrderBy  string `form:"order_by"  binding:"omitempty,oneof=created_at rating helpful"`
	Sort     string `form:"sort"      binding:"omitempty,oneof=asc desc"`
}

type AdminGetReviewsQuery struct {
	Page        int32   `form:"page"         binding:"omitempty,gte=1"`
	Limit       int32   `form:"limit"        binding:"omitempty,gte=1,lte=100"`
	Status      *int16  `form:"status"       binding:"omitempty,oneof=1 2 3"` // 1: Visible, 2: Hidden, 3: Reported
	Rating      *int32  `form:"rating"       binding:"omitempty,min=1,max=5"`
	ProductUUID *string `form:"product_uuid" binding:"omitempty,uuid"`
	Search      *string `form:"search"       binding:"omitempty,max=100"`
	OrderBy     string  `form:"order_by"     binding:"omitempty,oneof=created_at rating"`
	Sort        string  `form:"sort"         binding:"omitempty,oneof=asc desc"`
}

type GetMyReviewsQuery struct {
	Page  int32 `form:"page"      binding:"omitempty,gte=1"`
	Limit int32 `form:"limit"     binding:"omitempty,gte=1,lte=50"`
}

type ReviewUUIDParam struct {
	UUID string `uri:"uuid" binding:"required,uuid"`
}
type ProductReviewsParam struct {
	ProductUUID string `uri:"uuid" binding:"required,uuid"`
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
	CanEdit         bool     `json:"can_edit"`
	CreatedAt       string   `json:"created_at"`
}

type RatingSummaryRow struct {
	TotalReviews   int64   `json:"total_reviews"`
	AvgRating      float64 `json:"avg_rating"`
	WithMediaCount int64   `json:"with_media_count"`
	FiveStar       int64   `json:"five_star"`
	FourStar       int64   `json:"four_star"`
	ThreeStar      int64   `json:"three_star"`
	TwoStar        int64   `json:"two_star"`
	OneStar        int64   `json:"one_star"`
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
		CanEdit:      true,
		CreatedAt:    r.ReviewCreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:    r.ReviewUpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func MapToMyReviewDTO(r sqlc.GetReviewsByUserIDRow) MyReviewDTO {
	images := r.ReviewImages
	if images == nil {
		images = []string{}
	}

	canEdit := (r.ReviewEditCount == 0) && (time.Since(r.ReviewCreatedAt) <= 30*24*time.Hour)

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
		CanEdit:         canEdit,
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

func MapToAdminReviewDTO(r AdminReviewRow) AdminReviewDTO {
	images := r.ReviewImages
	if images == nil {
		images = []string{}
	}
	var replyAt *string
	if r.ReviewReplyAt != nil {
		formatted := r.ReviewReplyAt.Format("2006-01-02 15:04:05")
		replyAt = &formatted
	}
	return AdminReviewDTO{
		UUID:          r.ReviewUUID.String(),
		ProductUUID:   r.ProductUUID.String(),
		ProductName:   r.ProductName,
		OrderUUID:     r.OrderUUID.String(),
		ReviewerName:  r.ReviewerName,
		ReviewerEmail: r.ReviewerEmail,
		Rating:        int32(r.ReviewRating),
		Title:         r.ReviewTitle,
		Comment:       r.ReviewComment,
		Images:        images,
		VideoURL:      r.ReviewVideoURL,
		HasMedia:      r.ReviewHasMedia,
		Status:        r.ReviewStatus,
		Reply:         r.ReviewReply,
		ReplyAt:       replyAt,
		HelpfulCount:  r.ReviewHelpfulCount,
		ReportCount:   r.ReportCount,
		CreatedAt:     r.ReviewCreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:     r.ReviewUpdatedAt.Format("2006-01-02 15:04:05"),
	}
}
func MapToAdminReviewDTOs(rows []AdminReviewRow) []AdminReviewDTO {
	result := make([]AdminReviewDTO, len(rows))
	for i, r := range rows {
		result[i] = MapToAdminReviewDTO(r)
	}
	return result
}

func MapToReviewReportDTO(r sqlc.GetReviewReportsByReviewIDRow) ReviewReportDTO {
	return ReviewReportDTO{
		ReportID:  r.ReportID,
		UserID:    r.UserID,
		UserName:  r.UserName,
		UserEmail: r.UserEmail,
		Reason:    r.Reason,
		Details:   r.Details,
		CreatedAt: r.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

func MapToReviewReportDTOs(rows []sqlc.GetReviewReportsByReviewIDRow) []ReviewReportDTO {
	result := make([]ReviewReportDTO, len(rows))
	for i, r := range rows {
		result[i] = MapToReviewReportDTO(r)
	}
	return result
}
