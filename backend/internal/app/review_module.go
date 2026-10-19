package app

import (
	"goshop/internal/client"
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
	"goshop/pkg/rabbitmq"
)

type ReviewModule struct {
	routes      []routes.Route
	adminRoutes []routes.Route
}

func NewReviewModule(ctx *ModuleContext, rabbitmqService rabbitmq.RabbitMQService, orderReader v1service.OrderReader) *ReviewModule {

	// Initialize repository
	reviewRepo := repository.NewSqlReviewRepository(ctx.DB)
	productRepo := repository.NewSqlProductRepository(ctx.DB)
	userReader := client.NewIdentityUserReader(ctx.IdentityClient)

	// Initialize service
	reviewService := v1service.NewReviewServiceWithCatalog(reviewRepo, productRepo, orderReader, rabbitmqService, ctx.Redis, ctx.CatalogClient, userReader)

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
