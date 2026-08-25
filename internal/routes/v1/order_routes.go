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
		orders.POST("", or.handler.CreateOrder)
		orders.PATCH("/:uuid/cancel", or.handler.CancelOrder)
	}
}
