package routes

import (
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/middleware"
	"goshop/pkg/utils"
	"goshop/services/banner/internal/handler"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(
	r *gin.Engine,
	bannerHandler *handler.BannerHandler,
	tokenService auth.TokenService,
	cacheService cache.RedisCacheService,
) {
	httpLogger := utils.NewLoggerWithPath("banner", "banner_http.log", "info")
	recoveryLogger := utils.NewLoggerWithPath("banner", "banner_recovery.log", "warning")
	rateLimiterLogger := utils.NewLoggerWithPath("banner", "banner_rate_limiter.log", "warning")

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

	// Public: active banners for the storefront
	banners := r.Group("/api/v1/banners")
	{
		banners.GET("", bannerHandler.GetActiveBanners)
	}

	// Admin: full banner CRUD (same guard as the monolith's /admin group)
	admin := r.Group("/api/v1/admin/banners")
	if tokenService != nil {
		admin.Use(middleware.ApiKeyMiddleware(), middleware.AuthMiddleware())
		admin.Use(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleModerator))
	}
	{
		admin.GET("", bannerHandler.AdminGetBanners)
		admin.GET("/:uuid", bannerHandler.AdminGetBannerByUUID)
		admin.POST("", bannerHandler.AdminCreateBanner)
		admin.PUT("/:uuid", bannerHandler.AdminUpdateBanner)
		admin.PATCH("/:uuid/status", bannerHandler.AdminUpdateBannerStatus)
		admin.DELETE("/:uuid", bannerHandler.AdminSoftDeleteBanner)
		admin.PUT("/:uuid/restore", bannerHandler.AdminRestoreBanner)
		admin.DELETE("/:uuid/trash", bannerHandler.AdminTrashBanner)
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "banner"})
	})
}
