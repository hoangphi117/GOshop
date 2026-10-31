package routes

import (
	"goshop/services/notification/internal/handler"

	"github.com/gin-gonic/gin"
)

type NotificationRoutes struct {
	handler *handler.NotificationHandler
}

func NewNotificationRoutes(handler *handler.NotificationHandler) *NotificationRoutes {
	return &NotificationRoutes{handler: handler}
}

func (nr *NotificationRoutes) Register(r *gin.RouterGroup) {
	notifications := r.Group("/notifications")
	{
		notifications.GET("/me", nr.handler.GetMyNotifications)
		notifications.GET("/me/unread", nr.handler.GetUnreadCount)
		notifications.PATCH("/read-all", nr.handler.MarkAllNotificationsAsRead)
		notifications.PATCH("/:uuid/read", nr.handler.MarkNotificationAsRead)
		notifications.DELETE("/:uuid", nr.handler.DeleteNotification)
	}
}

type AdminNotificationRoutes struct {
	handler *handler.NotificationHandler
}

func NewAdminNotificationRoutes(handler *handler.NotificationHandler) *AdminNotificationRoutes {
	return &AdminNotificationRoutes{handler: handler}
}

func (r *AdminNotificationRoutes) Register(rg *gin.RouterGroup) {
	adminGroup := rg.Group("/notifications")
	{
		adminGroup.GET("", r.handler.GetAdminNotifications)
		adminGroup.GET("/unread", r.handler.GetAdminUnreadCount)
		adminGroup.PATCH("/read-all", r.handler.MarkAllAdminNotificationsAsRead)
		adminGroup.PATCH("/:uuid/read", r.handler.MarkNotificationAsRead)
		adminGroup.DELETE("/:uuid", r.handler.DeleteNotification)
		adminGroup.POST("", r.handler.CreateNotification)
		adminGroup.POST("/broadcast", r.handler.BroadcastNotification)
	}
}
