package app

import (
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
	"goshop/pkg/cache"
)

type BannerModule struct {
	routes      []routes.Route
	adminRoutes []routes.Route
}

func NewBannerModule(ctx *ModuleContext, cacheService cache.RedisCacheService) *BannerModule {
	bannerRepo := repository.NewSqlBannerRepository(ctx.DB)
	bannerService := v1service.NewBannerService(bannerRepo, cacheService)
	bannerHandler := v1handler.NewBannerHandler(bannerService)

	bannerRoutes := v1routes.NewBannerRoutes(bannerHandler)
	adminBannerRoutes := v1routes.NewAdminBannerRoutes(bannerHandler)

	return &BannerModule{
		routes:      []routes.Route{bannerRoutes},
		adminRoutes: []routes.Route{adminBannerRoutes},
	}
}

func (m *BannerModule) Routes() []routes.Route {
	return m.routes
}

func (m *BannerModule) AdminRoutes() []routes.Route {
	return m.adminRoutes
}
