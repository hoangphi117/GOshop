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
		orders.GET("/returns", or.handler.GetMyOrderReturns)
		orders.GET("/:uuid", or.handler.GetOrderByUUID)
		orders.POST("", or.handler.CreateOrder)
		orders.PATCH("/:uuid/cancel", or.handler.CancelOrder)
		orders.PUT("/:uuid/shipping-info", or.handler.UpdateOrderShippingInfo)
		orders.GET("/:uuid/invoice", or.handler.DownloadInvoice)

		orders.GET("/:uuid/timeline", or.handler.GetOrderTimeline)
		orders.POST("/calculate-fee", or.handler.CalculateShippingFee)
		orders.POST("/:uuid/return", or.handler.RequestOrderReturn)
		orders.GET("/:uuid/return", or.handler.GetOrderReturnDetails)
		orders.PATCH("/:uuid/return/cancel", or.handler.CancelOrderReturn)
		orders.POST("/:uuid/return/shipping-info", or.handler.SubmitReturnShipping)
		orders.POST("/:uuid/return/shipping", or.handler.SubmitReturnShipping)
		orders.PUT("/:uuid/return/bank-account", or.handler.UpdateReturnBankAccount)
	}
}
