package app

import (
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
)

type UserModule struct {
	routes routes.Route
}

func NewUserModule() *UserModule {

	// Initialize repository
	userRepo := repository.NewSqlUserRepository()

	// Initialize service
	userService := v1service.NewUserService(userRepo)

	// Initialize handler
	userHandler := v1handler.NewUserHandler(userService)

	// Initialize routes
	userRoutes := v1routes.NewUserRoutes(userHandler)

	return &UserModule{routes: userRoutes}
}

func (m *UserModule) Routes() routes.Route {
	return m.routes
}
