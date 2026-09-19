package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type AdminInventoryRoutes struct {
	handler *v1handler.InventoryHandler
}

func NewAdminInventoryRoutes(handler *v1handler.InventoryHandler) *AdminInventoryRoutes {
	return &AdminInventoryRoutes{
		handler: handler,
	}
}

func (r *AdminInventoryRoutes) Register(rg *gin.RouterGroup) {
	inventory := rg.Group("/inventory")
	{
		inventory.GET("/movements", r.handler.GetStockMovements)
		inventory.POST("/adjust", r.handler.AdjustStock)
	}
}
