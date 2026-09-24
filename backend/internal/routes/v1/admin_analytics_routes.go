package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type AdminAnalyticsRoutes struct {
	handler *v1handler.AnalyticsHandler
}

func NewAdminAnalyticsRoutes(handler *v1handler.AnalyticsHandler) *AdminAnalyticsRoutes {
	return &AdminAnalyticsRoutes{handler: handler}
}

func (r *AdminAnalyticsRoutes) Register(rg *gin.RouterGroup) {
	analytics := rg.Group("/analytics")
	{
		analytics.GET("/overview", r.handler.GetOverview)
		analytics.GET("/revenue-chart", r.handler.GetRevenueChart)
		analytics.GET("/recent-orders", r.handler.GetRecentOrders)
		analytics.GET("/top-products", r.handler.GetTopProducts)
		analytics.GET("/order-status-distribution", r.handler.GetOrderStatusDistribution)
		analytics.GET("/low-stock-alerts", r.handler.GetLowStockProducts)
		analytics.GET("/payment-methods", r.handler.GetPaymentMethodBreakdown)
		analytics.GET("/cart-funnel", r.handler.GetCartFunnel)
		analytics.GET("/returns", r.handler.GetReturnAnalytics)
		analytics.GET("/category-sales", r.handler.GetCategorySalesBreakdown)
		analytics.GET("/coupons", r.handler.GetCouponPerformance)
	}
}

