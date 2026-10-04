package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type ProductRoutes struct {
	handler *v1handler.ProductHandler
}

func NewProductRoutes(handler *v1handler.ProductHandler) *ProductRoutes {
	return &ProductRoutes{
		handler: handler,
	}
}

func (pr *ProductRoutes) Register(r *gin.RouterGroup) {
	products := r.Group("/products")
	{
		products.GET("", pr.handler.GetAllProducts)
		products.GET("/search", pr.handler.SearchProducts)
		products.GET("/suggest", pr.handler.SuggestProducts)
		products.GET("/:uuid", pr.handler.GetProductByUUID)
	}
}
