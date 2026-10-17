package handler

import (
	"net/http"
	"strconv"

	"goshop/internal/utils"
	"goshop/services/coupon/internal/dto"
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

func (h *CouponHandler) Preview(c *gin.Context) {
	var input dto.PreviewCouponInput
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

func (h *CouponHandler) Create(c *gin.Context) {
	var input dto.CreateCouponInput
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

func (h *CouponHandler) Update(c *gin.Context) {
	u, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "Invalid UUID"))
		return
	}

	var input dto.UpdateCouponInput
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
