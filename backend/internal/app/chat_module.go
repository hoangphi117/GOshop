package app

import (
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
	"goshop/pkg/ws"
)

type ChatModule struct {
	routes      []routes.Route
	adminRoutes []routes.Route
	service     v1service.ChatService
}

func NewChatModule(ctx *ModuleContext, wsHub ws.Hub, redisBridge ws.RedisBridge) *ChatModule {
	chatRepo := repository.NewSqlChatRepository(ctx.DB)
	userRepo := repository.NewSqlUserRepository(ctx.DB)
	orderRepo := repository.NewSqlOrderRepository(ctx.DB)

	chatService := v1service.NewChatService(chatRepo, userRepo, orderRepo, ctx.TxManager, ctx.Redis)
	if svc, ok := chatService.(interface{ SetWSHub(ws.Hub) }); ok {
		svc.SetWSHub(wsHub)
	}
	if svc, ok := chatService.(interface{ SetRedisBridge(ws.RedisBridge) }); ok {
		svc.SetRedisBridge(redisBridge)
	}

	chatHandler := v1handler.NewChatHandler(chatService)
	chatRoutes := v1routes.NewChatRoutes(chatHandler)
	adminChatRoutes := v1routes.NewAdminChatRoutes(chatHandler)

	return &ChatModule{
		routes:      []routes.Route{chatRoutes},
		adminRoutes: []routes.Route{adminChatRoutes},
		service:     chatService,
	}
}

func (m *ChatModule) Service() v1service.ChatService {
	return m.service
}

func (m *ChatModule) Routes() []routes.Route {
	return m.routes
}

func (m *ChatModule) AdminRoutes() []routes.Route {
	return m.adminRoutes
}
