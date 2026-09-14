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

// CreateCategory godoc
// @Summary      Create category
// @Description  Create a new category (Admin/Moderator)
// @Tags         Admin - Categories
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        body body v1dto.CreateCategoryInput true "Category creation payload"
// @Success      201 {object} utils.APIResponse{data=v1dto.CategoryDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Router       /admin/categories [post]
func (ch *CategoryHandler) CreateCategory(ctx *gin.Context) {
	var input v1dto.CreateCategoryInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	category := input.MapCreateInputToModel()

	createdCategory, err := ch.service.CreateCategory(ctx.Request.Context(), category)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	createdCategoryDTO := v1dto.MapCategoryToDTO(createdCategory)

	utils.ResponseSuccess(ctx, http.StatusCreated, "Category created successfully", createdCategoryDTO)

}

// GetAllCategories godoc
// @Summary      Get all categories
// @Description  Get paginated list of categories
// @Tags         Categories
// @Produce      json
// @Param        search query string false "Search keyword"
// @Param        order_by query string false "Order by field"
// @Param        sort query string false "Sort direction (asc/desc)"
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(10)
// @Success      200 {object} utils.APIResponse{data=[]v1dto.CategoryDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Router       /categories [get]
func (ch *CategoryHandler) GetAllCategories(ctx *gin.Context) {
	var params v1dto.GetCategoryParam
	if err := ctx.ShouldBindQuery(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	listCategories, total, err := ch.service.GetAllCategories(ctx.Request.Context(), params.Search, params.OrderBy, params.Sort, params.Page, params.Limit, false)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	listCategoriesDTO := v1dto.MapListCategoriesToDTO(listCategories)

	paginationResponse := utils.NewPaginationResponse(listCategoriesDTO, params.Page, params.Limit, total)

	utils.ResponseSuccess(ctx, http.StatusOK, "Categories retrieved successfully", paginationResponse)
}

// GetCategoryByUUID godoc
// @Summary      Get category by UUID
// @Description  Retrieve a single category by its UUID
// @Tags         Categories
// @Produce      json
// @Param        uuid path string true "Category UUID"
// @Success      200 {object} utils.APIResponse{data=v1dto.CategoryDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      404 {object} utils.ErrorResponse
// @Router       /categories/{uuid} [get]
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

	category, err := ch.service.GetCategoryByUUID(ctx.Request.Context(), categoryUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	categoryDTO := v1dto.MapCategoryToDTO(category)
	utils.ResponseSuccess(ctx, http.StatusOK, "Category retrieved successfully", categoryDTO)
}

// UpdateCategory godoc
// @Summary      Update category
// @Description  Update an existing category by UUID (Admin/Moderator)
// @Tags         Admin - Categories
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Category UUID"
// @Param        body body v1dto.UpdateCategoryInput true "Category update payload"
// @Success      200 {object} utils.APIResponse{data=v1dto.CategoryDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      404 {object} utils.ErrorResponse
// @Router       /admin/categories/{uuid} [put]
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

	updatedCategory, err := ch.service.UpdateCategory(ctx.Request.Context(), category)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	updatedCategoryDTO := v1dto.MapCategoryToDTO(updatedCategory)
	utils.ResponseSuccess(ctx, http.StatusOK, "Category updated successfully", updatedCategoryDTO)
}

// SoftDeleteCategory godoc
// @Summary      Soft delete category
// @Description  Soft delete a category by UUID (Admin/Moderator)
// @Tags         Admin - Categories
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Category UUID"
// @Success      200 {object} utils.APIResponse{data=v1dto.CategoryDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      404 {object} utils.ErrorResponse
// @Router       /admin/categories/{uuid}/soft-delete [delete]
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

	softDeletedCategory, err := ch.service.SoftDeleteCategory(ctx.Request.Context(), categoryUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	softDeletedCategoryDTO := v1dto.MapCategoryToDTO(softDeletedCategory)
	utils.ResponseSuccess(ctx, http.StatusOK, "Category soft deleted successfully", softDeletedCategoryDTO)
}

// RestoreCategory godoc
// @Summary      Restore category
// @Description  Restore a soft-deleted category by UUID (Admin/Moderator)
// @Tags         Admin - Categories
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Category UUID"
// @Success      200 {object} utils.APIResponse{data=v1dto.CategoryDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      404 {object} utils.ErrorResponse
// @Router       /admin/categories/{uuid}/restore [put]
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

	restoredCategory, err := ch.service.RestoreCategory(ctx.Request.Context(), categoryUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	restoredCategoryDTO := v1dto.MapCategoryToDTO(restoredCategory)
	utils.ResponseSuccess(ctx, http.StatusOK, "Category restored successfully", restoredCategoryDTO)
}

// TrashCategory godoc
// @Summary      Permanently delete category
// @Description  Permanently delete (hard delete) a category by UUID (Admin only)
// @Tags         Admin - Categories
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Category UUID"
// @Success      204 "No Content"
// @Failure      400 {object} utils.ErrorResponse
// @Failure      404 {object} utils.ErrorResponse
// @Router       /admin/categories/{uuid}/trash [delete]
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

	err = ch.service.TrashCategory(ctx.Request.Context(), categoryUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseStatusCode(ctx, http.StatusNoContent)
}
