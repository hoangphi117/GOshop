package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"google.golang.org/grpc"
	"net"

	identitysqlc "goshop/services/identity/internal/db/sqlc"
	identitygrpc "goshop/services/identity/internal/grpc"
	identityhandler "goshop/services/identity/internal/handler"
	identitymigrations "goshop/services/identity/internal/migrations"
	identityrepo "goshop/services/identity/internal/repository"
	identityroutes "goshop/services/identity/internal/routes"
	identityservice "goshop/services/identity/internal/service"
	identityworker "goshop/services/identity/worker"
	"goshop/shared/config"
	"goshop/shared/pkg/auth"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/db"
	"goshop/shared/pkg/kafka"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/mail"
	"goshop/shared/pkg/metrics"
	"goshop/shared/pkg/telemetry"
	"goshop/shared/pkg/utils"
	"goshop/shared/pkg/validation"
	"goshop/shared/pkg/ws"
	identityv1 "goshop/shared/proto/identity/v1"
)

// App encapsulates the Identity microservice runtime
type App struct {
	cfg        *config.Config
	server     *http.Server
	grpcServer *grpc.Server
	grpcAddr   string
}

// runMigrations applies the embedded identity migrations. Without it a fresh
// volume starts with an empty shopping_identity schema and the seeder fails on
// missing tables (outbox_events etc).
func runMigrations(cfg *config.Config) error {
	// golang-migrate parses postgres:// URLs; a key=value DSN fails with
	// "failed to parse scheme from database URL". Use the same env overrides
	// IdentityDNS() honours, rendered as a URL.
	host := utils.GetEnv("IDENTITY_DB_HOST", utils.GetEnv("DB_IDENTITY_HOST", cfg.DB.Host))
	port := utils.GetEnv("IDENTITY_DB_PORT", utils.GetEnv("DB_IDENTITY_PORT", cfg.DB.Port))
	user := utils.GetEnv("IDENTITY_DB_USER", utils.GetEnv("DB_IDENTITY_USER", cfg.DB.User))
	password := utils.GetEnv("IDENTITY_DB_PASSWORD", utils.GetEnv("DB_IDENTITY_PASSWORD", cfg.DB.Password))
	dbName := utils.GetEnv("IDENTITY_DB_NAME", utils.GetEnv("DB_IDENTITY_NAME", ""))
	sslMode := utils.GetEnv("IDENTITY_DB_SSL_MODE", utils.GetEnv("DB_IDENTITY_SSL_MODE", cfg.DB.SSLMode))

	if dbName == "" {
		logger.Log.Warn().Msg("Identity DB name is empty, skipping migrations")
		return nil
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		url.QueryEscape(user), url.QueryEscape(password), host, port, dbName, sslMode)

	sourceDriver, err := iofs.New(identitymigrations.FS, ".")
	if err != nil {
		return fmt.Errorf("load identity migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, connStr)
	if err != nil {
		return fmt.Errorf("init identity migrate instance: %w", err)
	}

	upErr := m.Up()
	_, _ = m.Close()

	if upErr != nil && !errors.Is(upErr, migrate.ErrNoChange) {
		return fmt.Errorf("apply identity migrations: %w", upErr)
	}
	logger.Log.Info().Msg("Identity service database migrations applied successfully")
	return nil
}

// New constructs and wires the Identity application
func New(cfg *config.Config) (*App, error) {
	if err := validation.InitValidator(); err != nil {
		return nil, fmt.Errorf("failed to initialize validator: %w", err)
	}

	r := gin.Default()
	r.GET("/metrics", metrics.Handler(nil))

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

	// Run embedded migrations before any handler touches the schema. Identity
	// owns its own database (IDENTITY_DB_NAME), so the connection string is
	// built from that rather than the generic DB_NAME.
	if err := runMigrations(cfg); err != nil {
		logger.Log.Error().Err(err).Msg("Identity service database migrations failed")
		return nil, err
	}

	redisClient := config.NewRedisClient()
	cacheRedisService := cache.NewRedisCacheService(redisClient)
	tokenService := auth.NewJWTService(cacheRedisService)

	mailLogger := utils.NewLoggerWithPath("identity", "identity_mail.log", "info")
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

	brokerLogger := utils.NewLoggerWithPath("identity", "identity_kafka.log", "info")
	broker, err := kafka.NewKafkaBroker(utils.GetEnv("KAFKA_BROKERS", ""),
		brokerLogger,
	)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to create Kafka service")
		return nil, err
	}

	wsHub := ws.NewHub()
	redisBridge := ws.NewRedisBridge(redisClient, wsHub)
	go redisBridge.StartSubscriber(context.Background())

	// Storage Layer (Dedicated Identity DB)
	identityPool := db.GetIdentityDBPool()
	if identityPool == nil {
		identityPool = db.DBPool
	}

	identitySqlc := identitysqlc.New(identityPool)
	userRepo := identityrepo.NewSqlUserRepository(identitySqlc, identityPool)
	addressRepo := identityrepo.NewSqlAddressRepository(identitySqlc, identityPool)
	outboxRepo := identityrepo.NewSqlOutboxRepository(identitySqlc)

	// Business Services
	authService := identityservice.NewAuthService(userRepo, tokenService, cacheRedisService, mailService, broker, wsHub, redisBridge, cfg.StorefrontURL)
	userService := identityservice.NewUserServiceWithOutbox(userRepo, redisClient, broker, outboxRepo)
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
	if broker != nil {
		identityOutboxWorker := identityworker.NewOutboxPublisherWorkerWithAggregates(
			outboxRepo,
			broker,
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
	grpcServer := grpc.NewServer(telemetry.ServerOptions()...)
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
