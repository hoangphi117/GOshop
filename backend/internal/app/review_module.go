package app

import (
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
	"goshop/pkg/ws"
)

type ReviewModule struct {
	routes      []routes.Route
	adminRoutes []routes.Route
}

func NewReviewModule(ctx *ModuleContext, wsHub ws.Hub) *ReviewModule {

	// Initialize repository
	reviewRepo := repository.NewSqlReviewRepository(ctx.DB)
	userRepo := repository.NewSqlUserRepository(ctx.DB)
	productRepo := repository.NewSqlProductRepository(ctx.DB)
	orderRepo := repository.NewSqlOrderRepository(ctx.DB)

	notificationRepo := repository.NewSqlNotificationRepository(ctx.DB)
	notificationService := v1service.NewNotificationService(notificationRepo, userRepo, wsHub)

	// Initialize service
	reviewService := v1service.NewReviewService(reviewRepo, userRepo, productRepo, orderRepo, notificationService, ctx.Redis)

	// Initialize handler
	reviewHandler := v1handler.NewReviewHandler(reviewService)

	// Initialize routes
	reviewRoutes := v1routes.NewReviewRoutes(reviewHandler)
	reviewPublicRoutes := v1routes.NewReviewPublicRoutes(reviewHandler)
	adminReviewRoutes := v1routes.NewAdminReviewRoutes(reviewHandler)

	return &ReviewModule{
		routes:      []routes.Route{reviewRoutes, reviewPublicRoutes},
		adminRoutes: []routes.Route{adminReviewRoutes},
	}
}

func (m *ReviewModule) Routes() []routes.Route {
	return m.routes
}

func (m *ReviewModule) AdminRoutes() []routes.Route {
	return m.adminRoutes
}
