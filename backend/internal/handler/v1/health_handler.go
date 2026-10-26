package v1handler

import (
	"context"
	"fmt"
	"goshop/pkg/utils"
	"goshop/pkg/discord"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/storage"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

var appStartTime = time.Now()

type HealthHandler struct {
	dbPool          *pgxpool.Pool
	redisClient     *redis.Client
	rabbitmqService rabbitmq.RabbitMQService
	storageService  storage.StorageService
	discordNotifier discord.Notifier
	logger          *zerolog.Logger
}

func NewHealthHandler(
	dbPool *pgxpool.Pool,
	redisClient *redis.Client,
	rabbitmqService rabbitmq.RabbitMQService,
	storageService storage.StorageService,
	discordNotifier discord.Notifier,
	logger *zerolog.Logger,
) *HealthHandler {
	return &HealthHandler{
		dbPool:          dbPool,
		redisClient:     redisClient,
		rabbitmqService: rabbitmqService,
		storageService:  storageService,
		discordNotifier: discordNotifier,
		logger:          logger,
	}
}

// HealthCheck godoc
// @Summary      Comprehensive System Healthcheck
// @Description  Validates connectivity of Database, Redis, RabbitMQ, and Object Storage. Sends Discord alerts on degradation.
// @Tags         Health
// @Produce      json
// @Success      200 {object} map[string]any
// @Failure      503 {object} map[string]any
// @Router       /healthz [get]
func (h *HealthHandler) HealthCheck(c *gin.Context) {
	h.checkDependencies(c, true)
}

func (h *HealthHandler) checkDependencies(c *gin.Context, alertOnFailure bool) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 4*time.Second)
	defer cancel()

	services := make(map[string]string)
	isHealthy := true
	var errorDetails []string

	// 1. Check PostgreSQL Database Pool
	if h.dbPool != nil {
		if err := h.dbPool.Ping(ctx); err != nil {
			services["database"] = "down"
			isHealthy = false
			errorDetails = append(errorDetails, fmt.Sprintf("PostgreSQL: %v", err))
		} else {
			services["database"] = "up"
		}
	} else {
		services["database"] = "unconfigured"
	}

	// 2. Check Redis Cache & PubSub
	if h.redisClient != nil {
		if err := h.redisClient.Ping(ctx).Err(); err != nil {
			services["redis"] = "down"
			isHealthy = false
			errorDetails = append(errorDetails, fmt.Sprintf("Redis: %v", err))
		} else {
			services["redis"] = "up"
		}
	} else {
		services["redis"] = "unconfigured"
	}

	// 3. Check RabbitMQ Broker
	if h.rabbitmqService != nil {
		if !h.rabbitmqService.IsHealthy() {
			services["rabbitmq"] = "down"
			isHealthy = false
			errorDetails = append(errorDetails, "RabbitMQ: connection closed")
		} else {
			services["rabbitmq"] = "up"
		}
	} else {
		services["rabbitmq"] = "unconfigured"
	}

	// 4. Check Storage Provider (S3 / Cloudflare R2 / MinIO / Local)
	if h.storageService != nil {
		if !h.storageService.IsHealthy(ctx) {
			services["storage"] = "down"
			isHealthy = false
			errorDetails = append(errorDetails, "Storage: healthcheck failed")
		} else {
			services["storage"] = "up"
		}
	} else {
		services["storage"] = "unconfigured"
	}

	uptime := time.Since(appStartTime).Round(time.Second).String()

	response := gin.H{
		"status":         "healthy",
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"uptime":         uptime,
		"uptime_seconds": int64(time.Since(appStartTime).Seconds()),
		"services":       services,
	}

	if !isHealthy {
		response["status"] = "degraded"

		// Send alert to Discord asynchronously
		if alertOnFailure && h.discordNotifier != nil {
			go func() {
				detailMsg := ""
				if len(errorDetails) > 0 {
					detailMsg = fmt.Sprintf("Errors: %v", errorDetails)
				}
				_ = h.discordNotifier.SendHealthStatus(context.Background(), false, services, detailMsg)
			}()
		}

		if h.logger != nil {
			h.logger.Warn().
				Str("trace_id", c.GetString("trace_id")).
				Interface("services", services).
				Strs("errors", errorDetails).
				Msg("Health check degraded")
		}
		c.JSON(http.StatusServiceUnavailable, response)
		return
	}

	if h.logger != nil {
		h.logger.Info().
			Str("trace_id", c.GetString("trace_id")).
			Interface("services", services).
			Msg("Health check passed")
	}
	c.JSON(http.StatusOK, response)
}

// LivenessCheck godoc (Kubernetes / Docker Liveness Probe)
func (h *HealthHandler) LivenessCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "alive",
		"uptime": time.Since(appStartTime).Round(time.Second).String(),
	})
}

// ReadinessCheck godoc (Kubernetes / Load Balancer Readiness Probe)
func (h *HealthHandler) ReadinessCheck(c *gin.Context) {
	h.checkDependencies(c, false)
}

// TestDiscordAlert godoc
// @Summary      Test Discord Webhook Alerting
// @Description  Manually triggers a test alert to verify Discord Webhook configuration
// @Tags         Health
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Produce      json
// @Success      200 {object} utils.APIResponse{data=map[string]any}
// @Router       /admin/health/test-discord [post]
func (h *HealthHandler) TestDiscordAlert(c *gin.Context) {
	if h.discordNotifier == nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "Discord notifier is not configured"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	fields := map[string]string{
		"Triggered By": c.GetString("user_email"),
		"Environment":  utils.GetEnv("APP_ENV", "development"),
		"Timestamp":    time.Now().UTC().Format(time.RFC3339),
	}

	err := h.discordNotifier.SendAlert(ctx, "Test Discord Notification", "This is a verification test from GOshop Health Monitoring System.", discord.SeverityInfo, fields)
	if err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeInternal, "Failed to send discord alert: "+err.Error()))
		return
	}

	utils.ResponseSuccess(c, http.StatusOK, "Discord test alert sent successfully", gin.H{
		"sent": true,
		"time": time.Now().UTC().Format(time.RFC3339),
	})
}
