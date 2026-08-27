package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type CartRoutes struct {
	handler *v1handler.CartHandler
}

func NewCartRoutes(handler *v1handler.CartHandler) *CartRoutes {
	return &CartRoutes{
		handler: handler,
	}
}

func (cr *CartRoutes) Register(r *gin.RouterGroup) {
	cart := r.Group("/cart")
	{
		cart.GET("", cr.handler.GetMyCart)
		cart.POST("/items", cr.handler.AddItemToCart)
	}
}
