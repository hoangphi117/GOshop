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
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/db"
	"goshop/pkg/logger"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/utils"
	"goshop/pkg/validation"
	paymentv1 "goshop/proto/payment/v1"
	ordercontract "goshop/services/order/contract"
	paymentsqlc "goshop/services/payment/internal/db/sqlc"
	paymentgrpc "goshop/services/payment/internal/grpc"
	paymenthandler "goshop/services/payment/internal/handler"
	"goshop/services/payment/internal/migrations"
	"goshop/services/payment/internal/provider"
	paymentrepo "goshop/services/payment/internal/repository"
	paymentroutes "goshop/services/payment/internal/routes"
	paymentservice "goshop/services/payment/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"google.golang.org/grpc"
)

type App struct {
	cfg            *config.Config
	server         *http.Server
	grpcServer     *grpc.Server
	grpcListener   net.Listener
	rabbitMQ       rabbitmq.RabbitMQService
	paymentService paymentservice.PaymentService
	orderClient    ordercontract.OrderClient
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
			logger.Log.Warn().Err(err).Msg("Failed to configure trusted proxies for payment service")
		}
	} else {
		if err := r.SetTrustedProxies([]string{"127.0.0.1", "::1"}); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to configure dev trusted proxies")
		}
	}

	r.MaxMultipartMemory = 8 << 20

	if err := db.InitDBWithConfig(cfg, "payment"); err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to initialize database for payment service")
		return nil, fmt.Errorf("database init failed: %w", err)
	}

	// Run embedded migrations
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s&x-migrations-table=schema_migrations_payment",
		cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.DBName, cfg.DB.SSLMode)
	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Could not load embedded migrations source for payment service")
	} else {
		m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, connStr)
		if err != nil {
			logger.Log.Warn().Err(err).Msg("Could not initialize migrate instance for payment service")
		} else {
			if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
				logger.Log.Error().Err(err).Msg("Failed to execute database migrations for payment service")
			} else {
				logger.Log.Info().Msg("Payment service database migrations applied successfully")
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

	// RabbitMQ
	rabbitmqLogger := utils.NewLoggerWithPath("payment_rabbitmq.log", "info")
	rabbitmqURL := utils.GetEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
	rabbitmqService, err := rabbitmq.NewRabbitMQService(rabbitmqURL, rabbitmqLogger)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("RabbitMQ not available on boot for payment service, will retry in background")
	}

	// Repositories & Order Reader
	dbPool := db.DBPool
	paymentQuerier := paymentsqlc.New(dbPool)
	paymentRepo := paymentrepo.NewSqlPaymentRepository(paymentQuerier)
	webhookRepo := paymentrepo.NewSqlWebhookEventRepository(paymentQuerier)
	outboxRepo := paymentrepo.NewSqlOutboxRepository(paymentQuerier)
	orderTarget := utils.GetEnv("ORDER_GRPC_ADDR", utils.GetEnv("ORDER_GRPC_TARGET", "localhost:50057"))
	orderClient, err := ordercontract.NewOrderClient(orderTarget)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize order gRPC client: %w", err)
	}
	orderReader := paymentservice.NewOrderReaderGRPCClient(orderClient)

	// Providers
	paymentConfig := provider.PaymentConfig{
		MoMo: provider.MoMoConfig{
			PartnerCode: utils.GetEnv("MOMO_PARTNER_CODE", ""),
			AccessKey:   utils.GetEnv("MOMO_ACCESS_KEY", ""),
			SecretKey:   utils.GetEnv("MOMO_SECRET_KEY", ""),
			Endpoint:    utils.GetEnv("MOMO_ENDPOINT", ""),
			QueryURL:    utils.GetEnv("MOMO_QUERY_URL", ""),
		},
		VNPay: provider.VNPayConfig{
			TmnCode:    utils.GetEnv("VNPAY_TMN_CODE", ""),
			HashSecret: utils.GetEnv("VNPAY_HASH_SECRET", ""),
			Endpoint:   utils.GetEnv("VNPAY_ENDPOINT", "https://sandbox.vnpayment.vn/paymentv2/vpcpay.html"),
			QueryURL:   utils.GetEnv("VNPAY_QUERY_URL", "https://sandbox.vnpayment.vn/merchant_webapi/api/transaction"),
		},
	}
	providers := make(map[string]provider.PaymentProvider)
	if momoFactory, err := provider.NewProviderFactory(provider.ProviderMoMo, paymentConfig); err == nil {
		if p, err := momoFactory.CreateProvider(); err == nil {
			providers["momo"] = p
		} else {
			logger.Log.Warn().Err(err).Msg("Failed to instantiate MoMo provider")
		}
	} else {
		logger.Log.Warn().Err(err).Msg("Failed to create MoMo provider factory")
	}

	if vnpayFactory, err := provider.NewProviderFactory(provider.ProviderVNPay, paymentConfig); err == nil {
		if p, err := vnpayFactory.CreateProvider(); err == nil {
			providers["vnpay"] = p
		} else {
			logger.Log.Warn().Err(err).Msg("Failed to instantiate VNPay provider")
		}
	} else {
		logger.Log.Warn().Err(err).Msg("Failed to create VNPay provider factory")
	}

	returnURL := utils.GetEnv("PAYMENT_RETURN_URL", "http://localhost:3001/payment/result")
	webhookBaseURL := utils.GetEnv("PAYMENT_WEBHOOK_BASE_URL", "http://localhost:8083")

	paymentService := paymentservice.NewPaymentService(
		paymentRepo,
		orderReader,
		webhookRepo,
		outboxRepo,
		providers,
		cacheRedisService,
		rabbitmqService,
		returnURL,
		webhookBaseURL,
	)

	// Handlers & HTTP Routes
	paymentHandler := paymenthandler.NewPaymentHandler(paymentService)
	publicRoutes := []paymentroutes.Route{paymentroutes.NewPaymentWebhookRoutes(paymentHandler)}
	userRoutes := []paymentroutes.Route{paymentroutes.NewPaymentRoutes(paymentHandler)}
	adminRoutes := []paymentroutes.Route{paymentroutes.NewAdminPaymentRoutes(paymentHandler)}

	paymentroutes.RegisterRoutes(r, tokenService, cacheRedisService, publicRoutes, userRoutes, adminRoutes)

	httpServer := &http.Server{
		Addr:           cfg.ServerAddress,
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// gRPC Server setup
	grpcAddr := fmt.Sprintf(":%s", utils.GetEnv("PAYMENT_GRPC_PORT", "50052"))
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logger.Log.Fatal().Err(err).Str("addr", grpcAddr).Msg("Failed to bind gRPC listener for payment service")
		return nil, fmt.Errorf("failed to listen on grpc addr %s: %w", grpcAddr, err)
	}

	grpcServer := grpc.NewServer()
	paymentv1.RegisterPaymentServiceServer(grpcServer, paymentgrpc.NewPaymentGRPCServer(paymentService))

	return &App{
		cfg:            cfg,
		server:         httpServer,
		grpcServer:     grpcServer,
		grpcListener:   grpcListener,
		rabbitMQ:       rabbitmqService,
		paymentService: paymentService,
		orderClient:    orderClient,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	httpErrors := make(chan error, 1)
	grpcErrors := make(chan error, 1)

	// 1. Start HTTP Server
	go func() {
		logger.Log.Info().Str("addr", a.server.Addr).Msg("Payment HTTP service listening and serving")
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			httpErrors <- err
		}
	}()

	// 2. Start gRPC Server
	go func() {
		logger.Log.Info().Str("addr", a.grpcListener.Addr().String()).Msg("Payment gRPC service listening and serving")
		if err := a.grpcServer.Serve(a.grpcListener); err != nil {
			grpcErrors <- err
		}
	}()

	// 3. Periodic Payment Reconciliation Job (interval: 2m). Each run asks the
	// gateway for the status of stale pending transactions before expiring
	// them, so payments whose IPN never arrived are still confirmed.
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		logger.Log.Info().Msg("Payment reconciliation background job started (interval: 2m)")

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if a.paymentService != nil {
					if err := a.paymentService.ReconcilePendingPayments(context.Background()); err != nil {
						logger.Log.Error().Err(err).Msg("Error occurred during periodic payment reconciliation")
					} else {
						logger.Log.Info().Msg("Payment reconciliation executed successfully")
					}
				}
			}
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-httpErrors:
		return fmt.Errorf("payment http server error: %w", err)
	case err := <-grpcErrors:
		return fmt.Errorf("payment grpc server error: %w", err)
	case sig := <-shutdown:
		logger.Log.Info().Str("signal", sig.String()).Msg("Starting graceful shutdown of Payment Service")

		shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		if a.rabbitMQ != nil {
			if err := a.rabbitMQ.Close(); err != nil {
				logger.Log.Warn().Err(err).Msg("Failed to close RabbitMQ connection cleanly")
			}
		}
		if a.orderClient != nil {
			if err := a.orderClient.Close(); err != nil {
				logger.Log.Warn().Err(err).Msg("Failed to close Order gRPC client cleanly")
			}
		}

		a.grpcServer.GracefulStop()

		if err := a.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("failed to shutdown payment http server gracefully: %w", err)
		}

		logger.Log.Info().Msg("Payment Service stopped gracefully")
	}

	return nil
}
