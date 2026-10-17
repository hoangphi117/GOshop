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

	"net"
	"strings"

	"goshop/internal/config"
	"goshop/internal/db"
	"goshop/internal/repository"
	"goshop/pkg/utils"
	"goshop/pkg/validation"
	"goshop/internal/worker"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/mail"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/ws"
	identityv1 "goshop/proto/identity/v1"
	identitysqlc "goshop/services/identity/internal/db/sqlc"
	identitygrpc "goshop/services/identity/internal/grpc"
	identityhandler "goshop/services/identity/internal/handler"
	identityrepo "goshop/services/identity/internal/repository"
	identityroutes "goshop/services/identity/internal/routes"
	identityservice "goshop/services/identity/internal/service"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// App encapsulates the Identity microservice runtime
type App struct {
	cfg        *config.Config
	server     *http.Server
	grpcServer *grpc.Server
	grpcAddr   string
}

// New constructs and wires the Identity application
func New(cfg *config.Config) (*App, error) {
	validation.InitValidator()

	r := gin.Default()

	if cfg.AppEnv == "production" {
		_ = r.SetTrustedProxies([]string{
			"127.0.0.1",
			"::1",
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
		})
	} else {
		_ = r.SetTrustedProxies([]string{"127.0.0.1", "::1"})
	}

	r.MaxMultipartMemory = 8 << 20

	if err := db.InitDBWithConfig(cfg, "identity"); err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to initialize database for identity service")
		return nil, err
	}

	redisClient := config.NewRedisClient()
	cacheRedisService := cache.NewRedisCacheService(redisClient)
	tokenService := auth.NewJWTService(cacheRedisService)

	mailLogger := utils.NewLoggerWithPath("identity_mail.log", "info")
	factory, err := mail.NewProviderFactory(mail.ProviderMailtrap)
	if err != nil {
		mailLogger.Error().Err(err).Msg("Failed to create mail provider factory")
		return nil, err
	}

	mailService, err := mail.NewMailService(cfg, mailLogger, factory)
	if err != nil {
		mailLogger.Error().Err(err).Msg("Failed to create mail service")
		return nil, err
	}

	rabbitmqLogger := utils.NewLoggerWithPath("identity_rabbitmq.log", "info")
	rabbitmqService, err := rabbitmq.NewRabbitMQService(utils.GetEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/"), rabbitmqLogger)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to create RabbitMQ service")
		return nil, err
	}

	wsHub := ws.NewHub()
	redisBridge := ws.NewRedisBridge(redisClient, wsHub)
	go redisBridge.StartSubscriber(context.Background())

	// Storage Layer (Dedicated Identity DB)
	identityQuerier := db.GetIdentityDB()
	if identityQuerier == nil {
		identityQuerier = db.DB
	}
	identityPool := db.GetIdentityDBPool()
	if identityPool == nil {
		identityPool = db.DBPool
	}

	identitySqlc := identitysqlc.New(identityPool)
	userRepo := identityrepo.NewSqlUserRepository(identitySqlc, identityPool)
	addressRepo := identityrepo.NewSqlAddressRepository(identitySqlc, identityPool)
	outboxRepo := identityrepo.NewSqlOutboxRepository(identitySqlc)

	// Business Services
	authService := identityservice.NewAuthService(userRepo, tokenService, cacheRedisService, mailService, rabbitmqService, wsHub, redisBridge, cfg.StorefrontURL)
	userService := identityservice.NewUserServiceWithOutbox(userRepo, redisClient, rabbitmqService, outboxRepo)
	addressService := identityservice.NewAddressService(addressRepo, userRepo)

	// Handlers
	authHandler := identityhandler.NewAuthHandler(authService)
	userHandler := identityhandler.NewUserHandler(userService)
	addressHandler := identityhandler.NewAddressHandler(addressService)

	// Route Registration (Self-contained in Identity microservice)
	identityroutes.RegisterRoutes(r, identityroutes.IdentityRouteDeps{
		AuthHandler:    authHandler,
		UserHandler:    userHandler,
		AddressHandler: addressHandler,
		TokenService:   tokenService,
		CacheService:   cacheRedisService,
	})

	// Transactional Outbox Publisher for Identity Service (Guaranteed At-Least-Once Delivery)
	coreOutboxRepo := repository.NewSqlOutboxRepository(identityQuerier)
	if rabbitmqService != nil {
		identityOutboxWorker := worker.NewOutboxPublisherWorkerWithAggregates(
			coreOutboxRepo,
			rabbitmqService,
			1*time.Second,
			50,
			logger.Log,
			[]string{"user"},
		)
		go identityOutboxWorker.Start(context.Background())
		logger.Log.Info().Msg("Identity Transactional Outbox Publisher Worker started")
	}

	server := &http.Server{
		Addr:           cfg.ServerAddress,
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// gRPC Server Initialization
	grpcPort := utils.GetEnv("IDENTITY_GRPC_PORT", ":50051")
	if !strings.HasPrefix(grpcPort, ":") {
		grpcPort = ":" + grpcPort
	}
	grpcServer := grpc.NewServer()
	identityGRPCServer := identitygrpc.NewServer(userRepo, addressRepo)
	identityv1.RegisterIdentityServiceServer(grpcServer, identityGRPCServer)

	return &App{
		cfg:        cfg,
		server:     server,
		grpcServer: grpcServer,
		grpcAddr:   grpcPort,
	}, nil
}

// Run starts the Identity HTTP and gRPC servers and listens for OS shutdown signals
func (a *App) Run(ctx context.Context) error {
	serverErrors := make(chan error, 1)

	go func() {
		logger.Log.Info().Str("addr", a.server.Addr).Msg("Identity service listening and serving HTTP")
		serverErrors <- a.server.ListenAndServe()
	}()

	lis, err := net.Listen("tcp", a.grpcAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on gRPC address %s: %w", a.grpcAddr, err)
	}

	grpcErrors := make(chan error, 1)
	go func() {
		logger.Log.Info().Str("addr", a.grpcAddr).Msg("Identity service listening and serving gRPC")
		grpcErrors <- a.grpcServer.Serve(lis)
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("identity HTTP service error: %w", err)
		}
		return nil

	case err := <-grpcErrors:
		return fmt.Errorf("identity gRPC service error: %w", err)

	case sig := <-shutdown:
		logger.Log.Info().Str("signal", sig.String()).Msg("Shutdown signal received, shutting down Identity service gracefully")
		a.grpcServer.GracefulStop()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := a.server.Shutdown(shutdownCtx); err != nil {
			_ = a.server.Close()
			return fmt.Errorf("failed to shutdown gracefully: %w", err)
		}
	}

	return nil
}
