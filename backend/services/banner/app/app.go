package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"goshop/internal/config"
	"goshop/internal/db"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/utils"
	"goshop/pkg/validation"
	bannersqlc "goshop/services/banner/internal/db/sqlc"
	bannerhandler "goshop/services/banner/internal/handler"
	"goshop/services/banner/internal/migrations"
	bannerrepo "goshop/services/banner/internal/repository"
	bannerroutes "goshop/services/banner/internal/routes"
	bannerservice "goshop/services/banner/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

type App struct {
	cfg           *config.Config
	server        *http.Server
	bannerService bannerservice.BannerService
}

func New(cfg *config.Config) (*App, error) {
	if err := validation.InitValidator(); err != nil {
		return nil, fmt.Errorf("failed to initialize validator: %w", err)
	}

	r := gin.Default()

	if cfg.AppEnv == "production" {
		if err := r.SetTrustedProxies([]string{
			"127.0.0.1",
			"::1",
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
		}); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to configure trusted proxies for banner service")
		}
	} else {
		if err := r.SetTrustedProxies([]string{"127.0.0.1", "::1"}); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to configure dev trusted proxies")
		}
	}

	if err := db.InitDBWithConfig(cfg, "banner"); err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to initialize database for banner service")
		return nil, fmt.Errorf("database init failed: %w", err)
	}

	// Run embedded migrations
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s&x-migrations-table=schema_migrations_banner",
		cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.DBName, cfg.DB.SSLMode)
	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Could not load embedded migrations source for banner service")
	} else {
		m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, connStr)
		if err != nil {
			logger.Log.Warn().Err(err).Msg("Could not initialize migrate instance for banner service")
		} else {
			if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
				logger.Log.Error().Err(err).Msg("Failed to execute database migrations for banner service")
			} else {
				logger.Log.Info().Msg("Banner service database migrations applied successfully")
			}
			sourceErr, dbErr := m.Close()
			if sourceErr != nil {
				logger.Log.Warn().Err(sourceErr).Msg("Error closing migration source")
			}
			if dbErr != nil {
				logger.Log.Warn().Err(dbErr).Msg("Error closing migration db connection")
			}
		}
	}

	redisClient := config.NewRedisClient()
	cacheRedisService := cache.NewRedisCacheService(redisClient)
	tokenService := auth.NewJWTService(cacheRedisService)

	bannerQuerier := bannersqlc.New(db.DBPool)
	bannerRepo := bannerrepo.NewSqlBannerRepository(bannerQuerier)
	bannerService := bannerservice.NewBannerService(bannerRepo, cacheRedisService)

	bannerHandler := bannerhandler.NewBannerHandler(bannerService)
	bannerroutes.RegisterRoutes(r, bannerHandler, tokenService, cacheRedisService)

	port := utils.GetEnv("BANNER_SERVER_PORT", "8089")
	httpServer := &http.Server{
		Addr:           ":" + port,
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	return &App{
		cfg:           cfg,
		server:        httpServer,
		bannerService: bannerService,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	httpErrors := make(chan error, 1)

	go func() {
		logger.Log.Info().Str("addr", a.server.Addr).Msg("Banner HTTP service listening and serving")
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			httpErrors <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-httpErrors:
		return fmt.Errorf("banner HTTP server error: %w", err)
	case sig := <-quit:
		logger.Log.Info().Str("signal", sig.String()).Msg("Banner service shutting down gracefully")
	case <-ctx.Done():
		logger.Log.Info().Msg("Context canceled, shutting down Banner service")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("banner HTTP server forced shutdown: %w", err)
	}

	logger.Log.Info().Msg("Banner service exited properly")
	return nil
}
