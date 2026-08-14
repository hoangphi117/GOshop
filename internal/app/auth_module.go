package app

import (
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
)

type AuthModule struct {
	routes routes.Route
}

func NewAuthModule(ctx *ModuleContext) *AuthModule {

	// Initialize repository
	userRepo := repository.NewSqlUserRepository(ctx.DB)

	// Initialize service
	authService := v1service.NewAuthService(userRepo)

	// Initialize handler
	authHandler := v1handler.NewAuthHandler(authService)

	// Initialize routes
	authRoutes := v1routes.NewAuthRoutes(authHandler)

	return &AuthModule{routes: authRoutes}
}

func (m *AuthModule) Routes() routes.Route {
	return m.routes
}
