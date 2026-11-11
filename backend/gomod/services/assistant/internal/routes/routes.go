package routes

import (
	"goshop/services/assistant/internal/handler"
	"goshop/shared/pkg/auth"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/middleware"
	"goshop/shared/pkg/utils"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts the assistant endpoints.
//
// Chat is signed-in only: the model burns tokens per turn, and the cached
// session is keyed by user id, so an anonymous caller would get a shared quota
// and a session that never "ends". AuthMiddleware aborts 401 without a valid
// bearer, which is exactly what the storefront shows as "sign in to chat".
func RegisterRoutes(
	r *gin.Engine,
	h *handler.AssistantHandler,
	tokenService auth.TokenService,
	cacheService cache.RedisCacheService,
) {
	httpLogger := utils.NewLoggerWithPath("assistant", "assistant_http.log", "info")
	recoveryLogger := utils.NewLoggerWithPath("assistant", "assistant_recovery.log", "warning")
	rateLimiterLogger := utils.NewLoggerWithPath("assistant", "assistant_rate_limiter.log", "warning")

	r.Use(gzip.Gzip(gzip.DefaultCompression,
		// Never buffer an event stream: gzip holds bytes until its window
		// fills, which would deliver the whole answer in one lump.
		gzip.WithExcludedPaths([]string{"/api/v1/assistant/stream"}),
	))
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

	// Signed-in only. Without tokenService the group has no auth middleware to
	// install, so mounting nothing is safer than mounting an open route.
	authed := r.Group("/api/v1/assistant", middleware.ApiKeyMiddleware(), middleware.AuthMiddleware())
	{
		authed.POST("/chat", h.Chat)
		authed.POST("/session", h.NewSession)
		// GET so the browser can read it with a plain streaming fetch (or
		// EventSource). Same auth policy: API key plus bearer.
		authed.GET("/stream", h.Stream)
	}
}
