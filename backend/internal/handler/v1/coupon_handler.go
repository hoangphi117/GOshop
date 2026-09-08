package v1handler

import (
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/validation"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CouponHandler struct {
	service v1service.CouponService
}

func NewCouponHandler(service v1service.CouponService) *CouponHandler {
	return &CouponHandler{
		service: service,
	}
}

// CreateCoupon godoc
// @Summary      Create coupon
// @Description  Create a new promotional discount coupon (Admin/Moderator)
// @Tags         Admin - Coupons
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        body body v1dto.CreateCouponInput true "Coupon payload"
// @Success      201 {object} utils.APIResponse{data=v1dto.CouponDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/coupons [post]
func (ch *CouponHandler) CreateCoupon(ctx *gin.Context) {
	var input v1dto.CreateCouponInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	coupon, err := ch.service.CreateCoupon(ctx, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated, "Coupon created successfully", coupon)
}

// GetAllCoupons godoc
// @Summary      Get all coupons
// @Description  Retrieve paginated coupons with filters (Admin/Moderator)
// @Tags         Admin - Coupons
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        search query string false "Search code/name"
// @Param        order_by query string false "Order by field"
// @Param        sort query string false "Sort order (asc/desc)"
// @Param        type query int false "Coupon type (1: Percentage, 2: Fixed amount)" Enums(1, 2)
// @Param        is_active query bool false "Filter active state"
// @Param        deleted query bool false "Include deleted coupons"
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(10)
// @Success      200 {object} utils.APIResponse{data=[]v1dto.CouponDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/coupons [get]
func (ch *CouponHandler) GetAllCoupons(ctx *gin.Context) {
	var params v1dto.ListCouponsParam
	if err := ctx.ShouldBindQuery(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	coupons, totalCount, err := ch.service.GetAllCoupons(ctx, params.Search, params.OrderBy, params.Sort, params.Type,
		params.IsActive, params.Page, params.Limit, params.Deleted)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	paginationResponse := utils.NewPaginationResponse(coupons, params.Page, params.Limit, totalCount)

	utils.ResponseSuccess(ctx, http.StatusOK, "Coupons retrieved successfully", paginationResponse)
}

// UpdateCoupon godoc
// @Summary      Update coupon
// @Description  Update coupon details by UUID (Admin/Moderator)
// @Tags         Admin - Coupons
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Coupon UUID"
// @Param        body body v1dto.UpdateCouponInput true "Coupon update payload"
// @Success      200 {object} utils.APIResponse{data=v1dto.CouponDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/coupons/{uuid} [put]
func (ch *CouponHandler) UpdateCoupon(ctx *gin.Context) {
	var params v1dto.GetCouponByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	couponUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var input v1dto.UpdateCouponInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	coupon, err := ch.service.UpdateCoupon(ctx, couponUuid, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Coupon updated successfully", coupon)
}

// SoftDeleteCoupon godoc
// @Summary      Soft delete coupon
// @Description  Soft delete a coupon by UUID (Admin/Moderator)
// @Tags         Admin - Coupons
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Coupon UUID"
// @Success      200 {object} utils.APIResponse{data=v1dto.CouponDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/coupons/{uuid} [delete]
func (ch *CouponHandler) SoftDeleteCoupon(ctx *gin.Context) {
	var params v1dto.GetCouponByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	couponUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	coupon, err := ch.service.SoftDeleteCoupon(ctx, couponUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Coupon soft deleted successfully", coupon)
}

// RestoreCoupon godoc
// @Summary      Restore coupon
// @Description  Restore a soft-deleted coupon by UUID (Admin/Moderator)
// @Tags         Admin - Coupons
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Coupon UUID"
// @Success      200 {object} utils.APIResponse{data=v1dto.CouponDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/coupons/{uuid}/restore [put]
func (ch *CouponHandler) RestoreCoupon(ctx *gin.Context) {
	var params v1dto.GetCouponByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	couponUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	coupon, err := ch.service.RestoreCoupon(ctx, couponUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Coupon restored successfully", coupon)
}

// TrashCoupon godoc
// @Summary      Permanently delete coupon
// @Description  Permanently delete a coupon by UUID (Admin only)
// @Tags         Admin - Coupons
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Coupon UUID"
// @Success      204 "No Content"
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/coupons/{uuid}/trash [delete]
func (ch *CouponHandler) TrashCoupon(ctx *gin.Context) {
	var params v1dto.GetCouponByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	couponUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	_, err = ch.service.TrashCoupon(ctx, couponUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseStatusCode(ctx, http.StatusNoContent)
}

// PreviewCoupon godoc
// @Summary      Preview coupon discount
// @Description  Preview coupon discount calculation for a given cart subtotal
// @Tags         Coupons
// @Accept       json
// @Produce      json
// @Param        body body v1dto.PreviewCouponInput true "Preview payload"
// @Success      200 {object} utils.APIResponse{data=v1dto.PreviewCouponDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Router       /coupons/preview [post]
func (ch *CouponHandler) PreviewCoupon(ctx *gin.Context) {
	var input v1dto.PreviewCouponInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	coupon, err := ch.service.PreviewCoupon(ctx, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Coupon previewed successfully", coupon)
}
