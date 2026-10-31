package routes

import (
	"goshop/services/media/internal/handler"
	"goshop/shared/pkg/auth"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/middleware"
	"goshop/shared/pkg/utils"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts the media HTTP surface. The monolith gateway proxies
// /api/v1/upload* here. Auth is enforced locally so the service stays safe if
// it is ever reached directly.
func RegisterRoutes(
	r *gin.Engine,
	uploadHandler *handler.UploadHandler,
	tokenService auth.TokenService,
	cacheService cache.RedisCacheService,
) {
	httpLogger := utils.NewLoggerWithPath("media", "media_http.log", "info")
	recoveryLogger := utils.NewLoggerWithPath("media", "media_recovery.log", "warning")
	rateLimiterLogger := utils.NewLoggerWithPath("media", "media_rate_limiter.log", "warning")

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

	// Both endpoints require a session, matching the monolith's protected group.
	protected := r.Group("/api/v1",
		middleware.ApiKeyMiddleware(),
		middleware.AuthMiddleware(),
	)
	{
		protected.GET("/upload/presign", uploadHandler.GetPresignedURL)
		protected.POST("/upload/presign", uploadHandler.GetPresignedURL)
		protected.POST("/upload/confirm", uploadHandler.ConfirmUpload)
	}
}
