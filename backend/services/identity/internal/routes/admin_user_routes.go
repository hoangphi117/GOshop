package routes

import (
	"goshop/pkg/middleware"
	"goshop/services/identity/internal/handler"

	"github.com/gin-gonic/gin"
)

type AdminUserRoutes struct {
	handler *handler.UserHandler
}

func NewAdminUserRoutes(handler *handler.UserHandler) *AdminUserRoutes {
	return &AdminUserRoutes{
		handler: handler,
	}
}

func (ur *AdminUserRoutes) Register(r *gin.RouterGroup) {
	users := r.Group("/users")
	{
		users.GET("", ur.handler.GetAllUsers)
		users.POST("", ur.handler.CreateUsers)
		users.GET("/:uuid", ur.handler.GetUserByUUID)
		users.GET("/soft-deleted", ur.handler.GetSoftDeletedUsers)
		users.PUT("/:uuid", ur.handler.UpdateUser)
		users.DELETE("/:uuid/soft-delete", ur.handler.SoftDeleteUser)
		users.PUT("/:uuid/restore", ur.handler.RestoreUser)
		users.DELETE("/:uuid/trash", middleware.RequireRole(middleware.RoleAdmin), ur.handler.TrashUser)
	}
}
