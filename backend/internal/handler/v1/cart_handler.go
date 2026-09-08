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

type CartHandler struct {
	service v1service.CartService
}

func NewCartHandler(service v1service.CartService) *CartHandler {
	return &CartHandler{
		service: service,
	}
}

// GetMyCart godoc
// @Summary      Get current user cart
// @Description  Get cart details and items for current authenticated user
// @Tags         Cart
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Success      200 {object} utils.APIResponse{data=v1dto.CartDTO}
// @Failure      401 {object} utils.ErrorResponse
// @Router       /cart [get]
func (ch *CartHandler) GetMyCart(ctx *gin.Context) {

	cart, err := ch.service.GetMyCart(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Cart retrieved successfully", cart)
}

// AddItemToCart godoc
// @Summary      Add item to cart
// @Description  Add product item with quantity to cart
// @Tags         Cart
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        body body v1dto.AddCartItemInput true "Item payload"
// @Success      200 {object} utils.APIResponse{data=v1dto.CartDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /cart/items [post]
func (ch *CartHandler) AddItemToCart(ctx *gin.Context) {
	var input v1dto.AddCartItemInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	cart, err := ch.service.AddItemToCart(ctx, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Item added to cart successfully", cart)
}

// UpdateCartItemQuantity godoc
// @Summary      Update cart item quantity
// @Description  Update item quantity in cart
// @Tags         Cart
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        item_uuid path string true "Cart item UUID"
// @Param        body body v1dto.UpdateCartItemQuantityInput true "Quantity payload"
// @Success      200 {object} utils.APIResponse{data=v1dto.CartDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /cart/items/{item_uuid} [put]
func (ch *CartHandler) UpdateCartItemQuantity(ctx *gin.Context) {
	var itemUUIDStr v1dto.UpdateCartItemUUID
	if err := ctx.ShouldBindUri(&itemUUIDStr); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	itemUUID, err := uuid.Parse(itemUUIDStr.UUID)

	var input v1dto.UpdateCartItemQuantityInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	cart, err := ch.service.UpdateCartItemQuantity(ctx, itemUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Cart item quantity updated successfully", cart)
}

// RemoveItemFromCart godoc
// @Summary      Remove item from cart
// @Description  Remove a specific item from cart
// @Tags         Cart
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        item_uuid path string true "Cart item UUID"
// @Success      200 {object} utils.APIResponse{data=v1dto.CartDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /cart/items/{item_uuid} [delete]
func (ch *CartHandler) RemoveItemFromCart(ctx *gin.Context) {
	var itemUUIDStr v1dto.UpdateCartItemUUID
	if err := ctx.ShouldBindUri(&itemUUIDStr); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	itemUUID, err := uuid.Parse(itemUUIDStr.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "invalid UUID format"))
		return
	}

	cartDTO, err := ch.service.RemoveItemFromCart(ctx, itemUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Item removed from cart successfully", cartDTO)
}

// ClearMyCart godoc
// @Summary      Clear cart
// @Description  Remove all items from current user cart
// @Tags         Cart
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Success      200 {object} utils.APIResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /cart [delete]
func (ch *CartHandler) ClearMyCart(ctx *gin.Context) {
	err := ch.service.ClearMyCart(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Cart cleared successfully", nil)
}
