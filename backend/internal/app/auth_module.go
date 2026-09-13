package app

import (
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/mail"
	"goshop/pkg/rabbitmq"
)

type AuthModule struct {
	routes []routes.Route
}

func NewAuthModule(ctx *ModuleContext, tokenService auth.TokenService, cacheService cache.RedisCacheService, mailService mail.EmailProviderService, rabbitmqService rabbitmq.RabbitMQService, storefrontURL string) *AuthModule {

	// Initialize repository
	userRepo := repository.NewSqlUserRepository(ctx.DB)

	// Initialize service
	authService := v1service.NewAuthService(userRepo, tokenService, cacheService, mailService, rabbitmqService, storefrontURL)

	// Initialize handler
	authHandler := v1handler.NewAuthHandler(authService)

	// Initialize routes
	authRoutes := v1routes.NewAuthRoutes(authHandler)

	return &AuthModule{routes: []routes.Route{authRoutes}}
}

func (m *AuthModule) Routes() []routes.Route {
	return m.routes
}
