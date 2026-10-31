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

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	catalogcontract "goshop/services/catalog/contract"
	identitycontract "goshop/services/identity/contract"
	ordercontract "goshop/services/order/contract"
	reviewsqlc "goshop/services/review/internal/db/sqlc"
	reviewhandler "goshop/services/review/internal/handler"
	"goshop/services/review/internal/migrations"
	reviewrepo "goshop/services/review/internal/repository"
	reviewroutes "goshop/services/review/internal/routes"
	reviewservice "goshop/services/review/internal/service"
	"goshop/shared/config"
	"goshop/shared/pkg/auth"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/db"
	"goshop/shared/pkg/logger"
	pkgpgx "goshop/shared/pkg/pgx"
	"goshop/shared/pkg/rabbitmq"
	"goshop/shared/pkg/utils"
	"goshop/shared/pkg/validation"
)

type App struct {
	cfg     *config.Config
	server  *http.Server
	rabbit  rabbitmq.RabbitMQService
	closers []func() error
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
			logger.Log.Warn().Err(err).Msg("Failed to configure trusted proxies for review service")
		}
	} else if err := r.SetTrustedProxies([]string{"127.0.0.1", "::1"}); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to configure dev trusted proxies")
	}

	if err := db.InitDBWithConfig(cfg, "review"); err != nil {
		return nil, fmt.Errorf("database init failed: %w", err)
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s&x-migrations-table=schema_migrations_review",
		cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.DBName, cfg.DB.SSLMode)
	if sourceDriver, err := iofs.New(migrations.FS, "."); err != nil {
		logger.Log.Warn().Err(err).Msg("Could not load embedded migrations source for review service")
	} else if m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, connStr); err != nil {
		logger.Log.Warn().Err(err).Msg("Could not initialize migrate instance for review service")
	} else {
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			logger.Log.Error().Err(err).Msg("Failed to execute database migrations for review service")
		} else {
			logger.Log.Info().Msg("Review service database migrations applied successfully")
		}
		_, _ = m.Close()
	}

	redisClient := config.NewRedisClient()
	cacheRedisService := cache.NewRedisCacheService(redisClient)
	tokenService := auth.NewJWTService(cacheRedisService)

	// Cross-domain gRPC ports
	catalogTarget := utils.GetEnv("CATALOG_GRPC_TARGET", "localhost:50055")
	catalogClient, err := catalogcontract.NewGRPCCatalogClient(catalogTarget)
	if err != nil {
		return nil, fmt.Errorf("catalog gRPC client (%s): %w", catalogTarget, err)
	}

	orderTarget := utils.GetEnv("ORDER_GRPC_TARGET", "localhost:50057")
	orderClient, err := ordercontract.NewGRPCOrderClient(orderTarget)
	if err != nil {
		return nil, fmt.Errorf("order gRPC client (%s): %w", orderTarget, err)
	}

	identityClient, err := identitycontract.NewGRPCIdentityClient(cfg.IdentityGRPCAddr, redisClient)
	if err != nil {
		return nil, fmt.Errorf("identity gRPC client (%s): %w", cfg.IdentityGRPCAddr, err)
	}

	rabbitLogger := utils.NewLoggerWithPath("review", "review_rabbitmq.log", "info")
	rabbitMQ, err := rabbitmq.NewRabbitMQService(
		utils.GetEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"), rabbitLogger)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("RabbitMQ unavailable on boot, review events disabled")
		rabbitMQ = nil
	}

	reviewRepo := reviewrepo.NewSqlReviewRepository(reviewsqlc.New(db.DBPool), db.DBPool)
	svc := reviewservice.NewReviewService(
		reviewRepo,
		catalogClient,
		orderClient,
		identityClient,
		rabbitMQ,
		cacheRedisService,
		pkgpgx.NewTxManager(db.DBPool),
	)

	reviewroutes.RegisterRoutes(r, reviewhandler.NewReviewHandler(svc), tokenService, cacheRedisService)

	app := &App{
		cfg:    cfg,
		rabbit: rabbitMQ,
		server: &http.Server{
			Addr:           ":" + utils.GetEnv("REVIEW_SERVER_PORT", "8091"),
			Handler:        r,
			ReadTimeout:    15 * time.Second,
			WriteTimeout:   15 * time.Second,
			MaxHeaderBytes: 1 << 20,
		},
	}
	app.closers = append(app.closers, orderClient.Close, catalogClient.Close)
	return app, nil
}

func (a *App) Run(ctx context.Context) error {
	httpErrors := make(chan error, 1)
	go func() {
		logger.Log.Info().Str("addr", a.server.Addr).Msg("Review HTTP service listening and serving")
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			httpErrors <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-httpErrors:
		return fmt.Errorf("review HTTP server error: %w", err)
	case sig := <-quit:
		logger.Log.Info().Str("signal", sig.String()).Msg("Review service shutting down")
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.server.Shutdown(shutdownCtx); err != nil {
		logger.Log.Warn().Err(err).Msg("Review HTTP server forced shutdown")
	}
	if a.rabbit != nil {
		_ = a.rabbit.Close()
	}
	for _, closeFn := range a.closers {
		_ = closeFn()
	}
	return nil
}
