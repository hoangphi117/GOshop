package v1handler

import (
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/validation"
	"net/http"

	"github.com/gin-gonic/gin"
)

type CategoryHandler struct {
	service v1service.CategoryService
}

func NewCategoryHandler(service v1service.CategoryService) *CategoryHandler {
	return &CategoryHandler{
		service: service,
	}
}

func (ch *CategoryHandler) CreateCategory(ctx *gin.Context) {
	var input v1dto.CreateCategoryInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	category := input.MapCreateInputToModel()

	createdCategory, err := ch.service.CreateCategory(ctx, category)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	createdCategoryDTO := v1dto.MapCategoryToDTO(createdCategory)

	utils.ResponseSuccess(ctx, http.StatusCreated, "Category created successfully", createdCategoryDTO)

}
