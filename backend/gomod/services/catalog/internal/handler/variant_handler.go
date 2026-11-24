package handler

import (
	"net/http"

	catalogdto "goshop/services/catalog/internal/dto"
	"goshop/shared/pkg/utils"
	"goshop/shared/pkg/validation"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// =============================================================================
// PRODUCT VARIANT HANDLERS
// =============================================================================

// ListVariants godoc
// @Summary      List product variants
// @Description  Returns all variants for a product (active-only for storefront, all for admin)
// @Tags         Products
// @Produce      json
// @Param        uuid       path  string  true  "Product UUID"
// @Param        active_only query bool   false "Only active variants (default true for storefront)"
// @Success      200 {object} utils.APIResponse{data=[]catalogdto.ProductVariantDTO}
// @Router       /products/{uuid}/variants [get]
func (h *CatalogHandler) ListVariants(c *gin.Context) {
	productUUID, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "invalid product UUID"))
		return
	}

	activeOnly := c.Query("active_only") != "false"
	variants, err := h.svc.GetVariantsByProduct(c.Request.Context(), productUUID, activeOnly)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}
	utils.ResponseSuccess(c, http.StatusOK, "Variants retrieved", variants)
}

// CreateVariant godoc
// @Summary      Create a product variant
// @Description  Admin: adds a new SKU variant to a product
// @Tags         Admin - Products
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path   string                            true "Product UUID"
// @Param        body body   catalogdto.CreateVariantInput     true "Variant payload"
// @Success      201 {object} utils.APIResponse{data=catalogdto.ProductVariantDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Router       /admin/products/{uuid}/variants [post]
func (h *CatalogHandler) CreateVariant(c *gin.Context) {
	productUUID, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "invalid product UUID"))
		return
	}

	var input catalogdto.CreateVariantInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(c, validation.HandleValidationErrors(err))
		return
	}

	variant, err := h.svc.CreateVariant(c.Request.Context(), productUUID, input)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}
	utils.ResponseSuccess(c, http.StatusCreated, "Variant created", variant)
}

// UpdateVariant godoc
// @Summary      Update a product variant
// @Description  Admin: patches an existing variant
// @Tags         Admin - Products
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid        path string                        true "Product UUID"
// @Param        variantUUID path string                        true "Variant UUID"
// @Param        body        body catalogdto.UpdateVariantInput true "Variant patch payload"
// @Success      200 {object} utils.APIResponse{data=catalogdto.ProductVariantDTO}
// @Router       /admin/products/{uuid}/variants/{variantUUID} [put]
func (h *CatalogHandler) UpdateVariant(c *gin.Context) {
	productUUID, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "invalid product UUID"))
		return
	}
	variantUUID, err := uuid.Parse(c.Param("variantUUID"))
	if err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "invalid variant UUID"))
		return
	}

	var input catalogdto.UpdateVariantInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(c, validation.HandleValidationErrors(err))
		return
	}
	input.VariantUUID = variantUUID.String()

	variant, err := h.svc.UpdateVariant(c.Request.Context(), productUUID, variantUUID, input)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}
	utils.ResponseSuccess(c, http.StatusOK, "Variant updated", variant)
}

// DeleteVariant godoc
// @Summary      Delete a product variant
// @Description  Admin: permanently removes a variant
// @Tags         Admin - Products
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid        path string true "Product UUID"
// @Param        variantUUID path string true "Variant UUID"
// @Success      200 {object} utils.APIResponse{}
// @Router       /admin/products/{uuid}/variants/{variantUUID} [delete]
func (h *CatalogHandler) DeleteVariant(c *gin.Context) {
	productUUID, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "invalid product UUID"))
		return
	}
	variantUUID, err := uuid.Parse(c.Param("variantUUID"))
	if err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "invalid variant UUID"))
		return
	}

	if err := h.svc.DeleteVariant(c.Request.Context(), productUUID, variantUUID); err != nil {
		utils.ResponseError(c, err)
		return
	}
	utils.ResponseSuccess(c, http.StatusOK, "Variant deleted", nil)
}
