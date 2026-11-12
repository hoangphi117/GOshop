package routes

import (
	"net/http"
	"os"

	"goshop/services/recommendations/internal/handler"
	"goshop/shared/pkg/middleware"
	"goshop/shared/pkg/utils"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(
	r *gin.Engine,
	recommendationHandler *handler.RecommendationHandler,
	personalizedHandler *handler.PersonalizedHandler,
	reembedHandler *handler.ReembedHandler,
) {
	httpLogger := utils.NewLoggerWithPath("recommendations", "recommendations_http.log", "info")
	recoveryLogger := utils.NewLoggerWithPath("recommendations", "recommendations_recovery.log", "warning")
	rateLimiterLogger := utils.NewLoggerWithPath("recommendations", "recommendations_rate_limiter.log", "warning")

	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.Use(
		middleware.RateLimiterMiddleware(rateLimiterLogger),
		middleware.CORSMiddleware(),
		middleware.TraceMiddleware(),
		middleware.LoggerMiddleware(httpLogger),
		middleware.RecoveryMiddleware(recoveryLogger),
	)

	// Public: homepage and product-page shelves. The anon cookie middleware is
	// what gives visitors an identity to personalize by, so it guards the
	// vectors without requiring sign-in.
	shelves := r.Group("")
	shelves.Use(middleware.AnonCookie())
	{
		shelves.GET("/api/v1/recommendations", recommendationHandler.BestSellers)
		shelves.GET("/api/v1/recommendations/personalized", personalizedHandler.ForVisitor)
		shelves.GET("/api/v1/products/similar", personalizedHandler.Similar)
		// Hybrid search replaces the catalog's keyword-only listing route for
		// the storefront. Neither retriever alone can answer both "exact SKU"
		// and "something for a rainy commute", and fusing them here keeps the
		// vectors where they live.
		shelves.GET("/api/v1/products/search", personalizedHandler.Search)
	}

	// Internal: written by the catalog service after a product is published.
	// Not under /api/v1 because it is not part of the public surface and must
	// never be reachable through the gateway's anonymous routes.
	internal := r.Group("/internal")
	internal.Use(internalToken())
	{
		internal.POST("/embeddings/:uuid", reembedHandler.Embed)
	}
}

// internalToken guards the write path with a shared secret. When the secret is
// unset (a dev box) the route stays open, which is the same posture as the rest
// of the dev stack; production compose sets it.
func internalToken() gin.HandlerFunc {
	expected := os.Getenv("INTERNAL_API_TOKEN")

	return func(ctx *gin.Context) {
		if expected == "" {
			ctx.Next()
			return
		}
		if ctx.GetHeader("X-Internal-Token") != expected {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "invalid internal token",
			})
			return
		}
		ctx.Next()
	}
}
