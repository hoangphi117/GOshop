package routes

import (
	"goshop/services/inventory/internal/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, inventoryHandler *handler.InventoryHandler) {
	admin := r.Group("/api/v1/admin/inventory")
	{
		admin.GET("/movements", inventoryHandler.GetStockMovements)
		admin.POST("/adjust", inventoryHandler.AdjustStock)
	}

	api := r.Group("/api/v1/inventory")
	{
		api.GET("/stock", inventoryHandler.GetStock)
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "inventory"})
	})
}
