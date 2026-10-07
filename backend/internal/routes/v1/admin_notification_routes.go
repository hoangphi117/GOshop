package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type AdminNotificationRoutes struct {
	handler *v1handler.NotificationHandler
}

func NewAdminNotificationRoutes(handler *v1handler.NotificationHandler) *AdminNotificationRoutes {
	return &AdminNotificationRoutes{handler: handler}
}

func (r *AdminNotificationRoutes) Register(rg *gin.RouterGroup) {
	adminGroup := rg.Group("/notifications")
	{
		// Query & counts
		adminGroup.GET("", r.handler.GetAdminNotifications)
		adminGroup.GET("/unread", r.handler.GetAdminUnreadCount)

		// Mark as read & delete
		adminGroup.PATCH("/read-all", r.handler.MarkAllAdminNotificationsAsRead)
		adminGroup.PATCH("/:uuid/read", r.handler.MarkNotificationAsRead)
		adminGroup.DELETE("/:uuid", r.handler.DeleteNotification)

		// Manual triggers (existing)
		adminGroup.POST("", r.handler.CreateNotification)
		adminGroup.POST("/broadcast", r.handler.BroadcastNotification)
	}
}
