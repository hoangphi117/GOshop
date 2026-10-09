package app

import (
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
)

type AnalyticsModule struct {
	adminRoutes []routes.Route
}

func NewAnalyticsModule(ctx *ModuleContext) *AnalyticsModule {
	querier := ctx.DB
	if ctx.ReadDB != nil {
		querier = ctx.ReadDB
	}
	analyticsRepo := repository.NewSqlAnalyticsRepository(querier)
	analyticsService := v1service.NewAnalyticsService(analyticsRepo)
	analyticsHandler := v1handler.NewAnalyticsHandler(analyticsService)

	adminAnalyticsRoutes := v1routes.NewAdminAnalyticsRoutes(analyticsHandler)

	return &AnalyticsModule{
		adminRoutes: []routes.Route{adminAnalyticsRoutes},
	}
}

func (m *AnalyticsModule) Routes() []routes.Route {
	return []routes.Route{}
}

func (m *AnalyticsModule) AdminRoutes() []routes.Route {
	return m.adminRoutes
}
