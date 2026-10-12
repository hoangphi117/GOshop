package routes

import (
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/middleware"
	"goshop/pkg/utils"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

type Route interface {
	Register(r *gin.RouterGroup)
}

func RegisterRoutes(
	r *gin.Engine,
	authService auth.TokenService,
	cacheService cache.RedisCacheService,
	userRoutes []Route,
	adminRoutes []Route,
) {
	httpLogger := utils.NewLoggerWithPath("notification_http.log", "info")
	recoveryLogger := utils.NewLoggerWithPath("notification_recovery.log", "warning")
	rateLimiterLogger := utils.NewLoggerWithPath("notification_rate_limiter.log", "warning")

	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.Use(
		middleware.RateLimiterMiddleware(rateLimiterLogger),
		middleware.CORSMiddleware(),
		middleware.TraceMiddleware(),
		middleware.LoggerMiddleware(httpLogger),
		middleware.RecoveryMiddleware(recoveryLogger),
	)

	middleware.InitAuthMiddleware(authService, cacheService)

	// Base API group
	v1api := r.Group("/api/v1")

	// Protected routes for authenticated users
	protected := v1api.Group("")
	protected.Use(middleware.ApiKeyMiddleware(), middleware.AuthMiddleware())

	for _, route := range userRoutes {
		route.Register(protected)
	}

	// Protected routes for admins
	admin := v1api.Group("/admin")
	admin.Use(middleware.ApiKeyMiddleware(), middleware.AuthMiddleware())
	admin.Use(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleModerator))

	for _, route := range adminRoutes {
		route.Register(admin)
	}
}
