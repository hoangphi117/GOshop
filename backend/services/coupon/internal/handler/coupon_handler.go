package handler

import (
	"net/http"
	"strconv"

	"goshop/pkg/utils"
	coupondto "goshop/services/coupon/internal/dto"
	"goshop/services/coupon/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CouponHandler struct {
	svc service.CouponService
}

func NewCouponHandler(svc service.CouponService) *CouponHandler {
	return &CouponHandler{svc: svc}
}

// PreviewCoupon godoc
// @Summary      Preview coupon discount
// @Description  Preview coupon discount calculation for a given cart subtotal
// @Tags         Coupons
// @Accept       json
// @Produce      json
// @Param        body body coupondto.PreviewCouponInput true "Preview payload"
// @Success      200 {object} utils.APIResponse{data=coupondto.PreviewCouponDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Router       /coupons/preview [post]
func (h *CouponHandler) Preview(c *gin.Context) {
	var input coupondto.PreviewCouponInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, err.Error()))
		return
	}

	result, err := h.svc.PreviewCoupon(c.Request.Context(), input)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}

	utils.ResponseSuccess(c, http.StatusOK, "Preview coupon successful", result)
}

func (h *CouponHandler) GetByCode(c *gin.Context) {
	code := c.Param("code")
	result, err := h.svc.GetByCode(c.Request.Context(), code)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}
	utils.ResponseSuccess(c, http.StatusOK, "Get coupon successful", result)
}

// GetByUUID godoc
// @Summary      Get coupon by UUID
// @Description  Retrieve a single coupon by its UUID (Admin)
// @Tags         Admin - Coupons
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Coupon UUID"
// @Success      200 {object} utils.APIResponse{data=coupondto.CouponDTO}
// @Failure      404 {object} utils.ErrorResponse
// @Router       /admin/coupons/{uuid} [get]
func (h *CouponHandler) GetByUUID(c *gin.Context) {
	u, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "Invalid UUID"))
		return
	}

	result, err := h.svc.GetByUUID(c.Request.Context(), u)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}
	utils.ResponseSuccess(c, http.StatusOK, "Get coupon successful", result)
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
// @Success      200 {object} utils.APIResponse{data=[]coupondto.CouponDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/coupons [get]
func (h *CouponHandler) List(c *gin.Context) {
	var search *string
	if s := c.Query("search"); s != "" {
		search = &s
	}

	var couponType *int32
	if t := c.Query("type"); t != "" {
		if val, err := strconv.Atoi(t); err == nil {
			v := int32(val)
			couponType = &v
		}
	}

	var isActive *bool
	if a := c.Query("is_active"); a != "" {
		val := a == "true" || a == "1"
		isActive = &val
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	coupons, total, err := h.svc.List(c.Request.Context(), search, couponType, isActive, int32(limit), int32(offset))
	if err != nil {
		utils.ResponseError(c, err)
		return
	}

	paginationResponse := utils.NewPaginationResponse(coupons, int32(page), int32(limit), int32(total))
	utils.ResponseSuccess(c, http.StatusOK, "Coupons retrieved successfully", paginationResponse)
}

// CreateCoupon godoc
// @Summary      Create coupon
// @Description  Create a new promotional discount coupon (Admin/Moderator)
// @Tags         Admin - Coupons
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        body body coupondto.CreateCouponInput true "Coupon payload"
// @Success      201 {object} utils.APIResponse{data=coupondto.CouponDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/coupons [post]
func (h *CouponHandler) Create(c *gin.Context) {
	var input coupondto.CreateCouponInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, err.Error()))
		return
	}

	result, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}
	utils.ResponseSuccess(c, http.StatusCreated, "Create coupon successful", result)
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
// @Param        body body coupondto.UpdateCouponInput true "Coupon update payload"
// @Success      200 {object} utils.APIResponse{data=coupondto.CouponDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/coupons/{uuid} [put]
func (h *CouponHandler) Update(c *gin.Context) {
	u, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "Invalid UUID"))
		return
	}

	var input coupondto.UpdateCouponInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, err.Error()))
		return
	}

	result, err := h.svc.Update(c.Request.Context(), u, input)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}
	utils.ResponseSuccess(c, http.StatusOK, "Update coupon successful", result)
}

// SoftDeleteCoupon godoc
// @Summary      Soft delete coupon
// @Description  Soft delete a coupon by UUID (Admin/Moderator)
// @Tags         Admin - Coupons
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Coupon UUID"
// @Success      200 {object} utils.APIResponse{data=coupondto.CouponDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/coupons/{uuid} [delete]
func (h *CouponHandler) Delete(c *gin.Context) {
	u, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "Invalid UUID"))
		return
	}

	if err := h.svc.Delete(c.Request.Context(), u); err != nil {
		utils.ResponseError(c, err)
		return
	}
	utils.ResponseSuccess(c, http.StatusOK, "Delete coupon successful", gin.H{"deleted": true})
}
