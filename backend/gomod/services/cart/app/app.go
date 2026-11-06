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
	cartconsumer "goshop/services/cart/internal/consumer"
	cartsqlc "goshop/services/cart/internal/db/sqlc"
	cartgrpc "goshop/services/cart/internal/grpc"
	carthandler "goshop/services/cart/internal/handler"
	"goshop/services/cart/internal/migrations"
	cartrepo "goshop/services/cart/internal/repository"
	cartroutes "goshop/services/cart/internal/routes"
	cartservice "goshop/services/cart/internal/service"
	catalogcontract "goshop/services/catalog/contract"
	"goshop/shared/config"
	"goshop/shared/pkg/auth"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/db"
	"goshop/shared/pkg/eventledger"
	"goshop/shared/pkg/kafka"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/messaging"
	"goshop/shared/pkg/telemetry"
	"goshop/shared/pkg/utils"
	"goshop/shared/pkg/validation"
	cartv1 "goshop/shared/proto/cart/v1"
)

type App struct {
	cfg           *config.Config
	server        *http.Server
	grpcServer    *grpc.Server
	grpcListener  net.Listener
	cartService   cartservice.CartService
	catalogClient catalogcontract.CatalogClient
	broker        messaging.Broker
	eventConsumer messaging.Handler
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
			logger.Log.Warn().Err(err).Msg("Failed to configure trusted proxies for cart service")
		}
	} else {
		if err := r.SetTrustedProxies([]string{"127.0.0.1", "::1"}); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to configure dev trusted proxies")
		}
	}

	r.MaxMultipartMemory = 8 << 20

	if err := db.InitDBWithConfig(cfg, "cart"); err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to initialize database for cart service")
		return nil, fmt.Errorf("database init failed: %w", err)
	}

	// Run embedded migrations
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s&x-migrations-table=schema_migrations_cart",
		cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.DBName, cfg.DB.SSLMode)
	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Could not load embedded migrations source for cart service")
	} else {
		m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, connStr)
		if err != nil {
			logger.Log.Warn().Err(err).Msg("Could not initialize migrate instance for cart service")
		} else {
			if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
				logger.Log.Error().Err(err).Msg("Failed to execute database migrations for cart service")
			} else {
				logger.Log.Info().Msg("Cart service database migrations applied successfully")
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

	// Kafka + event consumer (branch F: cart owns cart cleanup on order.created)
	cartBrokerLogger := utils.NewLoggerWithPath("cart", "cart_kafka.log", "info")
	brokerService, err := kafka.NewKafkaBroker(utils.GetEnv("KAFKA_BROKERS", ""),
		cartBrokerLogger,
	)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to connect Kafka for cart service event consumer")
		return nil, fmt.Errorf("kafka connect failed: %w", err)
	}
	if err := eventledger.Migrate(connStr, "event_ledger_migrations_cart"); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to migrate event ledger for cart service")
		return nil, err
	}

	// Catalog gRPC client
	catalogGRPCAddr := utils.GetEnv("CATALOG_GRPC_ADDR", "localhost:50055")
	catalogClient, err := catalogcontract.NewGRPCCatalogClient(catalogGRPCAddr)
	if err != nil {
		logger.Log.Warn().Err(err).Str("addr", catalogGRPCAddr).Msg("Could not connect to Catalog gRPC service")
	}

	dbPool := db.DBPool
	cartQuerier := cartsqlc.New(dbPool)
	cartRepo := cartrepo.NewSqlCartRepository(cartQuerier)
	cartService := cartservice.NewCartService(cartRepo, catalogClient, cacheRedisService)

	cartHandler := carthandler.NewCartHandler(cartService)
	cartroutes.RegisterRoutes(r, cartHandler, tokenService, cacheRedisService)

	// Phase-1 observability: /healthz reports this process's consumer lag.
	kafka.RegisterHealthz(r, "cart", brokerService, dbPool.Ping)

	port := utils.GetEnv("CART_SERVER_PORT", "8087")
	httpServer := &http.Server{
		Addr:           ":" + port,
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	grpcPort := utils.GetEnv("CART_GRPC_PORT", "50056")
	grpcAddr := fmt.Sprintf(":%s", grpcPort)
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logger.Log.Fatal().Err(err).Str("addr", grpcAddr).Msg("Failed to bind gRPC listener for cart service")
		return nil, fmt.Errorf("failed to listen on grpc addr %s: %w", grpcAddr, err)
	}

	grpcServer := grpc.NewServer(telemetry.ServerOptions()...)
	cartv1.RegisterCartServiceServer(grpcServer, cartgrpc.NewCartGRPCServer(cartService))

	return &App{
		cfg:           cfg,
		server:        httpServer,
		grpcServer:    grpcServer,
		grpcListener:  grpcListener,
		cartService:   cartService,
		catalogClient: catalogClient,
		broker:        brokerService,
		eventConsumer: eventledger.Wrap(
			eventledger.NewRepository(dbPool),
			"cart.order_created",
			cartconsumer.NewOrderCreatedConsumer(cartService).Apply,
			eventledger.NewZerologLogger(cartBrokerLogger),
		),
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	httpErrors := make(chan error, 1)
	grpcErrors := make(chan error, 1)

	// 1. Start HTTP Server
	go func() {
		logger.Log.Info().Str("addr", a.server.Addr).Msg("Cart HTTP service listening and serving")
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			httpErrors <- err
		}
	}()

	// 2. Start gRPC Server
	go func() {
		logger.Log.Info().Str("addr", a.grpcListener.Addr().String()).Msg("Cart gRPC service listening and serving")
		if err := a.grpcServer.Serve(a.grpcListener); err != nil {
			grpcErrors <- err
		}
	}()

	// 3. Start event consumer (cart owns cart cleanup on order.created)
	go func() {
		if err := a.broker.Consume(ctx, events.QueueOrderCreated, a.eventConsumer); err != nil {
			logger.Log.Error().Err(err).Str("queue", events.QueueOrderCreated).Msg("Cart event consumer stopped")
		} else {
			logger.Log.Info().Str("queue", events.QueueOrderCreated).Msg("Cart consuming order.created events")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-httpErrors:
		return fmt.Errorf("cart HTTP server error: %w", err)
	case err := <-grpcErrors:
		return fmt.Errorf("cart gRPC server error: %w", err)
	case sig := <-quit:
		logger.Log.Info().Str("signal", sig.String()).Msg("Cart service shutting down gracefully")
	case <-ctx.Done():
		logger.Log.Info().Msg("Context canceled, shutting down Cart service")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if a.grpcServer != nil {
		logger.Log.Info().Msg("Gracefully stopping Cart gRPC server")
		a.grpcServer.GracefulStop()
	}

	if err := a.broker.Close(); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to close Cart Kafka connection cleanly")
	}

	if a.catalogClient != nil {
		_ = a.catalogClient.Close()
	}

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("cart HTTP server forced shutdown: %w", err)
	}

	logger.Log.Info().Msg("Cart service exited properly")
	return nil
}
