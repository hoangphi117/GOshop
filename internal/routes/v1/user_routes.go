package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type UserRoutes struct {
	handler *v1handler.UserHandler
}

func NewUserRoutes(handler *v1handler.UserHandler) *UserRoutes {
	return &UserRoutes{
		handler: handler,
	}
}

func (ur *UserRoutes) Register(r *gin.RouterGroup) {
	users := r.Group("/users")
	{
		users.GET("/me", ur.handler.GetProfile)
		users.PUT("/me", ur.handler.UpdateProfile)
	}
}
