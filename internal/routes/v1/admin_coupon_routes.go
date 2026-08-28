package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type AdminCouponRoutes struct {
	handler *v1handler.CouponHandler
}

func NewAdminCouponRoutes(handler *v1handler.CouponHandler) *AdminCouponRoutes {
	return &AdminCouponRoutes{
		handler: handler,
	}
}

func (or *AdminCouponRoutes) Register(r *gin.RouterGroup) {
	coupons := r.Group("/coupons")
	{
		coupons.POST("", or.handler.CreateCoupon)
	}
}
