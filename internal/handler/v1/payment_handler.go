package v1handler

import (
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/validation"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
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

	if strings.ToLower(provider) == "vnpay" {
		ctx.JSON(http.StatusOK, gin.H{
			"RspCode": "00",
			"Message": "Confirm Success",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"resultCode": 0,
		"message":    "Success",
	})
}
