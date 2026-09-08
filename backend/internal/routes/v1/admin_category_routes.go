package v1routes

import (
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/middleware"

	"github.com/gin-gonic/gin"
)

type AdminCategoryRoutes struct {
	handler *v1handler.CategoryHandler
}

func NewAdminCategoryRoutes(handler *v1handler.CategoryHandler) *AdminCategoryRoutes {
	return &AdminCategoryRoutes{
		handler: handler,
	}
}

func (cr *AdminCategoryRoutes) Register(r *gin.RouterGroup) {
	categories := r.Group("/categories")
	{
		categories.GET("", cr.handler.GetAllCategories)
		categories.GET("/:uuid", cr.handler.GetCategoryByUUID)
		categories.POST("", cr.handler.CreateCategory)
		categories.PUT("/:uuid", cr.handler.UpdateCategory)
		categories.DELETE("/:uuid/soft-delete", cr.handler.SoftDeleteCategory)
		categories.PUT("/:uuid/restore", cr.handler.RestoreCategory)
		categories.DELETE("/:uuid/trash", middleware.RequireRole(middleware.RoleAdmin), cr.handler.TrashCategory)
	}
}
