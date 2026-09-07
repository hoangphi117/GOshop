package app

import (
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/ws"
)

type NotificationModule struct {
	routes      []routes.Route
	adminRoutes []routes.Route
}

func NewNotificationModule(ctx *ModuleContext, wsHub ws.Hub, tokenService auth.TokenService, cacheService cache.RedisCacheService) *NotificationModule {

	notificationRepo := repository.NewSqlNotificationRepository(ctx.DB)
	userRepo := repository.NewSqlUserRepository(ctx.DB)

	notificationService := v1service.NewNotificationService(notificationRepo, userRepo, wsHub)

	notificationHandler := v1handler.NewNotificationHandler(notificationService)

	notificationRoutes := v1routes.NewNotificationRoutes(notificationHandler)
	adminNotificationRoutes := v1routes.NewAdminNotificationRoutes(notificationHandler)

	wsHandler := v1handler.NewWSHandler(wsHub, tokenService, cacheService, userRepo)
	wsRoutes := v1routes.NewWSRoutes(wsHandler)

	return &NotificationModule{
		routes:      []routes.Route{notificationRoutes, wsRoutes},
		adminRoutes: []routes.Route{adminNotificationRoutes},
	}

}

func (m *NotificationModule) Routes() []routes.Route {
	return m.routes
}

func (m *NotificationModule) AdminRoutes() []routes.Route {
	return m.adminRoutes
}
