package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type PaymentRoutes struct {
	handler *v1handler.PaymentHandler
}

func NewPaymentRoutes(handler *v1handler.PaymentHandler) *PaymentRoutes {
	return &PaymentRoutes{
		handler: handler,
	}
}

func (r *PaymentRoutes) Register(rg *gin.RouterGroup) {
	payments := rg.Group("/payments")
	{
		payments.POST("/create-intent", r.handler.CreatePaymentIntent)
	}
}
