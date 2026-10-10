package app

import (
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
)

type AddressModule struct {
	routes []routes.Route
}

func NewAddressModule(ctx *ModuleContext) *AddressModule {
	querier := ctx.DB
	if ctx.IdentityDB != nil {
		querier = ctx.IdentityDB
	}

	// Initialize repository
	addressRepo := repository.NewSqlAddressRepository(querier)
	userRepo := repository.NewSqlUserRepository(querier)

	// Initialize service
	addressService := v1service.NewAddressService(addressRepo, userRepo)

	// Initialize handler
	addressHandler := v1handler.NewAddressHandler(addressService)

	// Initialize routes
	addressRoutes := v1routes.NewAddressRoutes(addressHandler)

	return &AddressModule{routes: []routes.Route{addressRoutes}}
}

func (m *AddressModule) Routes() []routes.Route {
	return m.routes
}
