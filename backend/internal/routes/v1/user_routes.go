package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type UserRoutes struct {
	handler        *v1handler.UserHandler
	sessionHandler *v1handler.SessionHandler
}

func NewUserRoutes(handler *v1handler.UserHandler, sessionHandler *v1handler.SessionHandler) *UserRoutes {
	return &UserRoutes{
		handler:        handler,
		sessionHandler: sessionHandler,
	}
}

func (ur *UserRoutes) Register(r *gin.RouterGroup) {
	users := r.Group("/users")
	{
		users.GET("/me", ur.handler.GetProfile)
		users.PUT("/me", ur.handler.UpdateProfile)
		users.PUT("/me/change-password", ur.handler.ChangePassword)

		if ur.sessionHandler != nil {
			users.GET("/sessions", ur.sessionHandler.ListSessions)
			users.DELETE("/sessions/:uuid", ur.sessionHandler.RevokeSession)
			users.DELETE("/sessions", ur.sessionHandler.RevokeAllOtherSessions)
		}
	}
}
