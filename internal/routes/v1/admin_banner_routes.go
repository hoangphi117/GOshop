package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type AdminBannerRoutes struct {
	handler *v1handler.BannerHandler
}

func NewAdminBannerRoutes(handler *v1handler.BannerHandler) *AdminBannerRoutes {
	return &AdminBannerRoutes{handler: handler}
}

func (r *AdminBannerRoutes) Register(rg *gin.RouterGroup) {
	banners := rg.Group("/banners")
	{
		banners.GET("", r.handler.AdminGetBanners)
		banners.GET("/:uuid", r.handler.AdminGetBannerByUUID)
		banners.POST("", r.handler.AdminCreateBanner)
		banners.PUT("/:uuid", r.handler.AdminUpdateBanner)
		banners.PATCH("/:uuid/status", r.handler.AdminUpdateBannerStatus)
		banners.DELETE("/:uuid", r.handler.AdminSoftDeleteBanner)
		banners.PUT("/:uuid/restore", r.handler.AdminRestoreBanner)
		banners.DELETE("/:uuid/trash", r.handler.AdminTrashBanner)
	}
}
