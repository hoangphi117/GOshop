package app

import (
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
)

type NotificationModule struct {
	routes      []routes.Route
	adminRoutes []routes.Route
}

func NewNotificationModule(ctx *ModuleContext) *NotificationModule {

	notificationRepo := repository.NewSqlNotificationRepository(ctx.DB)
	userRepo := repository.NewSqlUserRepository(ctx.DB)

	notificationService := v1service.NewNotificationService(notificationRepo, userRepo)

	notificationHandler := v1handler.NewNotificationHandler(notificationService)

	notificationRoutes := v1routes.NewNotificationRoutes(notificationHandler)
	adminNotificationRoutes := v1routes.NewAdminNotificationRoutes(notificationHandler)

	return &NotificationModule{
		routes:      []routes.Route{notificationRoutes},
		adminRoutes: []routes.Route{adminNotificationRoutes},
	}

}

func (m *NotificationModule) Routes() []routes.Route {
	return m.routes
}

func (m *NotificationModule) AdminRoutes() []routes.Route {
	return m.adminRoutes
}
