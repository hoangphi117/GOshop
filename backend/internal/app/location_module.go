package app

import (
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
)

type LocationModule struct {
	routes []routes.Route
}

func NewLocationModule(locationService v1service.LocationService) *LocationModule {
	locationHandler := v1handler.NewLocationHandler(locationService)
	locationRoutes := v1routes.NewLocationRoutes(locationHandler)

	return &LocationModule{
		routes: []routes.Route{locationRoutes},
	}
}

func (m *LocationModule) Routes() []routes.Route {
	return m.routes
}
