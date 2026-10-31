package routes

import (
	"github.com/gin-gonic/gin"

	handler "goshop/cmd/api/handler"
)

type HealthRoutes struct {
	handler *handler.HealthHandler
}

func NewHealthRoutes(handler *handler.HealthHandler) *HealthRoutes {
	return &HealthRoutes{handler: handler}
}

func (hr *HealthRoutes) Register(r *gin.RouterGroup) {
	hr.RegisterPublic(r)
}

// RegisterPublic makes the liveness/readiness probes reachable without auth.
func (hr *HealthRoutes) RegisterPublic(r *gin.RouterGroup) {
	// Support both GET and HEAD requests (UptimeRobot, AWS ALB, Pingdom send HEAD by default)
	r.GET("/healthz", hr.handler.HealthCheck)
	r.HEAD("/healthz", hr.handler.HealthCheck)

	r.GET("/healthz/live", hr.handler.LivenessCheck)
	r.HEAD("/healthz/live", hr.handler.LivenessCheck)

	r.GET("/healthz/ready", hr.handler.ReadinessCheck)
	r.HEAD("/healthz/ready", hr.handler.ReadinessCheck)
}

type HealthAdminRoutes struct {
	handler *handler.HealthHandler
}

func NewHealthAdminRoutes(handler *handler.HealthHandler) *HealthAdminRoutes {
	return &HealthAdminRoutes{handler: handler}
}

func (hr *HealthAdminRoutes) Register(r *gin.RouterGroup) {
	r.POST("/health/test-discord", hr.handler.TestDiscordAlert)
}
