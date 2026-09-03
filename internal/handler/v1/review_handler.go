package v1handler

import (
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/validation"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ReviewHandler struct {
	service v1service.ReviewService
}

func NewReviewHandler(service v1service.ReviewService) *ReviewHandler {
	return &ReviewHandler{
		service: service,
	}
}

func (rh *ReviewHandler) CreateReview(ctx *gin.Context) {
	var input v1dto.CreateReviewInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	review, err := rh.service.CreateReview(ctx, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated, "Review created successfully", review)
}
