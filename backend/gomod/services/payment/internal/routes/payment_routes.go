package routes

import (
	"goshop/services/payment/internal/handler"

	"github.com/gin-gonic/gin"
)

type PaymentRoutes struct {
	handler *handler.PaymentHandler
}

func NewPaymentRoutes(handler *handler.PaymentHandler) *PaymentRoutes {
	return &PaymentRoutes{handler: handler}
}

func (r *PaymentRoutes) Register(rg *gin.RouterGroup) {
	payments := rg.Group("/payments")
	{
		payments.POST("/create-intent", r.handler.CreatePaymentIntent)
		payments.POST("/intent", r.handler.CreatePaymentIntent)
		payments.GET("/transactions/:order_uuid", r.handler.GetPaymentTransactions)
		payments.GET("/order/:order_uuid", r.handler.GetPaymentTransactions)
	}
}

type PaymentWebhookRoutes struct {
	handler *handler.PaymentHandler
}

func NewPaymentWebhookRoutes(handler *handler.PaymentHandler) *PaymentWebhookRoutes {
	return &PaymentWebhookRoutes{handler: handler}
}

func (r *PaymentWebhookRoutes) Register(rg *gin.RouterGroup) {
	webhooks := rg.Group("/webhooks/payment")
	{
		webhooks.POST("/:provider", r.handler.HandleWebhook)
		webhooks.GET("/:provider", r.handler.HandleWebhook)
	}
	payments := rg.Group("/payments")
	{
		payments.POST("/webhook/:provider", r.handler.HandleWebhook)
		payments.GET("/webhook/:provider", r.handler.HandleWebhook)
	}
}

type AdminPaymentRoutes struct {
	handler *handler.PaymentHandler
}

func NewAdminPaymentRoutes(handler *handler.PaymentHandler) *AdminPaymentRoutes {
	return &AdminPaymentRoutes{handler: handler}
}

func (r *AdminPaymentRoutes) Register(rg *gin.RouterGroup) {
	payments := rg.Group("/payments")
	{
		payments.POST("/refund", r.handler.RefundPayment)
	}
}
