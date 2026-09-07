package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type NotificationRoutes struct {
	handler *v1handler.NotificationHandler
}

func NewNotificationRoutes(handler *v1handler.NotificationHandler) *NotificationRoutes {
	return &NotificationRoutes{
		handler: handler,
	}
}

func (nr *NotificationRoutes) Register(r *gin.RouterGroup) {
	notifications := r.Group("/notifications")
	{
		notifications.POST("/me", nr.handler.GetMyNotifications)
		notifications.GET("/me/unread", nr.handler.GetUnreadCount)
		notifications.PATCH("/read-all", nr.handler.MarkAllNotificationsAsRead)
		notifications.PATCH("/:uuid/read", nr.handler.MarkNotificationAsRead)

	}
}
