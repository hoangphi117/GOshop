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

func RegisterRoutes(r *gin.Engine, authService auth.TokenService, cacheService cache.RedisCacheService, normalRoutes []Route, adminRoutes []Route) {

	httpLogger := utils.NewLoggerWithPath("http.log", "info")
	recoveryLogger := utils.NewLoggerWithPath("recovery.log", "warning")
	rateLimiterLogger := utils.NewLoggerWithPath("rate_limiter.log", "warning")

	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.Use(
		middleware.RateLimiterMiddleware(rateLimiterLogger),
		middleware.CORSMiddleware(),
		middleware.TraceMiddleware(),
		middleware.LoggerMiddleware(httpLogger),
		middleware.RecoveryMiddleware(recoveryLogger),
	)

	middleware.InitAuthMiddleware(authService, cacheService)

	v1api := r.Group("/api/v1")

	public := v1api.Group("")

	protected := v1api.Group("")
	protected.Use(middleware.ApiKeyMiddleware(), middleware.AuthMiddleware())

	admin := v1api.Group("/admin")
	admin.Use(middleware.ApiKeyMiddleware(), middleware.AuthMiddleware())
	admin.Use(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleModerator))

	for _, route := range normalRoutes {
		switch r := route.(type) {
		case *v1routes.AuthRoutes, *v1routes.CategoryRoutes, *v1routes.ProductRoutes,
			*v1routes.PaymentWebhookRoutes, *v1routes.ReviewPublicRoutes,
			*v1routes.ShippingWebhookRoutes, *v1routes.WSRoutes, *v1routes.BannerRoutes:
			r.Register(public)
		default:
			r.Register(protected)
		}
	}

	for _, route := range adminRoutes {
		route.Register(admin)
	}

	r.NoRoute(func(ctx *gin.Context) {
		ctx.JSON(404, gin.H{
			"error": "Not found",
			"path":  ctx.Request.URL.Path,
		})
	})
}
