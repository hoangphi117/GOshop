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
	"google.golang.org/grpc"
	"net"

	"goshop/contracts/events"
	ratingconsumer "goshop/services/catalog/internal/consumer"
	catalogsqlc "goshop/services/catalog/internal/db/sqlc"
	cataloggrpc "goshop/services/catalog/internal/grpc"
	cataloghandler "goshop/services/catalog/internal/handler"
	"goshop/services/catalog/internal/migrations"
	catalogrepo "goshop/services/catalog/internal/repository"
	catalogroutes "goshop/services/catalog/internal/routes"
	catalogservice "goshop/services/catalog/internal/service"
	inventorycontract "goshop/services/inventory/contract"
	"goshop/shared/config"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/db"
	"goshop/shared/pkg/kafka"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/metrics"
	"goshop/shared/pkg/search"
	"goshop/shared/pkg/telemetry"
	"goshop/shared/pkg/utils"
	"goshop/shared/pkg/validation"
	catalogv1 "goshop/shared/proto/catalog/v1"
)

type App struct {
	cfg             *config.Config
	server          *http.Server
	grpcServer      *grpc.Server
	grpcListener    net.Listener
	catalogService  catalogservice.CatalogService
	inventoryClient inventorycontract.InventoryClient
}

func New(cfg *config.Config) (*App, error) {
	if err := validation.InitValidator(); err != nil {
		return nil, fmt.Errorf("failed to initialize validator: %w", err)
	}

	r := gin.Default()
	r.GET("/metrics", metrics.Handler(nil))

	if err := db.InitDBWithConfig(cfg, "catalog"); err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to initialize database for catalog service")
		return nil, fmt.Errorf("database init failed: %w", err)
	}

	// Run embedded migrations
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.DBName, cfg.DB.SSLMode)
	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Could not load embedded migrations source for catalog service")
	} else {
		m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, connStr)
		if err != nil {
			logger.Log.Warn().Err(err).Msg("Could not initialize migrate instance for catalog service")
		} else {
			if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
				logger.Log.Error().Err(err).Msg("Failed to execute database migrations for catalog service")
			} else {
				logger.Log.Info().Msg("Catalog service database migrations applied successfully")
			}
			_, _ = m.Close()
		}
	}

	dbPool := db.DBPool
	catalogRepo := catalogrepo.NewSqlCatalogRepository(dbPool)

	redisClient := config.NewRedisClient()
	cacheRedisService := cache.NewRedisCacheService(redisClient)

	searchClient, err := search.NewClient(cfg.Meilisearch.Host, cfg.Meilisearch.MasterKey, cfg.Meilisearch.IndexProducts)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to initialize Meilisearch client for catalog service")
	} else {
		// Without this the index has documents but no filterable/sortable
		// attributes, so every filtered search is rejected with a 400, the
		// service falls back to Postgres ILIKE on product_name, and category
		// words ("laptop") match nothing. SetupIndex existed but was never
		// called, which is how that stayed invisible.
		go func() {
			if err := searchClient.SetupIndex(context.Background()); err != nil {
				logger.Log.Warn().Err(err).Msg("Failed to configure Meilisearch products index")
			}
		}()
	}

	inventoryGRPCAddr := utils.GetEnv("INVENTORY_GRPC_ADDR", "localhost:50054")
	inventoryClient, err := inventorycontract.NewGRPCInventoryClient(inventoryGRPCAddr)
	if err != nil {
		logger.Log.Warn().Err(err).Str("addr", inventoryGRPCAddr).Msg("Could not connect to Inventory gRPC service")
	}

	svc := catalogservice.NewCatalogService(catalogRepo, dbPool, cacheRedisService, searchClient, inventoryClient)
	h := cataloghandler.NewCatalogHandler(svc)

	catalogroutes.RegisterRoutes(r, h)

	// Consume review rating updates from the Review service (reviews live in a separate DB).
	ratingQuerier := catalogsqlc.New(dbPool)
	ratingConsumer := ratingconsumer.NewRatingConsumer(ratingQuerier, cacheRedisService)
	if rmq, err := kafka.NewKafkaBroker(utils.GetEnv("KAFKA_BROKERS", ""), utils.NewLoggerWithPath("catalog", "catalog_kafka.log", "info")); err == nil {
		go func() {
			if err := rmq.Consume(context.Background(), events.QueueReviewRatingUpdated, ratingConsumer.HandleEvent); err != nil {
				logger.Log.Error().Err(err).Msg("Catalog review-rating consumer stopped")
			}
		}()
	} else {
		logger.Log.Warn().Err(err).Msg("Kafka unavailable, review rating updates disabled")
	}

	port := utils.GetEnv("CATALOG_SERVER_PORT", "8086")
	server := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	grpcPort := utils.GetEnv("CATALOG_GRPC_PORT", "50055")
	grpcLis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		logger.Log.Fatal().Err(err).Msgf("Failed to listen on gRPC port %s", grpcPort)
		return nil, fmt.Errorf("failed to listen on gRPC port %s: %w", grpcPort, err)
	}

	grpcServer := grpc.NewServer(telemetry.ServerOptions()...)
	catalogv1.RegisterCatalogServiceServer(grpcServer, cataloggrpc.NewCatalogGRPCServer(svc))

	return &App{
		cfg:             cfg,
		server:          server,
		grpcServer:      grpcServer,
		grpcListener:    grpcLis,
		catalogService:  svc,
		inventoryClient: inventoryClient,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	a.catalogService.StartOutboxWorker(ctx)

	errChan := make(chan error, 2)

	go func() {
		logger.Log.Info().Msgf("Catalog HTTP Server running on %s", a.server.Addr)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- fmt.Errorf("catalog http server error: %w", err)
		}
	}()

	go func() {
		logger.Log.Info().Msgf("Catalog gRPC Server running on %s", a.grpcListener.Addr().String())
		if err := a.grpcServer.Serve(a.grpcListener); err != nil {
			errChan <- fmt.Errorf("catalog grpc server error: %w", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return err
	case <-quit:
		logger.Log.Info().Msg("Shutting down Catalog service...")
	case <-ctx.Done():
		logger.Log.Info().Msg("Context cancelled, shutting down Catalog service...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error().Err(err).Msg("Catalog HTTP server forced to shutdown")
	}

	a.grpcServer.GracefulStop()

	if a.inventoryClient != nil {
		_ = a.inventoryClient.Close()
	}

	logger.Log.Info().Msg("Catalog service stopped cleanly")
	return nil
}
