package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type CategoryRoutes struct {
	handler *v1handler.CategoryHandler
}

func NewCategoryRoutes(handler *v1handler.CategoryHandler) *CategoryRoutes {
	return &CategoryRoutes{
		handler: handler,
	}
}

func (cr *CategoryRoutes) Register(r *gin.RouterGroup) {
	categories := r.Group("/categories")
	{
		categories.POST("", cr.handler.CreateCategory)
		categories.PUT("/:uuid", cr.handler.UpdateCategory)
		categories.DELETE("/:uuid/soft-delete", cr.handler.SoftDeleteCategory)
		categories.PUT("/:uuid/restore", cr.handler.RestoreCategory)
		categories.DELETE("/:uuid/trash", cr.handler.TrashCategory)
	}
}
