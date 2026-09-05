package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type OrderRoutes struct {
	handler *v1handler.OrderHandler
}

func NewOrderRoutes(handler *v1handler.OrderHandler) *OrderRoutes {
	return &OrderRoutes{
		handler: handler,
	}
}

func (or *OrderRoutes) Register(r *gin.RouterGroup) {
	orders := r.Group("/orders")
	{
		orders.GET("", or.handler.GetMyOrders)
		orders.GET("/:uuid", or.handler.GetOrderByUUID)
		orders.POST("", or.handler.CreateOrder)
		orders.PATCH("/:uuid/cancel", or.handler.CancelOrder)
		orders.PUT("/:uuid/shipping-info", or.handler.UpdateOrderShippingInfo)
		orders.GET("/:uuid/invoice", or.handler.DownloadInvoice)

		orders.GET("/:uuid/timeline", or.handler.GetOrderTimeline)
	}
}
