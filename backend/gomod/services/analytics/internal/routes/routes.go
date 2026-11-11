package routes

import (
	"goshop/services/analytics/internal/handler"
	"goshop/shared/pkg/auth"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/middleware"
	"goshop/shared/pkg/utils"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts the admin-only analytics endpoints and the public
// event-ingestion surface. The monolith gateway proxies /admin/analytics/* here.
func RegisterRoutes(
	r *gin.Engine,
	analyticsHandler *handler.AnalyticsHandler,
	eventsHandler *handler.EventHandler,
	tokenService auth.TokenService,
	cacheService cache.RedisCacheService,
) {
	httpLogger := utils.NewLoggerWithPath("analytics", "analytics_http.log", "info")
	recoveryLogger := utils.NewLoggerWithPath("analytics", "analytics_recovery.log", "warning")
	rateLimiterLogger := utils.NewLoggerWithPath("analytics", "analytics_rate_limiter.log", "warning")

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

	// Public customer surface: telemetry ingestion and the search-history read
	// model. Identity is optional (anon cookie or bearer token); nothing here
	// exposes another user's data because the subject always comes from the
	// request's own credentials.
	if eventsHandler != nil {
		pub := r.Group("/api/v1",
			middleware.ApiKeyMiddleware(),
			middleware.AnonCookie(),
			middleware.OptionalAuthMiddleware(),
		)
		{
			pub.POST("/events", eventsHandler.Ingest)
			pub.GET("/search/history", eventsHandler.GetSearchHistory)
			pub.DELETE("/search/history", eventsHandler.DeleteSearchHistory)
		}
	}

	admin := r.Group("/api/v1/admin/analytics",
		middleware.ApiKeyMiddleware(),
		middleware.AuthMiddleware(),
		middleware.RequireRole(middleware.RoleAdmin),
	)
	{
		admin.GET("/overview", analyticsHandler.GetOverview)
		admin.GET("/revenue-chart", analyticsHandler.GetRevenueChart)
		admin.GET("/recent-orders", analyticsHandler.GetRecentOrders)
		admin.GET("/top-products", analyticsHandler.GetTopProducts)
		admin.GET("/order-status-distribution", analyticsHandler.GetOrderStatusDistribution)
		admin.GET("/low-stock-alerts", analyticsHandler.GetLowStockProducts)
		admin.GET("/payment-methods", analyticsHandler.GetPaymentMethodBreakdown)
	}
}
