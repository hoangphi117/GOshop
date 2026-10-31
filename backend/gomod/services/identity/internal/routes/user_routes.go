package routes

import (
	"goshop/services/identity/internal/handler"

	"github.com/gin-gonic/gin"
)

type UserRoutes struct {
	handler *handler.UserHandler
}

func NewUserRoutes(handler *handler.UserHandler) *UserRoutes {
	return &UserRoutes{
		handler: handler,
	}
}

func (ur *UserRoutes) Register(r *gin.RouterGroup) {
	users := r.Group("/users")
	{
		users.GET("/me", ur.handler.GetProfile)
		users.PUT("/me", ur.handler.UpdateProfile)
		users.PUT("/me/change-password", ur.handler.ChangePassword)
	}
}
