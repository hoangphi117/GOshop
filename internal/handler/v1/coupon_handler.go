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
