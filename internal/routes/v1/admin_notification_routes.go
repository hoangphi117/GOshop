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
		// POST /api/v1/admin/notifications
		adminGroup.POST("", r.handler.CreateNotification)
	}
}
