package v1handler

import (
	"io"
	"net/http"

	v1dto "goshop/internal/dto/v1"
	"goshop/internal/middleware"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/validation"

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

// CreatePaymentIntent godoc
// @Summary      Create payment intent
// @Description  Initiate online payment checkout url with payment gateway
// @Tags         Payments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        body body v1dto.CreatePaymentIntentInput true "Payment intent payload"
// @Success      200 {object} utils.APIResponse{data=v1dto.PaymentIntentDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /payments/create-intent [post]
func (h *PaymentHandler) CreatePaymentIntent(ctx *gin.Context) {
	userID, err := middleware.MustGetUserID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var input v1dto.CreatePaymentIntentInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	clientIP := ctx.ClientIP()
	idempotencyKey := ctx.GetHeader("X-Idempotency-Key")

	result, err := h.paymentService.CreatePaymentIntent(ctx.Request.Context(), userID, clientIP, idempotencyKey, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Payment intent created successfully", result)
}

// HandleWebhook godoc
// @Summary      Payment gateway webhook
// @Description  Receive IPN / Webhook callbacks from payment gateways (MoMo, VNPay)
// @Tags         Payments
// @Param        provider path string true "Payment provider (e.g. momo, vnpay)"
// @Success      200 {object} map[string]interface{}
// @Failure      400 {object} utils.ErrorResponse
// @Router       /webhooks/payment/{provider} [post]
func (h *PaymentHandler) HandleWebhook(ctx *gin.Context) {
	provider := ctx.Param("provider")

	var rawPayload []byte
	var err error
	if ctx.Request.Method == http.MethodGet {
		rawPayload = []byte(ctx.Request.URL.RawQuery)
	} else {
		rawPayload, err = io.ReadAll(ctx.Request.Body)
		if err != nil {
			utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Failed to read request body"))
			return
		}
	}

	headers := make(map[string]string)
	for k, v := range ctx.Request.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	if err := h.paymentService.HandleWebhook(ctx.Request.Context(), provider, rawPayload, headers); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"resultCode": 0,
		"message":    "Success",
	})
}

// GetPaymentHistory godoc
// @Summary      Get payment history
// @Description  Get transaction payment history by order UUID
// @Tags         Payments
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        order_uuid path string true "Order UUID"
// @Success      200 {object} utils.APIResponse{data=[]v1dto.PaymentTransactionDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /payments/order/{order_uuid} [get]
func (h *PaymentHandler) GetPaymentHistory(ctx *gin.Context) {
	userID, err := middleware.MustGetUserID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}
	userRole, _ := middleware.GetUserRole(ctx)

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

	paymentHistory, err := h.paymentService.GetPaymentHistory(ctx.Request.Context(), userID, userRole, orderUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Payment history retrieved successfully", paymentHistory)
}

// ConfirmCODPayment godoc
// @Summary      Confirm COD payment
// @Description  Confirm that cash on delivery payment has been received (Admin/Moderator)
// @Tags         Admin - Payments
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        order_uuid path string true "Order UUID"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/payments/{order_uuid}/confirm-cod [post]
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

	if err := h.paymentService.ConfirmCODPayment(ctx.Request.Context(), orderUUID); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "COD payment confirmed successfully", nil)
}

// RefundPayment godoc
// @Summary      Refund payment
// @Description  Process payment refund transaction for an order (Admin/Moderator)
// @Tags         Admin - Payments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        body body v1dto.RefundPaymentInput true "Refund payload"
// @Success      200 {object} utils.APIResponse{data=v1dto.PaymentTransactionDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/payments/refund [post]
func (h *PaymentHandler) RefundPayment(ctx *gin.Context) {
	var input v1dto.RefundPaymentInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	idempotencyKey := ctx.GetHeader("X-Idempotency-Key")

	refundResult, err := h.paymentService.RefundPayment(ctx.Request.Context(), idempotencyKey, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Refund processed successfully", refundResult)
}
