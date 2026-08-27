package v1handler

import (
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/validation"
	"net/http"

	"github.com/gin-gonic/gin"
)

type CartHandler struct {
	service v1service.CartService
}

func NewCartHandler(service v1service.CartService) *CartHandler {
	return &CartHandler{
		service: service,
	}
}

func (ch *CartHandler) GetMyCart(ctx *gin.Context) {

	cart, err := ch.service.GetMyCart(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Cart retrieved successfully", cart)
}

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
