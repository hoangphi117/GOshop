package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type AdminProductRoutes struct {
	handler *v1handler.ProductHandler
}

func NewAdminProductRoutes(handler *v1handler.ProductHandler) *AdminProductRoutes {
	return &AdminProductRoutes{
		handler: handler,
	}
}

func (pr *AdminProductRoutes) Register(r *gin.RouterGroup) {
	products := r.Group("/products")
	{
		products.POST("", pr.handler.CreateProduct)
		products.PUT("/:uuid", pr.handler.UpdateProduct)
		products.DELETE("/:uuid/soft-delete", pr.handler.SoftDeleteProduct)
		products.PUT("/:uuid/restore", pr.handler.RestoreProduct)
		products.DELETE("/:uuid/trash", pr.handler.TrashProduct)
	}
}
