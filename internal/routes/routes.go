package routes

import (
	"goshop/internal/middleware"
	v1routes "goshop/internal/routes/v1"
	"goshop/internal/utils"
	"goshop/pkg/auth"
	"goshop/pkg/cache"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

type Route interface {
	Register(r *gin.RouterGroup)
}

func RegisterRoutes(r *gin.Engine, authService auth.TokenService, cacheService cache.RedisCacheService, routes ...Route) {

	httpLogger := utils.NewLoggerWithPath("../../internal/logs/http.log", "info")
	recoveryLogger := utils.NewLoggerWithPath("../../internal/logs/recovery.log", "warning")
	rateLimiterLogger := utils.NewLoggerWithPath("../../internal/logs/rate_limiter.log", "info")

	r.Use(gzip.Gzip(gzip.DefaultCompression))

	r.Use(
		middleware.RateLimiterMiddleware(rateLimiterLogger),
		middleware.CORSMiddleware(),
		middleware.TraceMiddleware(),
		middleware.LoggerMiddleware(httpLogger),
		middleware.RecoveryMiddleware(recoveryLogger),
		middleware.ApiKeyMiddleware(),
	)

	v1api := r.Group("/api/v1")

	middleware.InitAuthMiddleware(authService, cacheService)
	protected := v1api.Group("")
	protected.Use(middleware.AuthMiddleware())

	for _, route := range routes {

		switch route.(type) {
		case *v1routes.AuthRoutes:
			route.Register(v1api)
		default:
			route.Register(protected)
		}

	}

	r.NoRoute(func(ctx *gin.Context) {
		ctx.JSON(404, gin.H{
			"error": "Not found",
			"path":  ctx.Request.URL.Path,
		})
	})
}
