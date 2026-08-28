package app

import (
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
)

type OrderModule struct {
	routes      []routes.Route
	adminRoutes []routes.Route
}

func NewOrderModule(ctx *ModuleContext) *OrderModule {

	orderRepo := repository.NewSqlOrderRepository(ctx.DB)
	orderItemRepo := repository.NewSqlOrderItemRepository(ctx.DB)

	productRepo := repository.NewSqlProductRepository(ctx.DB)
	userRepo := repository.NewSqlUserRepository(ctx.DB)

	cartRepo := repository.NewSqlCartRepository(ctx.DB)
	cartService := v1service.NewCartService(cartRepo, userRepo, productRepo, ctx.Redis)
	orderService := v1service.NewOrderService(orderRepo, orderItemRepo, productRepo, userRepo, cartService, cartRepo, ctx.Redis)

	orderHandler := v1handler.NewOrderHandler(orderService)

	orderRoutes := v1routes.NewOrderRoutes(orderHandler)
	adminOrderRoutes := v1routes.NewAdminOrderRoutes(orderHandler)

	return &OrderModule{
		routes:      []routes.Route{orderRoutes},
		adminRoutes: []routes.Route{adminOrderRoutes},
	}

}

func (m *OrderModule) Routes() []routes.Route {
	return m.routes
}

func (m *OrderModule) AdminRoutes() []routes.Route {
	return m.adminRoutes
}
