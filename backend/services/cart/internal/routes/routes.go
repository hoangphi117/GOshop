package routes

import (
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/middleware"
	"goshop/pkg/utils"
	"goshop/services/cart/internal/handler"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(
	r *gin.Engine,
	cartHandler *handler.CartHandler,
	tokenService auth.TokenService,
	cacheService cache.RedisCacheService,
) {
	httpLogger := utils.NewLoggerWithPath("cart_http.log", "info")
	recoveryLogger := utils.NewLoggerWithPath("cart_recovery.log", "warning")
	rateLimiterLogger := utils.NewLoggerWithPath("cart_rate_limiter.log", "warning")

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

	cart := r.Group("/api/v1/cart")
	if tokenService != nil {
		cart.Use(middleware.ApiKeyMiddleware(), middleware.AuthMiddleware())
	}
	{
		cart.GET("", cartHandler.GetMyCart)
		cart.DELETE("", cartHandler.ClearMyCart)
		cart.POST("/items", cartHandler.AddItemToCart)
		cart.PUT("/items/:item_uuid", cartHandler.UpdateCartItemQuantity)
		cart.DELETE("/items/:item_uuid", cartHandler.RemoveItemFromCart)
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "cart"})
	})
}
