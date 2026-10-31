package routes

import (
	"goshop/services/coupon/internal/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, couponHandler *handler.CouponHandler) {
	api := r.Group("/api/v1/coupons")
	{
		api.POST("/preview", couponHandler.Preview)
		api.GET("/code/:code", couponHandler.GetByCode)
		api.GET("/:uuid", couponHandler.GetByUUID)
		api.GET("", couponHandler.List)
		api.POST("", couponHandler.Create)
		api.PUT("/:uuid", couponHandler.Update)
		api.DELETE("/:uuid", couponHandler.Delete)
	}

	admin := r.Group("/api/v1/admin/coupons")
	{
		admin.GET("", couponHandler.List)
		admin.POST("", couponHandler.Create)
		admin.GET("/:uuid", couponHandler.GetByUUID)
		admin.PUT("/:uuid", couponHandler.Update)
		admin.DELETE("/:uuid", couponHandler.Delete)
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "coupon"})
	})
}
