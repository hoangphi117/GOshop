package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"goshop/internal/config"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/storage"
	"goshop/pkg/utils"
	"goshop/pkg/validation"
	mediahandler "goshop/services/media/internal/handler"
	mediaroutes "goshop/services/media/internal/routes"
)

// App owns the media service HTTP server. Media is stateless: no database, no
// gRPC Ã¢â‚¬â€ it fronts pkg/storage (local disk / S3 / MinIO / R2) for uploads.
type App struct {
	cfg    *config.Config
	server *http.Server
}

func New(cfg *config.Config) (*App, error) {
	if err := validation.InitValidator(); err != nil {
		return nil, fmt.Errorf("failed to initialize validator: %w", err)
	}

	r := gin.Default()
	if cfg.AppEnv == "production" {
		if err := r.SetTrustedProxies([]string{
			"127.0.0.1", "::1", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
		}); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to configure trusted proxies for media service")
		}
	} else if err := r.SetTrustedProxies([]string{"127.0.0.1", "::1"}); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to configure dev trusted proxies")
	}

	r.MaxMultipartMemory = 8 << 20 // 8MB memory buffer for multipart forms (prevents OOM DoS)

	// Mirror the monolith's media serving: every upload lives in object storage,
	// so legacy /uploads/:filepath requests redirect to the public CDN/S3 URL.
	remoteBaseURL := cfg.Storage.S3.PublicBaseURL
	if remoteBaseURL == "" {
		remoteBaseURL = cfg.Storage.PublicBaseURL
	}
	if remoteBaseURL == "" && cfg.Storage.S3.Endpoint != "" {
		if cfg.Storage.S3.UsePathStyle {
			remoteBaseURL = fmt.Sprintf("%s/%s", strings.TrimRight(cfg.Storage.S3.Endpoint, "/"), cfg.Storage.S3.Bucket)
		} else {
			remoteBaseURL = cfg.Storage.S3.Endpoint
		}
	}
	if remoteBaseURL != "" {
		remoteBaseURL = strings.TrimRight(remoteBaseURL, "/")
		r.GET("/uploads/*filepath", func(c *gin.Context) {
			fp := strings.TrimPrefix(c.Param("filepath"), "/")
			c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("%s/%s", remoteBaseURL, fp))
		})
	}

	storageService, err := storage.NewStorageService(context.Background(), cfg.Storage, logger.Log)
	if err != nil {
		return nil, fmt.Errorf("storage init failed: %w", err)
	}

	rabbitLogger := utils.NewLoggerWithPath("media", "media_rabbit.log", "info")
	rabbitService, err := rabbitmq.NewRabbitMQService(utils.GetEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/"), rabbitLogger)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("RabbitMQ unavailable, media post-processing disabled")
		rabbitService = nil
	}

	redisClient := config.NewRedisClient()
	cacheRedisService := cache.NewRedisCacheService(redisClient)
	tokenService := auth.NewJWTService(cacheRedisService)

	mediaroutes.RegisterRoutes(r, mediahandler.NewUploadHandler(storageService, rabbitService), tokenService, cacheRedisService)

	server := &http.Server{
		Addr:           ":" + utils.GetEnv("MEDIA_SERVER_PORT", "8094"),
		Handler:        r,
		ReadTimeout:    60 * time.Second, // multipart uploads can be slow; matches nginx body timeout
		WriteTimeout:   60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	return &App{cfg: cfg, server: server}, nil
}

func (a *App) Run(ctx context.Context) error {
	httpErrors := make(chan error, 1)

	go func() {
		logger.Log.Info().Str("addr", a.server.Addr).Msg("Media HTTP service listening and serving")
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			httpErrors <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-httpErrors:
		return fmt.Errorf("media HTTP server error: %w", err)
	case sig := <-quit:
		logger.Log.Info().Str("signal", sig.String()).Msg("Media service shutting down")
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		logger.Log.Warn().Err(err).Msg("Media HTTP server forced shutdown")
	}
	return nil
}
