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

type ProductHandler struct {
	service v1service.ProductService
}

func NewProductHandler(service v1service.ProductService) *ProductHandler {
	return &ProductHandler{
		service: service,
	}
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

	product, err := ph.service.CreateProduct(ctx.Request.Context(), mappedInput)
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

	listProducts, total, err := ph.service.GetAllProducts(ctx.Request.Context(), params.Search, params.CategoryUUID, params.MinPrice, params.MaxPrice, params.Status, params.OrderBy, params.Sort, params.Page, params.Limit, false)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	listProductsDTO := v1dto.MapListProductsToDTO(listProducts)

	paginationResponse := utils.NewPaginationResponse(listProductsDTO, params.Page, params.Limit, total)

	utils.ResponseSuccess(ctx, http.StatusOK, "Products retrieved successfully", paginationResponse)
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
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	product, err := ph.service.GetProductByUUID(ctx.Request.Context(), productUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	productDTO := v1dto.MapGetProductByUUIDRowToDTO(product)
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

	updatedProduct, err := ph.service.UpdateProduct(ctx.Request.Context(), mappedInput)
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

	softDeletedProduct, err := ph.service.SoftDeleteProduct(ctx.Request.Context(), productUuid)
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

	restoredProduct, err := ph.service.RestoreProduct(ctx.Request.Context(), productUuid)
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

	err = ph.service.TrashProduct(ctx.Request.Context(), productUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseStatusCode(ctx, http.StatusNoContent)
}
