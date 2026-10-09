package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type LocationRoutes struct {
	handler *v1handler.LocationHandler
}

func NewLocationRoutes(handler *v1handler.LocationHandler) *LocationRoutes {
	return &LocationRoutes{handler: handler}
}

func (lr *LocationRoutes) Register(r *gin.RouterGroup) {
	// Canonical Location endpoints
	location := r.Group("/location")
	{
		location.GET("/provinces", lr.handler.GetProvinces)
		location.GET("/districts", lr.handler.GetDistricts)
		location.GET("/wards", lr.handler.GetWards)
	}

	// Backward-compatible shipping location endpoints (aliased to same LocationHandler)
	shipping := r.Group("/shipping")
	{
		shipping.GET("/provinces", lr.handler.GetProvinces)
		shipping.GET("/districts", lr.handler.GetDistricts)
		shipping.GET("/wards", lr.handler.GetWards)
	}
}
