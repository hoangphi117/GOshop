package v1handler

import (
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/validation"
	"net/http"

	"github.com/gin-gonic/gin"
)

type OrderHandler struct {
	service v1service.OrderService
}

func NewOrderHandler(service v1service.OrderService) *OrderHandler {
	return &OrderHandler{
		service: service,
	}
}

func (oh *OrderHandler) CreateOrder(ctx *gin.Context) {
	var input v1dto.CreateOrderInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	order, orderItems, err := oh.service.CreateOrder(ctx, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	orderDTO := v1dto.MapToOrderDTO(order, orderItems)

	utils.ResponseSuccess(ctx, http.StatusCreated, "Order created successfully", orderDTO)
}
