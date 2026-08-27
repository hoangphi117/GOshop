package app

import (
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
)

type CartModule struct {
	routes []routes.Route
}

func NewCartModule(ctx *ModuleContext) *CartModule {

	// Initialize repository
	cartRepo := repository.NewSqlCartRepository(ctx.DB)
	userRepo := repository.NewSqlUserRepository(ctx.DB)

	// Initialize service
	cartService := v1service.NewCartService(cartRepo, userRepo, ctx.Redis)

	// Initialize handler
	cartHandler := v1handler.NewCartHandler(cartService)

	// Initialize routes
	cartRoutes := v1routes.NewCartRoutes(cartHandler)

	return &CartModule{
		routes: []routes.Route{cartRoutes}}
}

func (m *CartModule) Routes() []routes.Route {
	return m.routes
}
