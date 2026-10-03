package app

import (
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
)

type UploadModule struct {
	routes      []routes.Route
	adminRoutes []routes.Route
}

func NewUploadModule(ctx *ModuleContext) *UploadModule {
	uploadHandler := v1handler.NewUploadHandler(ctx.Storage)
	uploadRoutes := v1routes.NewUploadRoutes(uploadHandler)

	return &UploadModule{
		routes: []routes.Route{uploadRoutes},
	}
}

func (m *UploadModule) Routes() []routes.Route {
	return m.routes
}
