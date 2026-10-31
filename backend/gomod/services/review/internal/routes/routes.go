package routes

import (
	"goshop/services/review/internal/handler"
	"goshop/shared/pkg/auth"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/middleware"
	"goshop/shared/pkg/utils"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts the public and admin review endpoints. The monolith
// gateway proxies /reviews, /products/{uuid}/reviews* and /admin/reviews* here.
func RegisterRoutes(
	r *gin.Engine,
	reviewHandler *handler.ReviewHandler,
	tokenService auth.TokenService,
	cacheService cache.RedisCacheService,
) {
	httpLogger := utils.NewLoggerWithPath("review", "review_http.log", "info")
	recoveryLogger := utils.NewLoggerWithPath("review", "review_recovery.log", "warning")
	rateLimiterLogger := utils.NewLoggerWithPath("review", "review_rate_limiter.log", "warning")

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

	// Public: product reviews (storefront, no auth).
	products := r.Group("/api/v1/products")
	{
		products.GET("/:uuid/reviews", reviewHandler.GetProductReviews)
		products.GET("/:uuid/reviews/summary", reviewHandler.GetProductRatingSummary)
	}

	// Authenticated customer endpoints.
	reviews := r.Group("/api/v1/reviews", middleware.ApiKeyMiddleware(), middleware.AuthMiddleware())
	{
		reviews.POST("", reviewHandler.CreateReview)
		reviews.GET("/me", reviewHandler.GetMyReviews)
		reviews.PUT("/:uuid", reviewHandler.UpdateReview)
		reviews.DELETE("/:uuid", reviewHandler.DeleteReview)
		reviews.POST("/:uuid/helpful", reviewHandler.ToggleHelpfulVote)
		reviews.POST("/:uuid/report", reviewHandler.ReportReview)
	}

	// Admin moderation surface.
	admin := r.Group("/api/v1/admin/reviews",
		middleware.ApiKeyMiddleware(),
		middleware.AuthMiddleware(),
		middleware.RequireRole(middleware.RoleAdmin, middleware.RoleModerator),
	)
	{
		admin.GET("", reviewHandler.AdminGetAllReviews)
		admin.PATCH("/:uuid/status", reviewHandler.UpdateReviewStatus)
		admin.PATCH("/:uuid/reply", reviewHandler.AdminReplyReview)
		admin.POST("/:uuid/dismiss-reports", reviewHandler.AdminDismissReports)
		admin.GET("/:uuid/reports", reviewHandler.GetReviewReports)
		admin.DELETE("/:uuid", reviewHandler.AdminForceDeleteReview)
	}
}
