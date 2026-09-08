package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type AdminPaymentRoutes struct {
	handler *v1handler.PaymentHandler
}

func NewAdminPaymentRoutes(handler *v1handler.PaymentHandler) *AdminPaymentRoutes {
	return &AdminPaymentRoutes{
		handler: handler,
	}
}

func (r *AdminPaymentRoutes) Register(rg *gin.RouterGroup) {
	payments := rg.Group("/payments")
	{
		payments.POST("/:order_uuid/confirm-cod", r.handler.ConfirmCODPayment)
		payments.POST("/refund", r.handler.RefundPayment)
	}
}
