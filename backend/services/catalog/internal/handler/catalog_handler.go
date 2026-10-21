package handler

import (
	"net/http"
	"strconv"
	"strings"

	"goshop/internal/utils"
	"goshop/pkg/validation"
	"goshop/services/catalog/internal/dto"
	"goshop/services/catalog/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CatalogHandler struct {
	svc service.CatalogService
}

func NewCatalogHandler(svc service.CatalogService) *CatalogHandler {
	return &CatalogHandler{svc: svc}
}

// =============================================================================
// CATEGORIES HANDLERS
// =============================================================================

func (h *CatalogHandler) CreateCategory(c *gin.Context) {
	var input dto.CreateCategoryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(c, validation.HandleValidationErrors(err))
		return
	}
	if input.Name == "" && input.AltName != "" {
		input.Name = input.AltName
	}
	if input.Name == "" {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "category_name is required"))
		return
	}

	cat, err := h.svc.CreateCategory(c.Request.Context(), input)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}

	utils.ResponseSuccess(c, http.StatusCreated, "Category created successfully", cat)
}

func (h *CatalogHandler) GetAllCategories(c *gin.Context) {
	var activeOnly *bool
	if status := c.Query("active_only"); status == "true" {
		t := true
		activeOnly = &t
	}

	cats, err := h.svc.GetAllCategories(c.Request.Context(), activeOnly)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}

	utils.ResponseSuccess(c, http.StatusOK, "Categories retrieved successfully", cats)
}

func (h *CatalogHandler) GetCategoryBySlug(c *gin.Context) {
	param := c.Param("slug")
	if u, err := uuid.Parse(param); err == nil {
		cat, err := h.svc.GetCategoryByUUID(c.Request.Context(), u)
		if err != nil {
			utils.ResponseError(c, err)
			return
		}
		utils.ResponseSuccess(c, http.StatusOK, "Category retrieved successfully", cat)
		return
	}

	cat, err := h.svc.GetCategoryBySlug(c.Request.Context(), param)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}

	utils.ResponseSuccess(c, http.StatusOK, "Category retrieved successfully", cat)
}

func (h *CatalogHandler) GetCategoryByUUID(c *gin.Context) {
	u, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "invalid category UUID"))
		return
	}

	cat, err := h.svc.GetCategoryByUUID(c.Request.Context(), u)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}

	utils.ResponseSuccess(c, http.StatusOK, "Category retrieved successfully", cat)
}

func (h *CatalogHandler) UpdateCategory(c *gin.Context) {
	uStr := c.Param("uuid")
	var input dto.UpdateCategoryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(c, validation.HandleValidationErrors(err))
		return
	}
	input.UUID = uStr
	if input.Name == nil && input.AltName != nil {
		input.Name = input.AltName
	}

	cat, err := h.svc.UpdateCategory(c.Request.Context(), input)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}

	utils.ResponseSuccess(c, http.StatusOK, "Category updated successfully", cat)
}

func (h *CatalogHandler) SoftDeleteCategory(c *gin.Context) {
	u, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "invalid category UUID"))
		return
	}

	cat, err := h.svc.SoftDeleteCategory(c.Request.Context(), u)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}

	utils.ResponseSuccess(c, http.StatusOK, "Category deleted successfully", cat)
}

func (h *CatalogHandler) RestoreCategory(c *gin.Context) {
	u, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "invalid category UUID"))
		return
	}

	cat, err := h.svc.RestoreCategory(c.Request.Context(), u)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}

	utils.ResponseSuccess(c, http.StatusOK, "Category restored successfully", cat)
}

func (h *CatalogHandler) TrashCategory(c *gin.Context) {
	u, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "invalid category UUID"))
		return
	}

	if err := h.svc.TrashCategory(c.Request.Context(), u); err != nil {
		utils.ResponseError(c, err)
		return
	}

	utils.ResponseSuccess(c, http.StatusOK, "Category permanently deleted", nil)
}

// =============================================================================
// PRODUCTS HANDLERS
// =============================================================================

func (h *CatalogHandler) CreateProduct(c *gin.Context) {
	var input dto.CreateProductInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(c, validation.HandleValidationErrors(err))
		return
	}
	if input.CategoryUUID == "" && input.AltCatUUID != "" {
		input.CategoryUUID = input.AltCatUUID
	}
	if input.ImageURL == nil && input.AltImageURL != nil {
		input.ImageURL = input.AltImageURL
	}
	if input.CategoryUUID == "" {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "category_uuid is required"))
		return
	}

	prod, err := h.svc.CreateProduct(c.Request.Context(), input)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}

	utils.ResponseSuccess(c, http.StatusCreated, "Product created successfully", prod)
}

func (h *CatalogHandler) GetAllProducts(c *gin.Context) {
	searchStr := c.Query("search")
	categoryUUID := c.Query("category_uuid")
	minPrice, _ := strconv.ParseFloat(c.Query("min_price"), 64)
	maxPrice, _ := strconv.ParseFloat(c.Query("max_price"), 64)
	minRating, _ := strconv.ParseFloat(c.Query("min_rating"), 64)
	status, _ := strconv.Atoi(c.Query("status"))
	orderBy := c.Query("order_by")
	sort := c.Query("sort")
	page, _ := strconv.Atoi(c.Query("page"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	deleted := c.Query("deleted") == "true"

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	products, total, err := h.svc.GetAllProducts(
		c.Request.Context(),
		searchStr, categoryUUID,
		minPrice, maxPrice, minRating,
		int32(status),
		orderBy, sort,
		int32(page), int32(limit),
		deleted,
	)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}

	paginationResponse := utils.NewPaginationResponse(products, int32(page), int32(limit), total)
	utils.ResponseSuccess(c, http.StatusOK, "Products retrieved successfully", paginationResponse)
}

func (h *CatalogHandler) GetProductBySlug(c *gin.Context) {
	param := c.Param("slug")
	if u, err := uuid.Parse(param); err == nil {
		prod, err := h.svc.GetProductByUUID(c.Request.Context(), u)
		if err != nil {
			utils.ResponseError(c, err)
			return
		}
		utils.ResponseSuccess(c, http.StatusOK, "Product retrieved successfully", prod)
		return
	}

	prod, err := h.svc.GetProductBySlug(c.Request.Context(), param)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}

	utils.ResponseSuccess(c, http.StatusOK, "Product retrieved successfully", prod)
}

func (h *CatalogHandler) GetProductByUUID(c *gin.Context) {
	u, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "invalid product UUID"))
		return
	}

	prod, err := h.svc.GetProductByUUID(c.Request.Context(), u)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}

	utils.ResponseSuccess(c, http.StatusOK, "Product retrieved successfully", prod)
}

func (h *CatalogHandler) SearchProducts(c *gin.Context) {
	q := c.Query("q")
	categoryUUID := c.Query("category_uuid")
	minPrice, _ := strconv.ParseFloat(c.Query("min_price"), 64)
	maxPrice, _ := strconv.ParseFloat(c.Query("max_price"), 64)
	minRating, _ := strconv.ParseFloat(c.Query("min_rating"), 64)
	sort := c.Query("sort")
	page, _ := strconv.Atoi(c.Query("page"))
	limit, _ := strconv.Atoi(c.Query("limit"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	products, total, err := h.svc.SearchProducts(
		c.Request.Context(),
		q, categoryUUID,
		minPrice, maxPrice, minRating,
		sort,
		int32(page), int32(limit),
	)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}

	paginationResponse := utils.NewPaginationResponse(products, int32(page), int32(limit), int32(total))
	utils.ResponseSuccess(c, http.StatusOK, "Products searched successfully", paginationResponse)
}

func (h *CatalogHandler) SuggestProducts(c *gin.Context) {
	q := c.Query("q")
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 || limit > 20 {
		limit = 8
	}

	suggestions, err := h.svc.SuggestProducts(c.Request.Context(), q, int32(limit))
	if err != nil {
		utils.ResponseError(c, err)
		return
	}

	utils.ResponseSuccess(c, http.StatusOK, "Suggestions retrieved successfully", suggestions)
}

func (h *CatalogHandler) UpdateProduct(c *gin.Context) {
	uStr := c.Param("uuid")
	var input dto.UpdateProductInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(c, validation.HandleValidationErrors(err))
		return
	}
	input.UUID = uStr
	if input.CategoryUUID == nil && input.AltCatUUID != nil {
		input.CategoryUUID = input.AltCatUUID
	}
	if input.ImageURL == nil && input.AltImageURL != nil {
		input.ImageURL = input.AltImageURL
	}

	prod, err := h.svc.UpdateProduct(c.Request.Context(), input)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}

	utils.ResponseSuccess(c, http.StatusOK, "Product updated successfully", prod)
}

func (h *CatalogHandler) SoftDeleteProduct(c *gin.Context) {
	u, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "invalid product UUID"))
		return
	}

	prod, err := h.svc.SoftDeleteProduct(c.Request.Context(), u)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}

	utils.ResponseSuccess(c, http.StatusOK, "Product deleted successfully", prod)
}

func (h *CatalogHandler) RestoreProduct(c *gin.Context) {
	u, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "invalid product UUID"))
		return
	}

	prod, err := h.svc.RestoreProduct(c.Request.Context(), u)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}

	utils.ResponseSuccess(c, http.StatusOK, "Product restored successfully", prod)
}

func (h *CatalogHandler) TrashProduct(c *gin.Context) {
	u, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "invalid product UUID"))
		return
	}

	if err := h.svc.TrashProduct(c.Request.Context(), u); err != nil {
		utils.ResponseError(c, err)
		return
	}

	utils.ResponseSuccess(c, http.StatusOK, "Product permanently deleted", nil)
}

func (h *CatalogHandler) ImportProductsCSV(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "CSV file is required"))
		return
	}

	if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".csv") {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "Only .csv files are supported"))
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "Failed to read uploaded file"))
		return
	}
	defer func() { _ = file.Close() }()

	res, err := h.svc.ImportProductsCSV(c.Request.Context(), file)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}

	utils.ResponseSuccess(c, http.StatusOK, "CSV import processed", res)
}
