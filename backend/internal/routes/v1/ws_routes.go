package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type WSRoutes struct {
	handler *v1handler.WSHandler
}

func NewWSRoutes(handler *v1handler.WSHandler) *WSRoutes {
	return &WSRoutes{
		handler: handler,
	}
}

func (r *WSRoutes) Register(rg *gin.RouterGroup) {
	// Endpoint: GET /api/v1/ws?token=<jwt_access_token>
	rg.GET("/ws", r.handler.HandleWS)
}
