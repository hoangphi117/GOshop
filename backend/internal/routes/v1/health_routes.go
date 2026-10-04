package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type HealthRoutes struct {
	handler *v1handler.HealthHandler
}

func NewHealthRoutes(handler *v1handler.HealthHandler) *HealthRoutes {
	return &HealthRoutes{handler: handler}
}

func (hr *HealthRoutes) Register(r *gin.RouterGroup) {
	// Support both GET and HEAD requests (UptimeRobot, AWS ALB, Pingdom send HEAD by default)
	r.GET("/healthz", hr.handler.HealthCheck)
	r.HEAD("/healthz", hr.handler.HealthCheck)

	r.GET("/healthz/live", hr.handler.LivenessCheck)
	r.HEAD("/healthz/live", hr.handler.LivenessCheck)

	r.GET("/healthz/ready", hr.handler.ReadinessCheck)
	r.HEAD("/healthz/ready", hr.handler.ReadinessCheck)
}

type HealthAdminRoutes struct {
	handler *v1handler.HealthHandler
}

func NewHealthAdminRoutes(handler *v1handler.HealthHandler) *HealthAdminRoutes {
	return &HealthAdminRoutes{handler: handler}
}

func (hr *HealthAdminRoutes) Register(r *gin.RouterGroup) {
	r.POST("/health/test-discord", hr.handler.TestDiscordAlert)
}
