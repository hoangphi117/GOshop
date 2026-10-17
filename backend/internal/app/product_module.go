package app

import (
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
)

type ProductModule struct {
	routes      []routes.Route
	adminRoutes []routes.Route
}

func NewProductModule(ctx *ModuleContext) *ProductModule {

	categoryRepo := repository.NewSqlCategoryRepository(ctx.DB)
	productRepo := repository.NewSqlProductRepository(ctx.DB)
	outboxRepo := repository.NewSqlOutboxRepository(ctx.DB)

	categoryService := v1service.NewCategoryService(categoryRepo, ctx.Redis)
	productService := v1service.NewProductService(productRepo, categoryRepo, outboxRepo, ctx.Redis, ctx.Search, ctx.TxManager, ctx.InventoryClient)

	categoryHandler := v1handler.NewCategoryHandler(categoryService)
	productHandler := v1handler.NewProductHandler(productService)

	categoryRoutes := v1routes.NewCategoryRoutes(categoryHandler)
	productRoutes := v1routes.NewProductRoutes(productHandler)
	adminCategoryRoutes := v1routes.NewAdminCategoryRoutes(categoryHandler)
	adminProductRoutes := v1routes.NewAdminProductRoutes(productHandler)

	return &ProductModule{
		routes:      []routes.Route{categoryRoutes, productRoutes},
		adminRoutes: []routes.Route{adminCategoryRoutes, adminProductRoutes},
	}

}

func (m *ProductModule) Routes() []routes.Route {
	return m.routes
}

func (m *ProductModule) AdminRoutes() []routes.Route {
	return m.adminRoutes
}
