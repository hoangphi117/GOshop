package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"goshop/internal/config"
	"goshop/pkg/db"
	"goshop/pkg/eventledger"
	"goshop/pkg/logger"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/telemetry"
	"goshop/pkg/utils"
	"goshop/pkg/validation"
	inventoryv1 "goshop/proto/inventory/v1"
	inventoryconsumer "goshop/services/inventory/internal/consumer"
	inventorysqlc "goshop/services/inventory/internal/db/sqlc"
	inventorygrpc "goshop/services/inventory/internal/grpc"
	inventoryhandler "goshop/services/inventory/internal/handler"
	"goshop/services/inventory/internal/migrations"
	inventoryrepo "goshop/services/inventory/internal/repository"
	inventoryroutes "goshop/services/inventory/internal/routes"
	inventoryservice "goshop/services/inventory/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/grpc"
)

type App struct {
	cfg                *config.Config
	server             *http.Server
	grpcServer         *grpc.Server
	grpcListener       net.Listener
	inventoryService   inventoryservice.InventoryService
	rabbitMQ           rabbitmq.RabbitMQService
	topicSubscriptions []topicSubscription
}

// topicSubscription binds one topic queue + routing key to an event handler.
type topicSubscription struct {
	queue      string
	routingKey string
	handler    func(amqp.Table, []byte) error
}

func New(cfg *config.Config) (*App, error) {
	if err := validation.InitValidator(); err != nil {
		return nil, fmt.Errorf("failed to initialize validator: %w", err)
	}

	r := gin.Default()

	if err := db.InitDBWithConfig(cfg, "inventory"); err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to initialize database for inventory service")
		return nil, fmt.Errorf("database init failed: %w", err)
	}

	// Run embedded migrations
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s&x-migrations-table=schema_migrations_inventory",
		cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.DBName, cfg.DB.SSLMode)
	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Could not load embedded migrations source for inventory service")
	} else {
		m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, connStr)
		if err != nil {
			logger.Log.Warn().Err(err).Msg("Could not initialize migrate instance for inventory service")
		} else {
			if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
				logger.Log.Error().Err(err).Msg("Failed to execute database migrations for inventory service")
			} else {
				logger.Log.Info().Msg("Inventory service database migrations applied successfully")
			}
			_, _ = m.Close()
		}
	}

	dbPool := db.DBPool
	inventoryQuerier := inventorysqlc.New(dbPool)
	inventoryRepo := inventoryrepo.NewSqlInventoryRepository(inventoryQuerier)
	svc := inventoryservice.NewInventoryService(inventoryRepo, dbPool)
	h := inventoryhandler.NewInventoryHandler(svc)

	inventoryroutes.RegisterRoutes(r, h)

	// RabbitMQ + event consumers (branch F: inventory owns stock release/restock)
	inventoryRabbitLogger := utils.NewLoggerWithPath("inventory_rabbitmq.log", "info")
	rabbitMQService, err := rabbitmq.NewRabbitMQService(
		utils.GetEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/"), inventoryRabbitLogger,
	)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to connect RabbitMQ for inventory service event consumers")
		return nil, fmt.Errorf("rabbitmq connect failed: %w", err)
	}
	if err := eventledger.Migrate(connStr, "event_ledger_migrations_inventory"); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to migrate event ledger for inventory service")
		return nil, err
	}
	ledgerRepo := eventledger.NewRepository(dbPool)
	ledgerLogger := eventledger.NewZerologLogger(inventoryRabbitLogger)

	port := utils.GetEnv("INVENTORY_SERVER_PORT", "8085")
	server := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	grpcPort := utils.GetEnv("INVENTORY_GRPC_PORT", "50054")
	grpcLis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		logger.Log.Fatal().Err(err).Msgf("Failed to listen on gRPC port %s", grpcPort)
		return nil, fmt.Errorf("failed to listen on gRPC port %s: %w", grpcPort, err)
	}

	grpcServer := grpc.NewServer(telemetry.ServerOptions()...)
	inventoryv1.RegisterInventoryServiceServer(grpcServer, inventorygrpc.NewInventoryGRPCServer(svc))

	return &App{
		cfg:              cfg,
		server:           server,
		grpcServer:       grpcServer,
		grpcListener:     grpcLis,
		inventoryService: svc,
		rabbitMQ:         rabbitMQService,
		topicSubscriptions: []topicSubscription{
			{
				queue:      rabbitmq.QueueInventoryOrderEvents,
				routingKey: rabbitmq.RoutingKeyOrderCancelled,
				handler: eventledger.Wrap(ledgerRepo, "inventory.order_cancelled",
					inventoryconsumer.NewOrderCancelledConsumer(svc).Apply, ledgerLogger),
			},
			{
				queue:      rabbitmq.QueueInventoryReturnEvents,
				routingKey: rabbitmq.RoutingKeyOrderReturnCompleted,
				handler: eventledger.Wrap(ledgerRepo, "inventory.return_completed",
					inventoryconsumer.NewReturnCompletedConsumer(svc).Apply, ledgerLogger),
			},
		},
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	errChan := make(chan error, 2)

	go func() {
		logger.Log.Info().Msgf("Inventory HTTP Server running on %s", a.server.Addr)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- fmt.Errorf("inventory http server error: %w", err)
		}
	}()

	go func() {
		logger.Log.Info().Msgf("Inventory gRPC Server running on %s", a.grpcListener.Addr().String())
		if err := a.grpcServer.Serve(a.grpcListener); err != nil {
			errChan <- fmt.Errorf("inventory grpc server error: %w", err)
		}
	}()

	// Topic consumers: stock release on order.cancelled, restock on
	// order_return.completed (branch F ownership move).
	go func() {
		for _, sub := range a.topicSubscriptions {
			if err := a.rabbitMQ.SubscribeTopic(ctx, sub.queue, []string{sub.routingKey}, sub.handler); err != nil {
				logger.Log.Error().Err(err).Str("queue", sub.queue).Str("routing_key", sub.routingKey).Msg("Inventory event consumer stopped")
			} else {
				logger.Log.Info().Str("queue", sub.queue).Str("routing_key", sub.routingKey).Msg("Inventory consuming topic events")
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return err
	case <-quit:
		logger.Log.Info().Msg("Shutting down Inventory service...")
	case <-ctx.Done():
		logger.Log.Info().Msg("Context cancelled, shutting down Inventory service...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error().Err(err).Msg("Inventory HTTP server forced to shutdown")
	}

	if err := a.rabbitMQ.Close(); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to close Inventory RabbitMQ connection cleanly")
	}

	a.grpcServer.GracefulStop()
	logger.Log.Info().Msg("Inventory service stopped cleanly")
	return nil
}
