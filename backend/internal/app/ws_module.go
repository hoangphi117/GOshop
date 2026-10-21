package app

import (
	"goshop/internal/client"
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/ws"
)

type WSModule struct {
	routes []routes.Route
}

func NewWSModule(
	ctx *ModuleContext,
	wsHub ws.Hub,
	tokenService auth.TokenService,
	cacheService cache.RedisCacheService,
) *WSModule {
	var userRepo repository.UserReader = repository.NewSqlUserRepository(ctx.DB)
	if ctx.IdentityClient != nil {
		userRepo = client.NewIdentityUserReader(ctx.IdentityClient)
	}
	chatRepo := repository.NewSqlChatRepository(ctx.DB)
	wsHandler := v1handler.NewWSHandler(wsHub, tokenService, cacheService, userRepo)
	wsHandler.SetChatRepo(chatRepo)
	wsRoutes := v1routes.NewWSRoutes(wsHandler)

	return &WSModule{
		routes: []routes.Route{wsRoutes},
	}
}

func (m *WSModule) Routes() []routes.Route {
	return m.routes
}

func (m *WSModule) AdminRoutes() []routes.Route {
	return nil
}
