package app

import (
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
)

type ProductModule struct {
	routes routes.Route
}

func NewProductModule(ctx *ModuleContext) *ProductModule {

	categoryRepo := repository.NewSqlCategoryRepository(ctx.DB)

	categoryService := v1service.NewCategoryService(categoryRepo, ctx.Redis)

	categoryHandler := v1handler.NewCategoryHandler(categoryService)

	categoryRoutes := v1routes.NewCategoryRoutes(categoryHandler)

	return &ProductModule{routes: categoryRoutes}

}

func (m *ProductModule) Routes() routes.Route {
	return m.routes
}
