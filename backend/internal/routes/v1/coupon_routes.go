package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type CouponRoutes struct {
	handler *v1handler.CouponHandler
}

func NewCouponRoutes(handler *v1handler.CouponHandler) *CouponRoutes {
	return &CouponRoutes{
		handler: handler,
	}
}

func (or *CouponRoutes) Register(r *gin.RouterGroup) {
	coupons := r.Group("/coupons")
	{
		coupons.POST("/preview", or.handler.PreviewCoupon)
	}
}
