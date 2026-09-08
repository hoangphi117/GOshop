package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type BannerRoutes struct {
	handler *v1handler.BannerHandler
}

func NewBannerRoutes(handler *v1handler.BannerHandler) *BannerRoutes {
	return &BannerRoutes{handler: handler}
}

func (r *BannerRoutes) Register(rg *gin.RouterGroup) {
	banners := rg.Group("/banners")
	{
		banners.GET("", r.handler.GetActiveBanners) // GET /api/v1/banners?position=home_main
	}
}
