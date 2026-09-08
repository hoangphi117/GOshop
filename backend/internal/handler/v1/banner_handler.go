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

// GET /api/v1/banners?position=home_main
func (h *BannerHandler) GetActiveBanners(ctx *gin.Context) {
	var query v1dto.GetBannersQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	banners, err := h.service.GetActiveBanners(ctx, query.Position)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Active banners retrieved successfully", banners)
}

// GET /api/v1/admin/banners
func (h *BannerHandler) AdminGetBanners(ctx *gin.Context) {
	var query v1dto.AdminGetBannersQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	banners, total, err := h.service.AdminGetBanners(ctx, query)
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

// GET /api/v1/admin/banners/:uuid
func (h *BannerHandler) AdminGetBannerByUUID(ctx *gin.Context) {
	bannerUUID, err := uuid.Parse(ctx.Param("uuid"))
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid banner UUID"))
		return
	}

	banner, err := h.service.AdminGetBannerByUUID(ctx, bannerUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Banner retrieved successfully", banner)
}

// POST /api/v1/admin/banners
func (h *BannerHandler) AdminCreateBanner(ctx *gin.Context) {
	var input v1dto.CreateBannerRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	banner, err := h.service.AdminCreateBanner(ctx, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated, "Banner created successfully", banner)
}

// PUT /api/v1/admin/banners/:uuid
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

	banner, err := h.service.AdminUpdateBanner(ctx, bannerUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Banner updated successfully", banner)
}

// PATCH /api/v1/admin/banners/:uuid/status
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

	banner, err := h.service.AdminUpdateBannerStatus(ctx, bannerUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Banner status updated successfully", banner)
}

// DELETE /api/v1/admin/banners/:uuid
func (h *BannerHandler) AdminSoftDeleteBanner(ctx *gin.Context) {
	bannerUUID, err := uuid.Parse(ctx.Param("uuid"))
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid banner UUID"))
		return
	}

	if err := h.service.AdminSoftDeleteBanner(ctx, bannerUUID); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Banner moved to trash successfully", nil)
}

// PUT /api/v1/admin/banners/:uuid/restore
func (h *BannerHandler) AdminRestoreBanner(ctx *gin.Context) {
	bannerUUID, err := uuid.Parse(ctx.Param("uuid"))
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid banner UUID"))
		return
	}

	banner, err := h.service.AdminRestoreBanner(ctx, bannerUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Banner restored successfully", banner)
}

// DELETE /api/v1/admin/banners/:uuid/trash
func (h *BannerHandler) AdminTrashBanner(ctx *gin.Context) {
	bannerUUID, err := uuid.Parse(ctx.Param("uuid"))
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid banner UUID"))
		return
	}

	if err := h.service.AdminTrashBanner(ctx, bannerUUID); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Banner permanently deleted successfully", nil)
}
