package handler

import (
	"net/http"
	"strconv"
	"strings"

	"goshop/pkg/utils"
	"goshop/pkg/validation"
	catalogdto "goshop/services/catalog/internal/dto"
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

// CreateCategory godoc
// @Summary      Create category
// @Description  Create a new category (Admin/Moderator)
// @Tags         Admin - Categories
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        body body catalogdto.CreateCategoryInput true "Category creation payload"
// @Success      201 {object} utils.APIResponse{data=catalogdto.CategoryDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Router       /admin/categories [post]
func (h *CatalogHandler) CreateCategory(c *gin.Context) {
	var input catalogdto.CreateCategoryInput
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
// @Success      200 {object} utils.APIResponse{data=[]catalogdto.CategoryDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Router       /categories [get]
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

// GetCategoryByUUID godoc
// @Summary      Get category by UUID
// @Description  Retrieve a single category by its UUID
// @Tags         Categories
// @Produce      json
// @Param        uuid path string true "Category UUID"
// @Success      200 {object} utils.APIResponse{data=catalogdto.CategoryDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      404 {object} utils.ErrorResponse
// @Router       /categories/{uuid} [get]
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

// UpdateCategory godoc
// @Summary      Update category
// @Description  Update an existing category by UUID (Admin/Moderator)
// @Tags         Admin - Categories
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Category UUID"
// @Param        body body catalogdto.UpdateCategoryInput true "Category update payload"
// @Success      200 {object} utils.APIResponse{data=catalogdto.CategoryDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      404 {object} utils.ErrorResponse
// @Router       /admin/categories/{uuid} [put]
func (h *CatalogHandler) UpdateCategory(c *gin.Context) {
	uStr := c.Param("uuid")
	var input catalogdto.UpdateCategoryInput
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

// SoftDeleteCategory godoc
// @Summary      Soft delete category
// @Description  Soft delete a category by UUID (Admin/Moderator)
// @Tags         Admin - Categories
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Category UUID"
// @Success      200 {object} utils.APIResponse{data=catalogdto.CategoryDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      404 {object} utils.ErrorResponse
// @Router       /admin/categories/{uuid}/soft-delete [delete]
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

// RestoreCategory godoc
// @Summary      Restore category
// @Description  Restore a soft-deleted category by UUID (Admin/Moderator)
// @Tags         Admin - Categories
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Category UUID"
// @Success      200 {object} utils.APIResponse{data=catalogdto.CategoryDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      404 {object} utils.ErrorResponse
// @Router       /admin/categories/{uuid}/restore [put]
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

// CreateProduct godoc
// @Summary      Create product
// @Description  Create a new product (Admin/Moderator)
// @Tags         Admin - Products
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        request body catalogdto.CreateProductInput true "Product creation payload"
// @Success      201 {object} utils.APIResponse{data=catalogdto.ProductDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /api/v1/admin/products [post]
func (h *CatalogHandler) CreateProduct(c *gin.Context) {
	var input catalogdto.CreateProductInput
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

// GetAllProducts godoc
// @Summary      Get list of products
// @Description  Retrieve paginated products with filtering by category, price, search, and sorting
// @Tags         Products
// @Accept       json
// @Produce      json
// @Param        search query string false "Search keyword"
// @Param        category_uuid query string false "Category UUID"
// @Param        min_price query number false "Minimum price"
// @Param        max_price query number false "Maximum price"
// @Param        status query int false "Product status (1: Active, 2: Inactive)"
// @Param        order_by query string false "Order by field (price, name, created_at)"
// @Param        sort query string false "Sort direction (asc, desc)"
// @Param        page query int false "Page number (default 1)"
// @Param        limit query int false "Page size limit (default 10)"
// @Success      200 {object} utils.APIResponse{data=[]catalogdto.ProductDTO} "Products retrieved successfully"
// @Failure      400 {object} utils.ErrorResponse "Invalid query parameters"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/products [get]
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

// GetProductByUUID godoc
// @Summary      Get product by UUID
// @Description  Retrieve product details by UUID
// @Tags         Products
// @Accept       json
// @Produce      json
// @Param        uuid path string true "Product UUID"
// @Success      200 {object} utils.APIResponse{data=catalogdto.ProductDTO} "Product retrieved successfully"
// @Failure      400 {object} utils.ErrorResponse "Invalid UUID format"
// @Failure      404 {object} utils.ErrorResponse "Product not found"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/products/{uuid} [get]
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

// SearchProducts godoc
// @Summary      Search products with typo-tolerance
// @Description  Fast instant product search using Meilisearch with typo tolerance (< 10ms)
// @Tags         Products
// @Accept       json
// @Produce      json
// @Param        q query string true "Search query keyword"
// @Param        category_uuid query string false "Filter by Category UUID"
// @Param        min_price query number false "Minimum price"
// @Param        max_price query number false "Maximum price"
// @Param        min_rating query number false "Minimum rating (0-5)"
// @Param        sort query string false "Sort option (price:asc, price:desc, created_at:desc, created_at:asc, rating:desc, rating:asc)"
// @Param        page query int false "Page number (default 1)"
// @Param        limit query int false "Page limit (default 20)"
// @Success      200 {object} utils.APIResponse{data=[]catalogdto.ProductDTO} "Products retrieved successfully"
// @Failure      400 {object} utils.ErrorResponse "Invalid parameters"
// @Router       /api/v1/products/search [get]
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

// SuggestProducts godoc
// @Summary      Instant product suggestions (Search-as-you-type)
// @Description  Ultra-fast product auto-complete suggestions (< 5ms)
// @Tags         Products
// @Accept       json
// @Produce      json
// @Param        q query string true "Search prefix/keyword"
// @Param        limit query int false "Maximum suggestions to return (default 8, max 20)"
// @Success      200 {object} utils.APIResponse{data=[]catalogdto.ProductSuggestionDTO} "Suggestions retrieved successfully"
// @Failure      400 {object} utils.ErrorResponse "Invalid query"
// @Router       /api/v1/products/suggest [get]
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

// UpdateProduct godoc
// @Summary      Update product
// @Description  Admin endpoint to update an existing product
// @Tags         Admin - Products
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Security     BearerAuth
// @Param        uuid path string true "Product UUID"
// @Param        request body catalogdto.UpdateProductInput true "Product update payload"
// @Success      200 {object} utils.APIResponse{data=catalogdto.ProductDTO} "Product updated successfully"
// @Failure      400 {object} utils.ErrorResponse "Validation error"
// @Failure      401 {object} utils.ErrorResponse "Unauthorized"
// @Failure      404 {object} utils.ErrorResponse "Product not found"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/admin/products/{uuid} [put]
func (h *CatalogHandler) UpdateProduct(c *gin.Context) {
	uStr := c.Param("uuid")
	var input catalogdto.UpdateProductInput
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

// SoftDeleteProduct godoc
// @Summary      Soft delete product
// @Description  Admin endpoint to mark a product as deleted
// @Tags         Admin - Products
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Security     BearerAuth
// @Param        uuid path string true "Product UUID"
// @Success      200 {object} utils.APIResponse{data=catalogdto.ProductDTO} "Product soft deleted successfully"
// @Failure      400 {object} utils.ErrorResponse "Invalid UUID"
// @Failure      401 {object} utils.ErrorResponse "Unauthorized"
// @Failure      404 {object} utils.ErrorResponse "Product not found"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/admin/products/{uuid} [delete]
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

// RestoreProduct godoc
// @Summary      Restore soft deleted product
// @Description  Admin endpoint to restore a previously deleted product
// @Tags         Admin - Products
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Security     BearerAuth
// @Param        uuid path string true "Product UUID"
// @Success      200 {object} utils.APIResponse{data=catalogdto.ProductDTO} "Product restored successfully"
// @Failure      400 {object} utils.ErrorResponse "Invalid UUID"
// @Failure      401 {object} utils.ErrorResponse "Unauthorized"
// @Failure      404 {object} utils.ErrorResponse "Product not found"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/admin/products/{uuid}/restore [put]
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

// TrashProduct godoc
// @Summary      Permanently delete product
// @Description  Admin endpoint to permanently delete a product from database
// @Tags         Admin - Products
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Security     BearerAuth
// @Param        uuid path string true "Product UUID"
// @Success      204 "Product permanently deleted"
// @Failure      400 {object} utils.ErrorResponse "Invalid UUID"
// @Failure      401 {object} utils.ErrorResponse "Unauthorized"
// @Failure      404 {object} utils.ErrorResponse "Product not found"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/admin/products/{uuid}/trash [delete]
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

// ImportCSV godoc
// @Summary      Import products from CSV
// @Description  Admin endpoint to batch import products from a CSV file (RFC 4180 compliant, atomic all-or-nothing transaction)
// @Tags         Admin - Products
// @Accept       multipart/form-data
// @Produce      json
// @Security     ApiKeyAuth
// @Security     BearerAuth
// @Param        file formData file true "CSV file (name,price,description,imageUrl,category_id,stock,sku)"
// @Success      200 {object} utils.APIResponse{data=catalogdto.ProductCSVImportResponse} "CSV import result report"
// @Failure      400 {object} utils.ErrorResponse "Bad request or validation error"
// @Failure      401 {object} utils.ErrorResponse "Unauthorized"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/admin/products/import-csv [post]
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
