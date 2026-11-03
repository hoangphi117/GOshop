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

	"goshop/contracts/events"
	analyticsqlc "goshop/services/analytics/internal/db/sqlc"
	analyticshandler "goshop/services/analytics/internal/handler"
	"goshop/services/analytics/internal/migrations"
	analyticsrepo "goshop/services/analytics/internal/repository"
	analyticsroutes "goshop/services/analytics/internal/routes"
	analyticsservice "goshop/services/analytics/internal/service"
	analyticsworker "goshop/services/analytics/internal/worker"
	catalogcontract "goshop/services/catalog/contract"
	identitycontract "goshop/services/identity/contract"
	inventorycontract "goshop/services/inventory/contract"
	ordercontract "goshop/services/order/contract"
	"goshop/shared/config"
	"goshop/shared/pkg/auth"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/db"
	"goshop/shared/pkg/kafka"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/messaging"
	"goshop/shared/pkg/utils"
	"goshop/shared/pkg/validation"
)

// analyticsQueue is the topic queue this service owns. It replaced the
// worker's subscription when analytics moved out of the monolith.
const analyticsQueue = "analytics.order_events"

type App struct {
	cfg     *config.Config
	server  *http.Server
	broker  messaging.Broker
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
			logger.Log.Warn().Err(err).Msg("Failed to configure trusted proxies for analytics service")
		}
	} else if err := r.SetTrustedProxies([]string{"127.0.0.1", "::1"}); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to configure dev trusted proxies")
	}

	if err := db.InitDBWithConfig(cfg, "analytics"); err != nil {
		return nil, fmt.Errorf("database init failed: %w", err)
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s&x-migrations-table=schema_migrations_analytics",
		cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.DBName, cfg.DB.SSLMode)
	if sourceDriver, err := iofs.New(migrations.FS, "."); err != nil {
		logger.Log.Warn().Err(err).Msg("Could not load embedded migrations source for analytics service")
	} else if m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, connStr); err != nil {
		logger.Log.Warn().Err(err).Msg("Could not initialize migrate instance for analytics service")
	} else {
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			logger.Log.Error().Err(err).Msg("Failed to execute database migrations for analytics service")
		} else {
			logger.Log.Info().Msg("Analytics service database migrations applied successfully")
		}
		_, _ = m.Close()
	}

	redisClient := config.NewRedisClient()
	cacheRedisService := cache.NewRedisCacheService(redisClient)
	tokenService := auth.NewJWTService(cacheRedisService)

	// Cross-domain gRPC ports. Analytics owns no other service's data; every
	// external dimension comes through one of these clients.
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
	// CountCustomersCreatedBetween lives on the concrete gRPC client only (the
	// shared IdentityClient interface also has an HTTP implementation without
	// that endpoint), so narrow it here.
	customerCounter, ok := identityClient.(*identitycontract.GRPCIdentityClient)
	if !ok {
		return nil, fmt.Errorf("identity client does not support customer counting")
	}

	inventoryTarget := utils.GetEnv("INVENTORY_GRPC_ADDR", "localhost:50054")
	inventoryClient, err := inventorycontract.NewGRPCInventoryClient(inventoryTarget)
	if err != nil {
		return nil, fmt.Errorf("inventory gRPC client (%s): %w", inventoryTarget, err)
	}

	brokerLogger := utils.NewLoggerWithPath("analytics", "analytics_kafka.log", "info")
	broker, err := kafka.NewKafkaBroker(utils.GetEnv("KAFKA_BROKERS", ""),
		brokerLogger,
	)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Kafka unavailable on boot, analytics projections will not update")
		broker = nil
	}

	repo := analyticsrepo.NewSqlAnalyticsRepository(analyticsqlc.New(db.DBPool))
	svc := analyticsservice.NewAnalyticsService(
		repo,
		customerCounter,
		catalogClient,
		inventoryClient,
	)

	analyticsroutes.RegisterRoutes(r, analyticshandler.NewAnalyticsHandler(svc), tokenService, cacheRedisService)

	// Order projection consumer: keeps the analytics read model current.
	if broker != nil {
		consumer := analyticsworker.NewOrderProjectionConsumer(db.DBPool, orderClient, logger.Log)
		dedup := withIdempotency(cacheRedisService, logger.Log)
		handler := dedup(analyticsQueue, func(headers messaging.Header, body []byte) error {
			// Kafka adapter sets event_type; old RabbitMQ path used x-routing-key.
			rk := headers.Get("event_type")
			if rk == "" {
				rk = headers.Get("x-routing-key")
			}
			switch rk {
			case events.RoutingKeyOrderCancelled:
				return consumer.HandleOrderCancelled(headers, body)
			case events.RoutingKeyOrderStatusChanged, events.RoutingKeyOrderShipped, events.RoutingKeyOrderDelivered:
				return consumer.HandleOrderStatusChanged(headers, body)
			case events.RoutingKeyPaymentSucceeded:
				return consumer.HandlePaymentSucceeded(headers, body)
			case events.RoutingKeyOrderReturnUpdated:
				return consumer.HandleOrderReturnUpdated(headers, body)
			case "payment.refunded":
				return consumer.HandlePaymentRefunded(headers, body)
			default:
				return consumer.HandleOrderCreated(headers, body)
			}
		})
		routingKeys := []string{
			events.RoutingKeyOrderCreated,
			events.RoutingKeyOrderCancelled,
			events.RoutingKeyOrderStatusChanged,
			events.RoutingKeyOrderShipped,
			events.RoutingKeyOrderDelivered,
			events.RoutingKeyPaymentSucceeded,
			events.RoutingKeyOrderReturnUpdated,
			"payment.refunded",
		}
		if err := broker.SubscribeTopic(context.Background(), analyticsQueue, routingKeys, handler); err != nil {
			logger.Log.Error().Err(err).Msg("Failed to subscribe analytics order events topic")
		} else {
			logger.Log.Info().Msgf("Analytics subscribed topic queue: %s", analyticsQueue)
		}
	}

	app := &App{
		cfg:    cfg,
		broker: broker,
		server: &http.Server{
			Addr:           ":" + utils.GetEnv("ANALYTICS_SERVER_PORT", "8092"),
			Handler:        r,
			ReadTimeout:    15 * time.Second,
			WriteTimeout:   15 * time.Second,
			MaxHeaderBytes: 1 << 20,
		},
	}
	app.closers = append(app.closers,
		orderClient.Close,
		catalogClient.Close,
		inventoryClient.Close,
	)
	return app, nil
}

func (a *App) Run(ctx context.Context) error {
	httpErrors := make(chan error, 1)
	go func() {
		logger.Log.Info().Str("addr", a.server.Addr).Msg("Analytics HTTP service listening and serving")
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			httpErrors <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-httpErrors:
		return fmt.Errorf("analytics HTTP server error: %w", err)
	case sig := <-quit:
		logger.Log.Info().Str("signal", sig.String()).Msg("Analytics service shutting down")
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.server.Shutdown(shutdownCtx); err != nil {
		logger.Log.Warn().Err(err).Msg("Analytics HTTP server forced shutdown")
	}
	if a.broker != nil {
		_ = a.broker.Close()
	}
	for _, closeFn := range a.closers {
		_ = closeFn()
	}
	return nil
}
