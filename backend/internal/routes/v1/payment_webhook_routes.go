package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type PaymentWebhookRoutes struct {
	handler *v1handler.PaymentHandler
}

func NewPaymentWebhookRoutes(handler *v1handler.PaymentHandler) *PaymentWebhookRoutes {
	return &PaymentWebhookRoutes{
		handler: handler,
	}
}

func (r *PaymentWebhookRoutes) Register(rg *gin.RouterGroup) {
	webhooks := rg.Group("/webhooks/payment")
	{
		webhooks.POST("/:provider", r.handler.HandleWebhook)
		webhooks.GET("/:provider", r.handler.HandleWebhook) // VNPay IPN
	}
}
