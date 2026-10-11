package app

import (
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
	"goshop/pkg/auth"
)

type UserModule struct {
	routes      []routes.Route
	adminRoutes []routes.Route
}

func NewUserModule(ctx *ModuleContext, tokenService auth.TokenService) *UserModule {
	querier := ctx.DB
	if ctx.IdentityDB != nil {
		querier = ctx.IdentityDB
	}
	userRepo := repository.NewSqlUserRepository(querier)
	outboxRepo := repository.NewSqlOutboxRepository(querier)

	// Initialize service
	userService := v1service.NewUserServiceWithOutbox(userRepo, ctx.Redis, ctx.RabbitMQ, outboxRepo)

	// Initialize handler
	userHandler := v1handler.NewUserHandler(userService)

	// Initialize routes
	userRoutes := v1routes.NewUserRoutes(userHandler)
	adminUserRoutes := v1routes.NewAdminUserRoutes(userHandler)

	return &UserModule{
		routes:      []routes.Route{userRoutes},
		adminRoutes: []routes.Route{adminUserRoutes},
	}
}

func (m *UserModule) Routes() []routes.Route {
	return m.routes
}

func (m *UserModule) AdminRoutes() []routes.Route {
	return m.adminRoutes
}
