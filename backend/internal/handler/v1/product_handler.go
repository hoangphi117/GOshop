package v1handler

import (
	"fmt"
	"net/http"
	"strings"

	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/validation"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ProductHandler struct {
	service      v1service.ProductService
	queryService v1service.ProductQueryService
	adminService v1service.AdminProductService
}

func NewProductHandler(service v1service.ProductService) *ProductHandler {
	return &ProductHandler{
		service:      service,
		queryService: service,
		adminService: service,
	}
}

func (ph *ProductHandler) WithQueryService(queryService v1service.ProductQueryService) *ProductHandler {
	if queryService != nil {
		ph.queryService = queryService
	}
	return ph
}

func (ph *ProductHandler) WithAdminService(adminService v1service.AdminProductService) *ProductHandler {
	if adminService != nil {
		ph.adminService = adminService
	}
	return ph
}

func (ph *ProductHandler) getQueryService() v1service.ProductQueryService {
	if ph.queryService != nil {
		return ph.queryService
	}
	return ph.service
}

func (ph *ProductHandler) getAdminService() v1service.AdminProductService {
	if ph.adminService != nil {
		return ph.adminService
	}
	return ph.service
}

// CreateProduct godoc
// @Summary      Create a new product
// @Description  Admin endpoint to create a new product catalog item
// @Tags         Admin - Products
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Security     BearerAuth
// @Param        request body v1dto.CreateProductRequest true "Product creation payload"
// @Success      201 {object} utils.APIResponse{data=v1dto.ProductDTO} "Created product successfully"
// @Failure      400 {object} utils.ErrorResponse "Validation error"
// @Failure      401 {object} utils.ErrorResponse "Unauthorized"
// @Failure      403 {object} utils.ErrorResponse "Forbidden"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/admin/products [post]
func (ph *ProductHandler) CreateProduct(ctx *gin.Context) {
	var input v1dto.CreateProductRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	mappedInput := input.MapCreateToServiceInput()

	product, err := ph.getAdminService().CreateProduct(ctx.Request.Context(), mappedInput)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	productDTO := v1dto.MapCreateProductRowToDTO(product)

	utils.ResponseSuccess(ctx, http.StatusCreated, "Created product successfully", productDTO)
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
// @Success      200 {object} utils.APIResponse{data=[]v1dto.ProductDTO} "Products retrieved successfully"
// @Failure      400 {object} utils.ErrorResponse "Invalid query parameters"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/products [get]
func (ph *ProductHandler) GetAllProducts(ctx *gin.Context) {
	var params v1dto.GetProductsParam
	if err := ctx.ShouldBindQuery(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	listProducts, total, err := ph.getQueryService().GetAllProducts(ctx.Request.Context(), params.Search, params.CategoryUUID, params.MinPrice, params.MaxPrice, params.MinRating, params.Status, params.OrderBy, params.Sort, params.Page, params.Limit, false)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	listProductsDTO := v1dto.MapListProductsToDTO(listProducts)

	paginationResponse := utils.NewPaginationResponse(listProductsDTO, params.Page, params.Limit, total)

	utils.ResponseSuccess(ctx, http.StatusOK, "Products retrieved successfully", paginationResponse)
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
// @Success      200 {object} utils.APIResponse{data=[]v1dto.ProductDTO} "Products retrieved successfully"
// @Failure      400 {object} utils.ErrorResponse "Invalid parameters"
// @Router       /api/v1/products/search [get]
func (ph *ProductHandler) SearchProducts(ctx *gin.Context) {
	var params v1dto.SearchProductsParam
	if err := ctx.ShouldBindQuery(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	products, total, err := ph.getQueryService().SearchProducts(
		ctx.Request.Context(),
		params.Q,
		params.CategoryUUID,
		params.MinPrice,
		params.MaxPrice,
		params.MinRating,
		params.Sort,
		params.Page,
		params.Limit,
	)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit < 1 {
		params.Limit = 20
	}

	paginationResponse := utils.NewPaginationResponse(products, params.Page, params.Limit, int32(total))
	utils.ResponseSuccess(ctx, http.StatusOK, "Products search completed successfully", paginationResponse)
}

// SuggestProducts godoc
// @Summary      Instant product suggestions (Search-as-you-type)
// @Description  Ultra-fast product auto-complete suggestions (< 5ms)
// @Tags         Products
// @Accept       json
// @Produce      json
// @Param        q query string true "Search prefix/keyword"
// @Param        limit query int false "Maximum suggestions to return (default 8, max 20)"
// @Success      200 {object} utils.APIResponse{data=[]v1dto.ProductSuggestionDTO} "Suggestions retrieved successfully"
// @Failure      400 {object} utils.ErrorResponse "Invalid query"
// @Router       /api/v1/products/suggest [get]
func (ph *ProductHandler) SuggestProducts(ctx *gin.Context) {
	var params v1dto.SuggestProductsParam
	if err := ctx.ShouldBindQuery(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	suggestions, err := ph.getQueryService().SuggestProducts(ctx.Request.Context(), params.Q, params.Limit)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Product suggestions retrieved successfully", suggestions)
}

// GetProductByUUID godoc
// @Summary      Get product by UUID
// @Description  Retrieve product details by UUID
// @Tags         Products
// @Accept       json
// @Produce      json
// @Param        uuid path string true "Product UUID"
// @Success      200 {object} utils.APIResponse{data=v1dto.ProductDTO} "Product retrieved successfully"
// @Failure      400 {object} utils.ErrorResponse "Invalid UUID format"
// @Failure      404 {object} utils.ErrorResponse "Product not found"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/products/{uuid} [get]
func (ph *ProductHandler) GetProductByUUID(ctx *gin.Context) {
	var params v1dto.GetProductByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	productUuid, err := uuid.Parse(params.Uuid)
	if err == nil {
		product, err := ph.getQueryService().GetProductByUUID(ctx.Request.Context(), productUuid)
		if err != nil {
			utils.ResponseError(ctx, err)
			return
		}
		productDTO := v1dto.MapGetProductByUUIDRowToDTO(product)
		utils.ResponseSuccess(ctx, http.StatusOK, "Product retrieved successfully", productDTO)
		return
	}

	// Fallback to query by slug when identifier is not a valid UUID
	product, err := ph.getQueryService().GetProductBySlug(ctx.Request.Context(), params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	productDTO := v1dto.MapGetProductBySlugRowToDTO(product)
	utils.ResponseSuccess(ctx, http.StatusOK, "Product retrieved successfully", productDTO)
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
// @Param        request body v1dto.UpdateProductRequest true "Product update payload"
// @Success      200 {object} utils.APIResponse{data=v1dto.ProductDTO} "Product updated successfully"
// @Failure      400 {object} utils.ErrorResponse "Validation error"
// @Failure      401 {object} utils.ErrorResponse "Unauthorized"
// @Failure      404 {object} utils.ErrorResponse "Product not found"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/admin/products/{uuid} [put]
func (ph *ProductHandler) UpdateProduct(ctx *gin.Context) {

	var params v1dto.GetProductByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	productUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var input v1dto.UpdateProductRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	mappedInput := input.MapUpdateToServiceInput(productUuid)

	updatedProduct, err := ph.getAdminService().UpdateProduct(ctx.Request.Context(), mappedInput)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	productDTO := v1dto.MapUpdateProductRowToDTO(updatedProduct)
	utils.ResponseSuccess(ctx, http.StatusOK, "Product updated successfully", productDTO)
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
// @Success      200 {object} utils.APIResponse{data=v1dto.ProductDTO} "Product soft deleted successfully"
// @Failure      400 {object} utils.ErrorResponse "Invalid UUID"
// @Failure      401 {object} utils.ErrorResponse "Unauthorized"
// @Failure      404 {object} utils.ErrorResponse "Product not found"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/admin/products/{uuid} [delete]
func (ph *ProductHandler) SoftDeleteProduct(ctx *gin.Context) {
	var params v1dto.GetProductByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	productUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	softDeletedProduct, err := ph.getAdminService().SoftDeleteProduct(ctx.Request.Context(), productUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	softDeletedProductDTO := v1dto.MapProductToDTO(softDeletedProduct)
	utils.ResponseSuccess(ctx, http.StatusOK, "Product soft deleted successfully", softDeletedProductDTO)
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
// @Success      200 {object} utils.APIResponse{data=v1dto.ProductDTO} "Product restored successfully"
// @Failure      400 {object} utils.ErrorResponse "Invalid UUID"
// @Failure      401 {object} utils.ErrorResponse "Unauthorized"
// @Failure      404 {object} utils.ErrorResponse "Product not found"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/admin/products/{uuid}/restore [put]
func (ph *ProductHandler) RestoreProduct(ctx *gin.Context) {
	var params v1dto.GetProductByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	productUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	restoredProduct, err := ph.getAdminService().RestoreProduct(ctx.Request.Context(), productUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	restoredProductDTO := v1dto.MapProductToDTO(restoredProduct)
	utils.ResponseSuccess(ctx, http.StatusOK, "Product restored successfully", restoredProductDTO)
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
func (ph *ProductHandler) TrashProduct(ctx *gin.Context) {
	var params v1dto.GetProductByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	productUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	err = ph.getAdminService().TrashProduct(ctx.Request.Context(), productUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseStatusCode(ctx, http.StatusNoContent)
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
// @Success      200 {object} utils.APIResponse{data=v1dto.ProductCSVImportResponse} "CSV import result report"
// @Failure      400 {object} utils.ErrorResponse "Bad request or validation error"
// @Failure      401 {object} utils.ErrorResponse "Unauthorized"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/admin/products/import-csv [post]
func (ph *ProductHandler) ImportCSV(ctx *gin.Context) {
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "File is required (form field: 'file')"))
		return
	}

	if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".csv") {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Only .csv files are supported"))
		return
	}

	if fileHeader.Size > 5*1024*1024 {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "File size must not exceed 5MB"))
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		utils.ResponseError(ctx, utils.WrapError(utils.ErrCodeInternal, "Failed to read uploaded file", err))
		return
	}
	defer file.Close()

	result, err := ph.getAdminService().ImportProductsCSV(ctx.Request.Context(), file)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	if result.ErrorCount > 0 {
		utils.ResponseSuccess(ctx, http.StatusOK, "CSV import completed with validation errors (no records were saved)", result)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, fmt.Sprintf("Successfully imported %d products", result.SuccessCount), result)
}

