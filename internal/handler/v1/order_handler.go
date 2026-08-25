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

func (oh *OrderHandler) UpdateOrderStatus(ctx *gin.Context) {
	var params v1dto.GetOrderByUUIDParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orderUUID, err := uuid.Parse(params.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID"))
		return
	}

	var input v1dto.UpdateOrderStatusInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	order := input.MapUpdateStatusInputToModel(orderUUID)

	updatedOrder, orderItems, err := oh.service.UpdateOrderStatus(ctx, order)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	updatedOrderDTO := v1dto.MapToOrderDTO(updatedOrder, orderItems)
	utils.ResponseSuccess(ctx, http.StatusOK, "Order status updated successfully", updatedOrderDTO)
}

func (oh *OrderHandler) CancelOrder(ctx *gin.Context) {
	var params v1dto.GetOrderByUUIDParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orderUUID, err := uuid.Parse(params.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID"))
		return
	}

	err = oh.service.CancelOrder(ctx, orderUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Order canceled successfully", nil)
}
