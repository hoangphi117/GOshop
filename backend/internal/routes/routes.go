package routes

import (
	_ "goshop/docs"
	"goshop/internal/middleware"
	v1routes "goshop/internal/routes/v1"
	"goshop/internal/utils"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"net/http"
	"strings"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
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

	// Swagger documentation route
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/swagger", func(ctx *gin.Context) {
		ctx.Redirect(301, "/swagger/index.html")
	})

	// JWKS (JSON Web Key Set) endpoint for offline JWT verification across microservices
	jwksHandler := func(ctx *gin.Context) {
		ctx.Header("Cache-Control", "public, max-age=3600")
		ctx.JSON(http.StatusOK, authService.GetJWKS())
	}
	r.GET("/.well-known/jwks.json", jwksHandler)

	v1api := r.Group("/api/v1")
	v1api.GET("/.well-known/jwks.json", jwksHandler)

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
			*v1routes.ShippingWebhookRoutes, *v1routes.WSRoutes, *v1routes.BannerRoutes, *v1routes.HealthRoutes,
			*v1routes.LocationRoutes:
			r.Register(public)
		default:
			r.Register(protected)
		}
	}

	for _, route := range adminRoutes {
		route.Register(admin)
	}

	r.NoRoute(func(ctx *gin.Context) {
		p := ctx.Request.URL.Path
		if !strings.HasPrefix(p, "/api/v1") && !strings.HasPrefix(p, "/internal") && !strings.HasPrefix(p, "/swagger") && !strings.HasPrefix(p, "/.well-known") {
			ctx.Request.URL.Path = "/api/v1" + p
			r.HandleContext(ctx)
			return
		}
		ctx.JSON(404, gin.H{
			"error": "Not found",
			"path":  ctx.Request.URL.Path,
		})
	})
}

