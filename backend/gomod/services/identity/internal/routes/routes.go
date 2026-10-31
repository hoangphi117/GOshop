package routes

import (
	"net/http"
	"strings"

	"goshop/services/identity/internal/handler"
	_ "goshop/shared/docs"
	"goshop/shared/pkg/auth"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/middleware"
	"goshop/shared/pkg/utils"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// IdentityRouteDeps holds the handlers and services required for Identity microservice routing
type IdentityRouteDeps struct {
	AuthHandler    *handler.AuthHandler
	UserHandler    *handler.UserHandler
	AddressHandler *handler.AddressHandler
	TokenService   auth.TokenService
	CacheService   cache.RedisCacheService
}

// RegisterRoutes registers all HTTP endpoints for the Identity microservice
func RegisterRoutes(r *gin.Engine, deps IdentityRouteDeps) {
	httpLogger := utils.NewLoggerWithPath("identity", "identity_http.log", "info")
	recoveryLogger := utils.NewLoggerWithPath("identity", "identity_recovery.log", "warning")
	rateLimiterLogger := utils.NewLoggerWithPath("identity", "identity_rate_limiter.log", "warning")

	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.Use(
		middleware.RateLimiterMiddleware(rateLimiterLogger),
		middleware.CORSMiddleware(),
		middleware.TraceMiddleware(),
		middleware.LoggerMiddleware(httpLogger),
		middleware.RecoveryMiddleware(recoveryLogger),
	)

	middleware.InitAuthMiddleware(deps.TokenService, deps.CacheService)

	// Swagger documentation route
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/swagger", func(ctx *gin.Context) {
		ctx.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})

	// JWKS (JSON Web Key Set) endpoint for offline JWT verification across microservices
	jwksHandler := func(ctx *gin.Context) {
		ctx.Header("Cache-Control", "public, max-age=3600")
		ctx.JSON(http.StatusOK, deps.TokenService.GetJWKS())
	}
	r.GET("/.well-known/jwks.json", jwksHandler)

	// Liveness probe. The API gateway's health check probes this endpoint over
	// HTTP instead of opening a pool; without it /healthz reports 503 and nginx
	// disables the upstream after three misses.
	r.GET("/healthz", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"service": "identity", "status": "ok"})
	})

	v1api := r.Group("/api/v1")
	v1api.GET("/.well-known/jwks.json", jwksHandler)

	// 1. Public Routes (Auth)
	public := v1api.Group("")
	authRoutes := NewAuthRoutes(deps.AuthHandler, deps.CacheService)
	authRoutes.Register(public)

	// 2. Protected Routes (Users, Addresses)
	protected := v1api.Group("")
	protected.Use(middleware.ApiKeyMiddleware(), middleware.AuthMiddleware())
	userRoutes := NewUserRoutes(deps.UserHandler)
	userRoutes.Register(protected)
	addressRoutes := NewAddressRoutes(deps.AddressHandler)
	addressRoutes.Register(protected)

	// 3. Admin Routes (Admin Users)
	admin := v1api.Group("/admin")
	admin.Use(middleware.ApiKeyMiddleware(), middleware.AuthMiddleware())
	admin.Use(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleModerator))
	adminUserRoutes := NewAdminUserRoutes(deps.UserHandler)
	adminUserRoutes.Register(admin)

	// 404 fallback handler
	r.NoRoute(func(ctx *gin.Context) {
		p := ctx.Request.URL.Path
		if !strings.HasPrefix(p, "/api/v1") && !strings.HasPrefix(p, "/internal") && !strings.HasPrefix(p, "/swagger") && !strings.HasPrefix(p, "/.well-known") {
			ctx.Request.URL.Path = "/api/v1" + p
			r.HandleContext(ctx)
			return
		}
		ctx.JSON(http.StatusNotFound, gin.H{
			"error": "Not found",
			"path":  ctx.Request.URL.Path,
		})
	})
}
