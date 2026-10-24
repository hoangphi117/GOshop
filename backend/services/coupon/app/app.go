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
	"goshop/internal/db"
	"goshop/pkg/eventledger"
	"goshop/pkg/logger"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/utils"
	"goshop/pkg/validation"
	couponv1 "goshop/proto/coupon/v1"
	couponconsumer "goshop/services/coupon/internal/consumer"
	couponsqlc "goshop/services/coupon/internal/db/sqlc"
	coupongrpc "goshop/services/coupon/internal/grpc"
	couponhandler "goshop/services/coupon/internal/handler"
	"goshop/services/coupon/internal/migrations"
	couponrepo "goshop/services/coupon/internal/repository"
	couponroutes "goshop/services/coupon/internal/routes"
	couponservice "goshop/services/coupon/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/grpc"
)

type App struct {
	cfg            *config.Config
	server         *http.Server
	grpcServer     *grpc.Server
	grpcListener   net.Listener
	couponService  couponservice.CouponService
	rabbitMQ       rabbitmq.RabbitMQService
	eventConsumers []func(amqp.Table, []byte) error
}

func New(cfg *config.Config) (*App, error) {
	if err := validation.InitValidator(); err != nil {
		return nil, fmt.Errorf("failed to initialize validator: %w", err)
	}

	r := gin.Default()

	if err := db.InitDBWithConfig(cfg, "coupon"); err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to initialize database for coupon service")
		return nil, fmt.Errorf("database init failed: %w", err)
	}

	// Run embedded migrations
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s&x-migrations-table=schema_migrations_coupon",
		cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.DBName, cfg.DB.SSLMode)
	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Could not load embedded migrations source for coupon service")
	} else {
		m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, connStr)
		if err != nil {
			logger.Log.Warn().Err(err).Msg("Could not initialize migrate instance for coupon service")
		} else {
			if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
				logger.Log.Error().Err(err).Msg("Failed to execute database migrations for coupon service")
			} else {
				logger.Log.Info().Msg("Coupon service database migrations applied successfully")
			}
			_, _ = m.Close()
		}
	}

	dbPool := db.DBPool
	couponQuerier := couponsqlc.New(dbPool)
	couponRepo := couponrepo.NewSqlCouponRepository(couponQuerier)
	svc := couponservice.NewCouponService(couponRepo, dbPool)
	couponHandler := couponhandler.NewCouponHandler(svc)

	couponroutes.RegisterRoutes(r, couponHandler)

	// RabbitMQ + event consumer (branch F: coupon owns coupon release on order.cancelled)
	couponRabbitLogger := utils.NewLoggerWithPath("coupon_rabbitmq.log", "info")
	rabbitMQService, err := rabbitmq.NewRabbitMQService(
		utils.GetEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/"), couponRabbitLogger,
	)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to connect RabbitMQ for coupon service event consumer")
		return nil, fmt.Errorf("rabbitmq connect failed: %w", err)
	}
	if err := eventledger.Migrate(connStr, "event_ledger_migrations_coupon"); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to migrate event ledger for coupon service")
		return nil, err
	}

	port := utils.GetEnv("COUPON_SERVER_PORT", "8084")
	server := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	grpcPort := utils.GetEnv("COUPON_GRPC_PORT", "50053")
	grpcLis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		logger.Log.Fatal().Err(err).Msgf("Failed to listen on gRPC port %s", grpcPort)
		return nil, fmt.Errorf("failed to listen on gRPC port %s: %w", grpcPort, err)
	}

	grpcServer := grpc.NewServer()
	couponv1.RegisterCouponServiceServer(grpcServer, coupongrpc.NewCouponGRPCServer(svc))

	return &App{
		cfg:           cfg,
		server:        server,
		grpcServer:    grpcServer,
		grpcListener:  grpcLis,
		couponService: svc,
		rabbitMQ:      rabbitMQService,
		eventConsumers: []func(amqp.Table, []byte) error{
			eventledger.Wrap(
				eventledger.NewRepository(dbPool),
				"coupon.order_cancelled",
				couponconsumer.NewOrderCancelledConsumer(svc).Apply,
				eventledger.NewZerologLogger(couponRabbitLogger),
			),
		},
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	errChan := make(chan error, 2)

	go func() {
		logger.Log.Info().Msgf("Coupon HTTP Server running on %s", a.server.Addr)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- fmt.Errorf("coupon http server error: %w", err)
		}
	}()

	go func() {
		logger.Log.Info().Msgf("Coupon gRPC Server running on %s", a.grpcListener.Addr().String())
		if err := a.grpcServer.Serve(a.grpcListener); err != nil {
			errChan <- fmt.Errorf("coupon grpc server error: %w", err)
		}
	}()

	// Topic consumer: coupon release on order.cancelled (branch F ownership move)
	go func() {
		for _, consumerFn := range a.eventConsumers {
			if err := a.rabbitMQ.SubscribeTopic(ctx, rabbitmq.QueueCouponOrderEvents, []string{rabbitmq.RoutingKeyOrderCancelled}, consumerFn); err != nil {
				logger.Log.Error().Err(err).Str("queue", rabbitmq.QueueCouponOrderEvents).Msg("Coupon event consumer stopped")
			} else {
				logger.Log.Info().Str("queue", rabbitmq.QueueCouponOrderEvents).Str("routing_key", rabbitmq.RoutingKeyOrderCancelled).Msg("Coupon consuming order.cancelled events")
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return err
	case <-quit:
		logger.Log.Info().Msg("Shutting down Coupon service...")
	case <-ctx.Done():
		logger.Log.Info().Msg("Context cancelled, shutting down Coupon service...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error().Err(err).Msg("Coupon HTTP server forced to shutdown")
	}

	if err := a.rabbitMQ.Close(); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to close Coupon RabbitMQ connection cleanly")
	}

	a.grpcServer.GracefulStop()
	logger.Log.Info().Msg("Coupon service stopped cleanly")
	return nil
}
