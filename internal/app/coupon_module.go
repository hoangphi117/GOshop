package app

import (
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
)

type CouponModule struct {
	routes      []routes.Route
	adminRoutes []routes.Route
}

func NewCouponModule(ctx *ModuleContext) *CouponModule {

	couponRepo := repository.NewSqlCouponRepository(ctx.DB)

	couponService := v1service.NewCouponService(couponRepo, ctx.Redis)

	couponHandler := v1handler.NewCouponHandler(couponService)

	couponRoutes := v1routes.NewCouponRoutes(couponHandler)
	adminCouponRoutes := v1routes.NewAdminCouponRoutes(couponHandler)

	return &CouponModule{
		routes:      []routes.Route{couponRoutes},
		adminRoutes: []routes.Route{adminCouponRoutes},
	}

}

func (m *CouponModule) Routes() []routes.Route {
	return m.routes
}

func (m *CouponModule) AdminRoutes() []routes.Route {
	return m.adminRoutes
}
