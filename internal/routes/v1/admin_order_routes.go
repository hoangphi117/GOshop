package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type AdminOrderRoutes struct {
	handler *v1handler.OrderHandler
}

func NewAdminOrderRoutes(handler *v1handler.OrderHandler) *AdminOrderRoutes {
	return &AdminOrderRoutes{
		handler: handler,
	}
}

func (or *AdminOrderRoutes) Register(r *gin.RouterGroup) {
	orders := r.Group("/orders")
	{
		orders.PATCH("/:uuid", or.handler.UpdateOrderStatus)
	}
}
