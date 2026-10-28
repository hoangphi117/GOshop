package routes

import (
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/middleware"
	"goshop/pkg/utils"
	"goshop/services/chat/internal/handler"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts the chat HTTP surface. The monolith gateway proxies
// /api/v1/chat/* and /api/v1/admin/chat/* here. Auth is enforced locally so the
// service stays safe if it is ever reached directly.
func RegisterRoutes(
	r *gin.Engine,
	chatHandler *handler.ChatHandler,
	tokenService auth.TokenService,
	cacheService cache.RedisCacheService,
) {
	httpLogger := utils.NewLoggerWithPath("chat_http.log", "info")
	recoveryLogger := utils.NewLoggerWithPath("chat_recovery.log", "warning")
	rateLimiterLogger := utils.NewLoggerWithPath("chat_rate_limiter.log", "warning")

	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.Use(
		middleware.RateLimiterMiddleware(rateLimiterLogger),
		middleware.CORSMiddleware(),
		middleware.TraceMiddleware(),
		middleware.LoggerMiddleware(httpLogger),
		middleware.RecoveryMiddleware(recoveryLogger),
	)

	if tokenService != nil {
		middleware.InitAuthMiddleware(tokenService, cacheService)
	}

	// `/chat/shop-status` is public so the storefront can render the badge
	// before login, matching the monolith's public route group.
	public := r.Group("/api/v1/chat")
	{
		public.GET("/shop-status", chatHandler.GetShopStatus)
	}

	storefront := r.Group("/api/v1/chat",
		middleware.ApiKeyMiddleware(),
		middleware.AuthMiddleware(),
	)
	{
		storefront.GET("/conversation", chatHandler.GetMyConversation)
		storefront.GET("/messages", chatHandler.GetMyMessages)
		storefront.POST("/messages", chatHandler.SendMessage)
		storefront.PATCH("/messages/read", chatHandler.MarkAsRead)
		storefront.PATCH("/messages/:id/recall", chatHandler.RecallMessage)
		storefront.GET("/unread-count", chatHandler.GetUnreadCount)
		storefront.POST("/typing", chatHandler.SendTyping)
	}

	admin := r.Group("/api/v1/admin/chat",
		middleware.ApiKeyMiddleware(),
		middleware.AuthMiddleware(),
		middleware.RequireRole(middleware.RoleAdmin, middleware.RoleModerator),
	)
	{
		admin.GET("/conversations", chatHandler.AdminListConversations)
		admin.GET("/conversations/:id", chatHandler.AdminGetConversation)
		admin.GET("/conversations/:id/messages", chatHandler.AdminListMessages)
		admin.POST("/conversations/:id/messages", chatHandler.AdminSendMessage)
		admin.PATCH("/conversations/:id/read", chatHandler.AdminMarkAsRead)
		admin.PATCH("/conversations/:id/close", chatHandler.AdminCloseConversation)
		admin.PATCH("/conversations/:id/reopen", chatHandler.AdminReopenConversation)
		admin.PATCH("/messages/:id/recall", chatHandler.AdminRecallMessage)
		admin.GET("/unread-count", chatHandler.AdminGetUnreadCount)
		admin.POST("/conversations/:id/typing", chatHandler.AdminSendTyping)
		admin.POST("/users/:user_uuid/conversation", chatHandler.AdminGetOrCreateUserConversation)
	}
}
