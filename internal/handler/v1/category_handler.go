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

func (ch *CategoryHandler) GetAllCategories(ctx *gin.Context) {
	var params v1dto.GetCategoryParam
	if err := ctx.ShouldBindQuery(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	listCategories, total, err := ch.service.GetAllCategories(ctx, params.Search, params.OrderBy, params.Sort, params.Page, params.Limit, false)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	listCategoriesDTO := v1dto.MapListCategoriesToDTO(listCategories)

	paginationResponse := utils.NewPaginationResponse(listCategoriesDTO, params.Page, params.Limit, total)

	utils.ResponseSuccess(ctx, http.StatusOK, "Categories retrieved successfully", paginationResponse)
}

func (ch *CategoryHandler) GetCategoryByUUID(ctx *gin.Context) {
	var params v1dto.GetCategoryByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	categoryUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	category, err := ch.service.GetCategoryByUUID(ctx, categoryUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	categoryDTO := v1dto.MapCategoryToDTO(category)
	utils.ResponseSuccess(ctx, http.StatusOK, "Category retrieved successfully", categoryDTO)
}

func (ch *CategoryHandler) UpdateCategory(ctx *gin.Context) {
	var params v1dto.GetCategoryByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	categoryUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var input v1dto.UpdateCategoryInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	category := input.MapUpdateInputToModel(categoryUuid)

	updatedCategory, err := ch.service.UpdateCategory(ctx, category)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	updatedCategoryDTO := v1dto.MapCategoryToDTO(updatedCategory)
	utils.ResponseSuccess(ctx, http.StatusOK, "Category updated successfully", updatedCategoryDTO)
}

func (ch *CategoryHandler) SoftDeleteCategory(ctx *gin.Context) {
	var params v1dto.GetCategoryByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	categoryUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	softDeletedCategory, err := ch.service.SoftDeleteCategory(ctx, categoryUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	softDeletedCategoryDTO := v1dto.MapCategoryToDTO(softDeletedCategory)
	utils.ResponseSuccess(ctx, http.StatusOK, "Category soft deleted successfully", softDeletedCategoryDTO)
}

func (ch *CategoryHandler) RestoreCategory(ctx *gin.Context) {
	var params v1dto.GetCategoryByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	categoryUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	restoredCategory, err := ch.service.RestoreCategory(ctx, categoryUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	restoredCategoryDTO := v1dto.MapCategoryToDTO(restoredCategory)
	utils.ResponseSuccess(ctx, http.StatusOK, "Category restored successfully", restoredCategoryDTO)
}

func (ch *CategoryHandler) TrashCategory(ctx *gin.Context) {
	var params v1dto.GetCategoryByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	categoryUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	err = ch.service.TrashCategory(ctx, categoryUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseStatusCode(ctx, http.StatusNoContent)
}
