package v1handler

import (
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
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
