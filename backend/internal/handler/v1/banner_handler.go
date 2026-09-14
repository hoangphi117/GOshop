package v1handler

import (
	"net/http"

	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/validation"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type BannerHandler struct {
	service v1service.BannerService
}

func NewBannerHandler(service v1service.BannerService) *BannerHandler {
	return &BannerHandler{service: service}
}

// GetActiveBanners godoc
// @Summary      Get active banners
// @Description  Get active banners for public display (e.g., home page)
// @Tags         Banners
// @Produce      json
// @Param        position query string false "Banner position" Enums(home_main, home_sub, home_middle, home_popup, category_top, sidebar, common)
// @Success      200 {object} utils.APIResponse{data=[]v1dto.BannerDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Router       /banners [get]
func (h *BannerHandler) GetActiveBanners(ctx *gin.Context) {
	var query v1dto.GetBannersQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	banners, err := h.service.GetActiveBanners(ctx.Request.Context(), query.Position)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Active banners retrieved successfully", banners)
}

// AdminGetBanners godoc
// @Summary      Get all banners
// @Description  Retrieve paginated banners with filters (Admin/Moderator)
// @Tags         Admin - Banners
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        position query string false "Filter by position"
// @Param        is_active query bool false "Filter active state"
// @Param        deleted query bool false "Include soft-deleted banners"
// @Param        search query string false "Search title"
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(10)
// @Success      200 {object} utils.APIResponse{data=[]v1dto.AdminBannerDetailDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/banners [get]
func (h *BannerHandler) AdminGetBanners(ctx *gin.Context) {
	var query v1dto.AdminGetBannersQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	banners, total, err := h.service.AdminGetBanners(ctx.Request.Context(), query)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	page := query.Page
	if page <= 0 {
		page = 1
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 10
	}

	pagination := utils.NewPaginationResponse(banners, page, limit, int32(total))
	utils.ResponseSuccess(ctx, http.StatusOK, "Banners retrieved successfully", pagination)
}

// AdminGetBannerByUUID godoc
// @Summary      Get banner detail
// @Description  Retrieve banner by UUID (Admin/Moderator)
// @Tags         Admin - Banners
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Banner UUID"
// @Success      200 {object} utils.APIResponse{data=v1dto.AdminBannerDetailDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/banners/{uuid} [get]
func (h *BannerHandler) AdminGetBannerByUUID(ctx *gin.Context) {
	bannerUUID, err := uuid.Parse(ctx.Param("uuid"))
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid banner UUID"))
		return
	}

	banner, err := h.service.AdminGetBannerByUUID(ctx.Request.Context(), bannerUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Banner retrieved successfully", banner)
}

// AdminCreateBanner godoc
// @Summary      Create banner
// @Description  Create a new banner (Admin/Moderator)
// @Tags         Admin - Banners
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        body body v1dto.CreateBannerRequest true "Banner payload"
// @Success      201 {object} utils.APIResponse{data=v1dto.AdminBannerDetailDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/banners [post]
func (h *BannerHandler) AdminCreateBanner(ctx *gin.Context) {
	var input v1dto.CreateBannerRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	banner, err := h.service.AdminCreateBanner(ctx.Request.Context(), input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated, "Banner created successfully", banner)
}

// AdminUpdateBanner godoc
// @Summary      Update banner
// @Description  Update banner details by UUID (Admin/Moderator)
// @Tags         Admin - Banners
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Banner UUID"
// @Param        body body v1dto.UpdateBannerRequest true "Banner update payload"
// @Success      200 {object} utils.APIResponse{data=v1dto.AdminBannerDetailDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/banners/{uuid} [put]
func (h *BannerHandler) AdminUpdateBanner(ctx *gin.Context) {
	bannerUUID, err := uuid.Parse(ctx.Param("uuid"))
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid banner UUID"))
		return
	}

	var input v1dto.UpdateBannerRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	banner, err := h.service.AdminUpdateBanner(ctx.Request.Context(), bannerUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Banner updated successfully", banner)
}

// AdminUpdateBannerStatus godoc
// @Summary      Update banner status
// @Description  Enable or disable banner active status (Admin/Moderator)
// @Tags         Admin - Banners
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Banner UUID"
// @Param        body body v1dto.UpdateBannerStatusRequest true "Status payload"
// @Success      200 {object} utils.APIResponse{data=v1dto.AdminBannerDetailDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/banners/{uuid}/status [patch]
func (h *BannerHandler) AdminUpdateBannerStatus(ctx *gin.Context) {
	bannerUUID, err := uuid.Parse(ctx.Param("uuid"))
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid banner UUID"))
		return
	}

	var input v1dto.UpdateBannerStatusRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	banner, err := h.service.AdminUpdateBannerStatus(ctx.Request.Context(), bannerUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Banner status updated successfully", banner)
}

// AdminSoftDeleteBanner godoc
// @Summary      Soft delete banner
// @Description  Move banner to trash (Admin/Moderator)
// @Tags         Admin - Banners
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Banner UUID"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/banners/{uuid} [delete]
func (h *BannerHandler) AdminSoftDeleteBanner(ctx *gin.Context) {
	bannerUUID, err := uuid.Parse(ctx.Param("uuid"))
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid banner UUID"))
		return
	}

	if err := h.service.AdminSoftDeleteBanner(ctx.Request.Context(), bannerUUID); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Banner moved to trash successfully", nil)
}

// AdminRestoreBanner godoc
// @Summary      Restore banner
// @Description  Restore a soft-deleted banner (Admin/Moderator)
// @Tags         Admin - Banners
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Banner UUID"
// @Success      200 {object} utils.APIResponse{data=v1dto.AdminBannerDetailDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/banners/{uuid}/restore [put]
func (h *BannerHandler) AdminRestoreBanner(ctx *gin.Context) {
	bannerUUID, err := uuid.Parse(ctx.Param("uuid"))
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid banner UUID"))
		return
	}

	banner, err := h.service.AdminRestoreBanner(ctx.Request.Context(), bannerUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Banner restored successfully", banner)
}

// AdminTrashBanner godoc
// @Summary      Permanently delete banner
// @Description  Permanently delete a banner from database (Admin/Moderator)
// @Tags         Admin - Banners
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Banner UUID"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/banners/{uuid}/trash [delete]
func (h *BannerHandler) AdminTrashBanner(ctx *gin.Context) {
	bannerUUID, err := uuid.Parse(ctx.Param("uuid"))
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid banner UUID"))
		return
	}

	if err := h.service.AdminTrashBanner(ctx.Request.Context(), bannerUUID); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Banner permanently deleted successfully", nil)
}
