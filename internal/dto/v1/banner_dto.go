package v1dto

import (
	"time"

	"goshop/internal/db/sqlc"
)

// Public DTO for clients (Home page, Category page)
type BannerDTO struct {
	UUID     string  `json:"uuid"`
	Title    string  `json:"title"`
	ImageURL string  `json:"image_url"`
	LinkURL  *string `json:"link_url,omitempty"`
	Position string  `json:"position"`
	Order    int32   `json:"order"`
}

// Admin detailed DTO
type AdminBannerDetailDTO struct {
	UUID      string     `json:"uuid"`
	Title     string     `json:"title"`
	ImageURL  string     `json:"image_url"`
	LinkURL   *string    `json:"link_url,omitempty"`
	Position  string     `json:"position"`
	Order     int32      `json:"order"`
	IsActive  bool       `json:"is_active"`
	StartAt   *time.Time `json:"start_at,omitempty"`
	EndAt     *time.Time `json:"end_at,omitempty"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type CreateBannerRequest struct {
	Title    string     `json:"title" binding:"required,min=2,max=255"`
	ImageURL string     `json:"image_url" binding:"required"`
	LinkURL  *string    `json:"link_url" binding:"omitempty"`
	Position string     `json:"position" binding:"required,oneof=home_main home_sub home_middle home_popup category_top sidebar common"`
	Order    int32      `json:"order" binding:"omitempty,gte=0"`
	IsActive *bool      `json:"is_active" binding:"omitempty"`
	StartAt  *time.Time `json:"start_at" binding:"omitempty"`
	EndAt    *time.Time `json:"end_at" binding:"omitempty"`
}

type UpdateBannerRequest struct {
	Title    *string    `json:"title" binding:"omitempty,min=2,max=255"`
	ImageURL *string    `json:"image_url" binding:"omitempty"`
	LinkURL  *string    `json:"link_url" binding:"omitempty"`
	Position *string    `json:"position" binding:"omitempty,oneof=home_main home_sub home_middle home_popup category_top sidebar common"`
	Order    *int32     `json:"order" binding:"omitempty,gte=0"`
	IsActive *bool      `json:"is_active" binding:"omitempty"`
	StartAt  *time.Time `json:"start_at" binding:"omitempty"`
	EndAt    *time.Time `json:"end_at" binding:"omitempty"`
}

type UpdateBannerStatusRequest struct {
	IsActive *bool `json:"is_active" binding:"required"`
}

type GetBannersQuery struct {
	Position string `form:"position" binding:"omitempty,oneof=home_main home_sub home_middle home_popup category_top sidebar common"`
}

type AdminGetBannersQuery struct {
	Page     int32   `form:"page" binding:"omitempty,gte=1"`
	Limit    int32   `form:"limit" binding:"omitempty,gte=1,lte=100"`
	Position *string `form:"position" binding:"omitempty,oneof=home_main home_sub home_middle home_popup category_top sidebar common"`
	IsActive *bool   `form:"is_active" binding:"omitempty"`
	Deleted  *bool   `form:"deleted" binding:"omitempty"`
}

func MapBannerToDTO(b sqlc.GetActiveBannersByPositionRow) BannerDTO {
	return BannerDTO{
		UUID:     b.BannerUuid.String(),
		Title:    b.BannerTitle,
		ImageURL: b.BannerImageUrl,
		LinkURL:  b.BannerLinkUrl,
		Position: b.BannerPosition,
		Order:    b.BannerOrder,
	}
}

func MapAdminBannerDetailDTO(b sqlc.Banner) AdminBannerDetailDTO {
	var startAt, endAt, deletedAt *time.Time
	if b.BannerStartAt.Valid {
		t := b.BannerStartAt.Time
		startAt = &t
	}
	if b.BannerEndAt.Valid {
		t := b.BannerEndAt.Time
		endAt = &t
	}
	if b.BannerDeletedAt.Valid {
		t := b.BannerDeletedAt.Time
		deletedAt = &t
	}

	return AdminBannerDetailDTO{
		UUID:      b.BannerUuid.String(),
		Title:     b.BannerTitle,
		ImageURL:  b.BannerImageUrl,
		LinkURL:   b.BannerLinkUrl,
		Position:  b.BannerPosition,
		Order:     b.BannerOrder,
		IsActive:  b.BannerIsActive,
		StartAt:   startAt,
		EndAt:     endAt,
		DeletedAt: deletedAt,
		CreatedAt: b.BannerCreatedAt,
		UpdatedAt: b.BannerUpdatedAt,
	}
}
