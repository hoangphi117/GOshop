package handler

import (
	"net/http"
	"strconv"

	"goshop/internal/utils"
	"goshop/pkg/middleware"
	"goshop/pkg/validation"
	cartdto "goshop/services/cart/internal/dto"
	"goshop/services/cart/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CartHandler struct {
	service service.CartService
}

func NewCartHandler(service service.CartService) *CartHandler {
	return &CartHandler{
		service: service,
	}
}

func extractUserIdentity(ctx *gin.Context) (uuid.UUID, *int32, error) {
	// Try standard middleware first
	userUUID, err := middleware.GetUserUUID(ctx)
	if err == nil && userUUID != uuid.Nil {
		var userIDPtr *int32
		if uidVal, exists := ctx.Get("userID"); exists {
			if uid, ok := uidVal.(int32); ok && uid > 0 {
				userIDPtr = &uid
			}
		}
		return userUUID, userIDPtr, nil
	}

	// Fallback to headers
	if uStr := ctx.GetHeader("X-User-UUID"); uStr != "" {
		if parsed, err := uuid.Parse(uStr); err == nil {
			var userIDPtr *int32
			if idStr := ctx.GetHeader("X-User-ID"); idStr != "" {
				if id64, err := strconv.ParseInt(idStr, 10, 32); err == nil {
					id32 := int32(id64)
					userIDPtr = &id32
				}
			}
			return parsed, userIDPtr, nil
		}
	}

	return uuid.Nil, nil, utils.NewError(utils.ErrCodeUnauthorized, "unauthorized: missing user identity")
}

// GetMyCart godoc
// @Summary      Get current user cart
// @Description  Get cart details and items for current authenticated user
// @Tags         Cart
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Success      200 {object} utils.APIResponse{data=cartdto.CartDTO}
// @Failure      401 {object} utils.ErrorResponse
// @Router       /cart [get]
func (h *CartHandler) GetMyCart(ctx *gin.Context) {
	userUUID, userID, err := extractUserIdentity(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	cart, err := h.service.GetMyCart(ctx.Request.Context(), userUUID, userID)
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
// @Param        body body cartdto.AddCartItemInput true "Item payload"
// @Success      200 {object} utils.APIResponse{data=cartdto.CartDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /cart/items [post]
func (h *CartHandler) AddItemToCart(ctx *gin.Context) {
	userUUID, userID, err := extractUserIdentity(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var input cartdto.AddCartItemInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	cart, err := h.service.AddItemToCart(ctx.Request.Context(), userUUID, userID, input)
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
// @Param        body body cartdto.UpdateCartItemQuantityInput true "Quantity payload"
// @Success      200 {object} utils.APIResponse{data=cartdto.CartDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /cart/items/{item_uuid} [put]
func (h *CartHandler) UpdateCartItemQuantity(ctx *gin.Context) {
	userUUID, userID, err := extractUserIdentity(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var itemUUIDStr cartdto.UpdateCartItemUUID
	if err := ctx.ShouldBindUri(&itemUUIDStr); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	itemUUID, err := uuid.Parse(itemUUIDStr.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "invalid UUID format"))
		return
	}

	var input cartdto.UpdateCartItemQuantityInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	cart, err := h.service.UpdateCartItemQuantity(ctx.Request.Context(), userUUID, userID, itemUUID, input)
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
// @Success      200 {object} utils.APIResponse{data=cartdto.CartDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /cart/items/{item_uuid} [delete]
func (h *CartHandler) RemoveItemFromCart(ctx *gin.Context) {
	userUUID, userID, err := extractUserIdentity(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var itemUUIDStr cartdto.UpdateCartItemUUID
	if err := ctx.ShouldBindUri(&itemUUIDStr); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	itemUUID, err := uuid.Parse(itemUUIDStr.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "invalid UUID format"))
		return
	}

	cartDTO, err := h.service.RemoveItemFromCart(ctx.Request.Context(), userUUID, userID, itemUUID)
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
func (h *CartHandler) ClearMyCart(ctx *gin.Context) {
	userUUID, userID, err := extractUserIdentity(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	err = h.service.ClearMyCart(ctx.Request.Context(), userUUID, userID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Cart cleared successfully", nil)
}
