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
