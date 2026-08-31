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

type PaymentHandler struct {
	paymentService v1service.PaymentService
}

func NewPaymentHandler(paymentService v1service.PaymentService) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
	}
}

func (h *PaymentHandler) CreatePaymentIntent(ctx *gin.Context) {
	var input v1dto.CreatePaymentIntentInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	result, err := h.paymentService.CreatePaymentIntent(ctx, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Payment intent created successfully", result)
}

func (h *PaymentHandler) HandleWebhook(ctx *gin.Context) {
	provider := ctx.Param("provider")

	if err := h.paymentService.HandleWebhook(ctx, provider); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"resultCode": 0,
		"message":    "Success",
	})
}

func (h *PaymentHandler) GetPaymentHistory(ctx *gin.Context) {
	var params v1dto.GetTransactionHistoryByOrderUUID
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orderUUID, err := uuid.Parse(params.OrderUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	paymentHistory, err := h.paymentService.GetPaymentHistory(ctx, orderUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Payment history retrieved successfully", paymentHistory)
}

func (h *PaymentHandler) ConfirmCODPayment(ctx *gin.Context) {
	var params v1dto.ConfirmCODPaymentInput
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orderUUID, err := uuid.Parse(params.OrderUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	if err := h.paymentService.ConfirmCODPayment(ctx, orderUUID); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "COD payment confirmed successfully", nil)
}

func (h *PaymentHandler) RefundPayment(ctx *gin.Context) {
	var input v1dto.RefundPaymentInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	refundResult, err := h.paymentService.RefundPayment(ctx, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Refund processed successfully", refundResult)
}
