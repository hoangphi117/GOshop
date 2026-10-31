package routes

import (
	"goshop/services/order/internal/handler"
	"goshop/shared/pkg/auth"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/middleware"
	"goshop/shared/pkg/utils"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(
	r *gin.Engine,
	orderHandler *handler.OrderHandler,
	tokenService auth.TokenService,
	cacheService cache.RedisCacheService,
) {
	httpLogger := utils.NewLoggerWithPath("order", "order_http.log", "info")
	recoveryLogger := utils.NewLoggerWithPath("order", "order_recovery.log", "warning")
	rateLimiterLogger := utils.NewLoggerWithPath("order", "order_rate_limiter.log", "warning")

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

	api := r.Group("/api/v1")

	// Customer protected routes
	orders := api.Group("/orders")
	if tokenService != nil {
		orders.Use(middleware.ApiKeyMiddleware(), middleware.AuthMiddleware())
	}
	{
		orders.GET("", orderHandler.GetMyOrders)
		orders.GET("/returns", orderHandler.GetMyOrderReturns)
		orders.GET("/:uuid", orderHandler.GetOrderByUUID)
		orders.POST("", orderHandler.CreateOrder)
		orders.PATCH("/:uuid/cancel", orderHandler.CancelOrder)
		orders.PUT("/:uuid/shipping-info", orderHandler.UpdateOrderShippingInfo)
		orders.POST("/:uuid/received", orderHandler.ConfirmOrderDelivery)
		orders.POST("/:uuid/confirm-delivery", orderHandler.ConfirmOrderDelivery)
		orders.GET("/:uuid/invoice", orderHandler.DownloadInvoice)

		orders.GET("/:uuid/timeline", orderHandler.GetOrderTimeline)
		orders.POST("/calculate-fee", orderHandler.CalculateShippingFee)
		orders.POST("/:uuid/return", orderHandler.RequestOrderReturn)
		orders.GET("/:uuid/return", orderHandler.GetOrderReturnDetails)
		orders.PATCH("/:uuid/return/cancel", orderHandler.CancelOrderReturn)
		orders.POST("/:uuid/return/shipping-info", orderHandler.SubmitReturnShipping)
		orders.POST("/:uuid/return/shipping", orderHandler.SubmitReturnShipping)
		orders.PUT("/:uuid/return/bank-account", orderHandler.UpdateReturnBankAccount)
		orders.POST("/:uuid/sync-tracking", orderHandler.SyncOrderTracking)
	}

	// Admin protected routes
	admin := api.Group("/admin")
	if tokenService != nil {
		admin.Use(middleware.ApiKeyMiddleware(), middleware.AuthMiddleware())
		admin.Use(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleModerator))
	}
	{
		adminOrders := admin.Group("/orders")
		{
			adminOrders.GET("", orderHandler.GetAllOrders)
			adminOrders.GET("/:uuid", orderHandler.GetOrderByUUID)
			adminOrders.GET("/:uuid/invoice", orderHandler.DownloadInvoice)
			adminOrders.PATCH("/:uuid", orderHandler.UpdateOrderStatus)
			adminOrders.PATCH("/:uuid/payment-status", orderHandler.UpdateOrderPaymentStatus)

			// Shipping
			adminOrders.POST("/:uuid/shipping", orderHandler.CreateShipping)
			adminOrders.POST("/:uuid/shipping/cancel", orderHandler.CancelShipping)
			adminOrders.GET("/:uuid/shipping/print-bill", orderHandler.GetPrintBill)
			adminOrders.POST("/:uuid/sync-tracking", orderHandler.SyncOrderTracking)
			adminOrders.POST("/sync-active-tracking", orderHandler.AdminSyncActiveTracking)
			adminOrders.POST("/bulk-status", orderHandler.BulkUpdateOrderStatus)
			adminOrders.POST("/bulk-cancel", orderHandler.BulkCancelOrders)
		}

		adminReturns := admin.Group("/returns")
		{
			adminReturns.GET("", orderHandler.AdminGetReturns)
			adminReturns.GET("/:uuid", orderHandler.AdminGetReturnDetail)
			adminReturns.PATCH("/:uuid", orderHandler.AdminUpdateReturnStatus)
			adminReturns.POST("/:uuid/refund", orderHandler.AdminProcessReturnRefund)
		}
	}

	// Shipping Webhook & Public Location routes
	shipping := api.Group("/shipping")
	{
		shipping.POST("/webhook/ghn", orderHandler.HandleShippingWebhook)
		shipping.POST("/calculate-fee", orderHandler.CalculateShippingFee)
		shipping.GET("/provinces", orderHandler.GetProvinces)
		shipping.GET("/districts", orderHandler.GetDistricts)
		shipping.GET("/districts/:province_id", orderHandler.GetDistricts)
		shipping.GET("/wards", orderHandler.GetWards)
		shipping.GET("/wards/:district_id", orderHandler.GetWards)
	}
}
