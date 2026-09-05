package v1handler

import (
	"encoding/json"
	"fmt"
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

func (oh *OrderHandler) GetMyOrders(ctx *gin.Context) {
	var input v1dto.GetOrdersParam
	if err := ctx.ShouldBindQuery(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orders, totalOrders, err := oh.service.GetMyOrders(ctx, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	paginationResponse := utils.NewPaginationResponse(orders, input.Page, input.Limit, totalOrders)

	utils.ResponseSuccess(ctx, http.StatusOK, "Get my orders successfully", paginationResponse)
}

func (oh *OrderHandler) GetOrderByUUID(ctx *gin.Context) {
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

	order, orderItems, err := oh.service.GetOrderByUUID(ctx, orderUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	orderDTO := v1dto.MapGetOrderByUUIDRowToOrderDTO(order, orderItems)
	utils.ResponseSuccess(ctx, http.StatusOK, "Get order by UUID successfully", orderDTO)
}

func (oh *OrderHandler) GetAllOrders(ctx *gin.Context) {
	var input v1dto.GetOrdersParam
	if err := ctx.ShouldBindQuery(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orders, totalOrders, err := oh.service.GetAllOrders(ctx, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	paginationResponse := utils.NewPaginationResponse(orders, input.Page, input.Limit, totalOrders)

	utils.ResponseSuccess(ctx, http.StatusOK, "Get all orders successfully", paginationResponse)
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

func (oh *OrderHandler) UpdateOrderShippingInfo(ctx *gin.Context) {
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

	var input v1dto.UpdateOrderShippingInfoInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	order := input.MapUpdateShippingInputToModel(orderUUID)

	updatedOrder, updatedOrderItems, err := oh.service.UpdateOrderShippingInfo(ctx, order)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	updatedOrderDTO := v1dto.MapToOrderDTO(updatedOrder, updatedOrderItems)

	utils.ResponseSuccess(ctx, http.StatusOK, "Shipping info updated successfully", updatedOrderDTO)
}

func (oh *OrderHandler) UpdateOrderPaymentStatus(ctx *gin.Context) {
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

	var input v1dto.UpdateOrderPaymentStatusInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	updatedOrder, updatedOrderItems, err := oh.service.UpdateOrderPaymentStatus(ctx, orderUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	updatedOrderDTO := v1dto.MapToOrderDTO(updatedOrder, updatedOrderItems)
	utils.ResponseSuccess(ctx, http.StatusOK, "Payment status updated successfully", updatedOrderDTO)
}

func (oh *OrderHandler) DownloadInvoice(ctx *gin.Context) {
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

	pdfBytes, fileName, err := oh.service.GetOrderInvoicePDF(ctx, orderUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	disposition := "inline"
	if ctx.Query("action") == "download" {
		disposition = "attachment"
	}

	ctx.Header("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, fileName))
	ctx.Header("Content-Type", "application/pdf")
	ctx.Header("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
	ctx.Header("Cache-Control", "private, max-age=86400")
	ctx.Data(http.StatusOK, "application/pdf", pdfBytes)
}

// SHIPPING

func (oh *OrderHandler) CreateShipping(ctx *gin.Context) {
	var params v1dto.CreateShippingParams
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	var input v1dto.CreateShippingInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orderUUID, err := uuid.Parse(params.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID"))
		return
	}

	res, err := oh.service.CreateShipping(ctx, orderUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated, "Shipping order created successfully", res)
}

func (oh *OrderHandler) GetOrderTimeline(ctx *gin.Context) {
	var params v1dto.GetOrderTimeLineParams
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orderUUID, err := uuid.Parse(params.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID"))
		return
	}

	timeline, err := oh.service.GetOrderTimeline(ctx, orderUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Order timeline retrieved successfully", timeline)
}

func (oh *OrderHandler) HandleShippingWebhook(ctx *gin.Context) {
	rawBody, err := ctx.GetRawData()
	if err != nil {
		utils.ResponseError(ctx, utils.WrapError(utils.ErrCodeBadRequest, "Failed to read request body", err))
		return
	}

	var payload v1dto.GHNWebhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		utils.ResponseError(ctx, utils.WrapError(utils.ErrCodeBadRequest, "Invalid JSON payload", err))
		return
	}

	if payload.OrderCode == "" || payload.Status == "" {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "OrderCode and Status are required"))
		return
	}

	if err := oh.service.HandleShippingWebhook(ctx, rawBody, payload); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Webhook processed successfully",
	})
}

func (oh *OrderHandler) CalculateShippingFee(ctx *gin.Context) {
	var input v1dto.CalculateShippingFeeInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	res, err := oh.service.CalculateShippingFee(ctx, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Shipping fee calculated successfully", res)
}
