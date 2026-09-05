package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type ShippingWebhookRoutes struct {
	handler *v1handler.OrderHandler
}

func NewShippingWebhookRoutes(handler *v1handler.OrderHandler) *ShippingWebhookRoutes {
	return &ShippingWebhookRoutes{handler: handler}
}

func (r *ShippingWebhookRoutes) Register(rg *gin.RouterGroup) {
	shipping := rg.Group("/shipping")
	{
		shipping.POST("/webhook/ghn", r.handler.HandleShippingWebhook)

		shipping.GET("/provinces", r.handler.GetProvinces)
		shipping.GET("/districts", r.handler.GetDistricts)
		shipping.GET("/wards", r.handler.GetWards)
	}
}
