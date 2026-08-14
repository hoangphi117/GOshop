package app

import (
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
	"goshop/pkg/auth"
)

type AuthModule struct {
	routes routes.Route
}

func NewAuthModule(ctx *ModuleContext, tokenService auth.TokenService) *AuthModule {

	// Initialize repository
	userRepo := repository.NewSqlUserRepository(ctx.DB)

	// Initialize service
	authService := v1service.NewAuthService(userRepo, tokenService)

	// Initialize handler
	authHandler := v1handler.NewAuthHandler(authService)

	// Initialize routes
	authRoutes := v1routes.NewAuthRoutes(authHandler)

	return &AuthModule{routes: authRoutes}
}

func (m *AuthModule) Routes() routes.Route {
	return m.routes
}
