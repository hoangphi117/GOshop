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
