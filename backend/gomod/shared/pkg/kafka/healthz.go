// Package kafka health helpers: one shared /healthz shape for every service
// that consumes Kafka (phase 1 observability). Services register the route
// with two lines instead of duplicating threshold/typing logic.
package kafka

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// LagAlertThreshold mirrors the api health handler: any consumer group above
// this lag flips /healthz to degraded so a stuck consumer pages.
const LagAlertThreshold = 1000

// LagReporter is the minimal surface Healthz needs: the concrete kafkaBroker
// implements it; mocks in tests do not. Same optional pattern as the api
// lagReporter, never a hard Broker interface requirement.
type LagReporter interface {
	LagInfo() map[string]int64
}

// RegisterHealthz mounts GET /healthz on r: lag per consumer group owned by
// this process, degraded 503 when any group exceeds the threshold. ping is an
// optional DB check taking the request context; pass nil to skip it. broker may
// be nil (service booted without Kafka); the check then reports down.
func RegisterHealthz(r *gin.Engine, service string, broker interface{}, ping func(context.Context) error) {
	r.GET("/healthz", func(c *gin.Context) {
		services := gin.H{"database": "up"}
		healthy := true
		if ping != nil {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
			defer cancel()
			if err := ping(ctx); err != nil {
				services["database"] = "down"
				healthy = false
			}
		} else {
			services["database"] = "unconfigured"
		}

		var lagger LagReporter
		if broker != nil {
			if lr, ok := broker.(LagReporter); ok {
				lagger = lr
			}
		}
		if lagger == nil {
			services["messaging"] = "down"
			healthy = false
		} else {
			services["messaging"] = "up"
		}

		status, code := "healthy", http.StatusOK
		if !healthy {
			status, code = "degraded", http.StatusServiceUnavailable
		}
		response := gin.H{"status": status, "service": service, "services": services}
		if lagger != nil {
			if lag := lagger.LagInfo(); len(lag) > 0 {
				response["messaging_lag"] = lag
				for group, n := range lag {
					if n > LagAlertThreshold {
						response["status"] = "degraded"
						code = http.StatusServiceUnavailable
						services[group] = fmt.Sprintf("lag %d exceeds %d", n, LagAlertThreshold)
					}
				}
			}
		}
		c.JSON(code, response)
	})
}
