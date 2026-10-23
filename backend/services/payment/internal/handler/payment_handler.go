package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"goshop/pkg/logger"
	"goshop/pkg/utils"
	"goshop/services/payment/internal/dto"
	"goshop/services/payment/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PaymentHandler struct {
	service service.PaymentService
}

func NewPaymentHandler(service service.PaymentService) *PaymentHandler {
	return &PaymentHandler{service: service}
}

func (h *PaymentHandler) CreatePaymentIntent(ctx *gin.Context) {
	var input dto.CreatePaymentIntentInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseError(ctx, utils.WrapError(utils.ErrCodeBadRequest, "Invalid payment intent input", err))
		return
	}

	userIDVal, exists := ctx.Get("userID")
	if exists {
		if uid, ok := userIDVal.(int32); ok {
			input.UserID = uid
		}
	}
	input.ClientIP = ctx.ClientIP()
	input.IdempotencyKey = ctx.GetHeader("X-Idempotency-Key")

	intent, err := h.service.CreatePaymentIntent(ctx.Request.Context(), input)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to create payment intent")
		utils.ResponseError(ctx, utils.WrapError(utils.ErrCodeInternal, "Payment intent creation failed", err))
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Payment intent created successfully", intent)
}

func (h *PaymentHandler) GetPaymentTransactions(ctx *gin.Context) {
	orderUUIDStr := ctx.Param("order_uuid")
	orderUUID, err := uuid.Parse(orderUUIDStr)
	if err != nil {
		utils.ResponseError(ctx, utils.WrapError(utils.ErrCodeBadRequest, fmt.Sprintf("invalid order uuid: %s", orderUUIDStr), err))
		return
	}

	txns, err := h.service.GetPaymentTransactionsByOrderUUID(ctx.Request.Context(), orderUUID)
	if err != nil {
		logger.Log.Error().Err(err).Str("order_uuid", orderUUIDStr).Msg("Failed to get payment transactions")
		utils.ResponseError(ctx, utils.WrapError(utils.ErrCodeInternal, "Failed to retrieve payment transactions", err))
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Payment transactions retrieved successfully", txns)
}

func (h *PaymentHandler) HandleWebhook(ctx *gin.Context) {
	provider := ctx.Param("provider")
	if provider == "" {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "payment provider parameter is required"))
		return
	}

	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		utils.ResponseError(ctx, utils.WrapError(utils.ErrCodeBadRequest, "failed to read webhook request body", err))
		return
	}
	if len(body) == 0 && ctx.Request.URL.RawQuery != "" {
		body = []byte(ctx.Request.URL.RawQuery)
	}

	headers := make(map[string]string)
	for k, v := range ctx.Request.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	outcome, err := h.service.HandleWebhook(ctx.Request.Context(), provider, body, headers)
	if provider == "vnpay" {
		// VNPay IPN contract: always HTTP 200 with an RspCode/Message body.
		// A non-200 or a foreign body shape makes VNPay treat the IPN as
		// undelivered and retry, eventually disabling IPN for the merchant.
		rsp := vnpayRspCodeFor(err)
		if err == nil && outcome == service.WebhookAlreadyProcessed {
			rsp = vnpayResponse{Code: "02", Message: "Order already confirmed"}
		}
		if err != nil {
			logger.Log.Error().Err(err).Str("provider", provider).Msg("Webhook processing failed")
		}
		vnpayRespond(ctx, rsp)
		return
	}

	if err != nil {
		logger.Log.Error().Err(err).Str("provider", provider).Msg("Webhook processing failed")
		utils.ResponseError(ctx, utils.WrapError(utils.ErrCodeBadRequest, "webhook processing failed", err))
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Webhook processed successfully", gin.H{"status": "success"})
}

// vnpayResponse is the body VNPay expects from an IPN endpoint.
type vnpayResponse struct {
	Code    string `json:"RspCode"`
	Message string `json:"Message"`
}

func vnpayRespond(ctx *gin.Context, body vnpayResponse) {
	// VNPay requires HTTP 200 for every business outcome; the RspCode carries
	// the result (00 = confirmed, 01 = order not found, 02 = already confirmed,
	// 04 = amount mismatch, 97 = bad signature, 99 = unknown error).
	ctx.JSON(http.StatusOK, body)
}

// vnpayRspCodeFor maps a webhook error to VNPay's documented RspCode values.
func vnpayRspCodeFor(err error) vnpayResponse {
	switch {
	case errors.Is(err, service.ErrPaymentTxnNotFound):
		return vnpayResponse{Code: "01", Message: "Order not found"}
	case errors.Is(err, service.ErrWebhookSignature):
		return vnpayResponse{Code: "97", Message: "Invalid signature"}
	case errors.Is(err, service.ErrWebhookAmountMismatch):
		return vnpayResponse{Code: "04", Message: "Invalid amount"}
	default:
		return vnpayResponse{Code: "99", Message: "Unknown error"}
	}
}

func (h *PaymentHandler) RefundPayment(ctx *gin.Context) {
	var input dto.RefundPaymentInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseError(ctx, utils.WrapError(utils.ErrCodeBadRequest, "invalid refund input", err))
		return
	}

	input.IdempotencyKey = ctx.GetHeader("X-Idempotency-Key")

	txn, err := h.service.RefundPayment(ctx.Request.Context(), input)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Refund failed")
		utils.ResponseError(ctx, utils.WrapError(utils.ErrCodeInternal, "payment refund failed", err))
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Payment refunded successfully", txn)
}
