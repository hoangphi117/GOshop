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

func (ph *ProductHandler) CreateProduct(ctx *gin.Context) {
	var input v1dto.CreateProductRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	mappedInput := input.MapCreateToServiceInput()

	product, err := ph.service.CreateProduct(ctx, mappedInput)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	productDTO := v1dto.MapCreateProductRowToDTO(product)

	utils.ResponseSuccess(ctx, http.StatusCreated, "Created product successfully", productDTO)
}

func (ph *ProductHandler) GetAllProducts(ctx *gin.Context) {
	var params v1dto.GetProductsParam
	if err := ctx.ShouldBindQuery(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	listProducts, total, err := ph.service.GetAllProducts(ctx, params.Search, params.CategoryUUID, params.MinPrice, params.MaxPrice, params.Status, params.OrderBy, params.Sort, params.Page, params.Limit, false)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	listProductsDTO := v1dto.MapListProductsToDTO(listProducts)

	paginationResponse := utils.NewPaginationResponse(listProductsDTO, params.Page, params.Limit, total)

	utils.ResponseSuccess(ctx, http.StatusOK, "Products retrieved successfully", paginationResponse)
}

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

	product, err := ph.service.GetProductByUUID(ctx, productUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	productDTO := v1dto.MapGetProductByUUIDRowToDTO(product)
	utils.ResponseSuccess(ctx, http.StatusOK, "Product retrieved successfully", productDTO)
}

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

	updatedProduct, err := ph.service.UpdateProduct(ctx, mappedInput)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	productDTO := v1dto.MapUpdateProductRowToDTO(updatedProduct)
	utils.ResponseSuccess(ctx, http.StatusOK, "Product updated successfully", productDTO)
}

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

	softDeletedProduct, err := ph.service.SoftDeleteProduct(ctx, productUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	softDeletedProductDTO := v1dto.MapProductToDTO(softDeletedProduct)
	utils.ResponseSuccess(ctx, http.StatusOK, "Product soft deleted successfully", softDeletedProductDTO)
}

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

	restoredProduct, err := ph.service.RestoreProduct(ctx, productUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	restoredProductDTO := v1dto.MapProductToDTO(restoredProduct)
	utils.ResponseSuccess(ctx, http.StatusOK, "Product restored successfully", restoredProductDTO)
}

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

	err = ph.service.TrashProduct(ctx, productUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseStatusCode(ctx, http.StatusNoContent)
}
