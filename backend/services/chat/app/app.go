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
	pkgpgx "goshop/pkg/pgx"
	"goshop/pkg/utils"
	"goshop/pkg/validation"
	ws "goshop/pkg/ws"
	chatv1 "goshop/proto/chat/v1"
	chatsqlc "goshop/services/chat/internal/db/sqlc"
	chatgrpc "goshop/services/chat/internal/grpc"
	chathandler "goshop/services/chat/internal/handler"
	"goshop/services/chat/internal/migrations"
	chatrepo "goshop/services/chat/internal/repository"
	chatroutes "goshop/services/chat/internal/routes"
	chatservice "goshop/services/chat/internal/service"
	identitycontract "goshop/services/identity/contract"
	ordercontract "goshop/services/order/contract"

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
	identityClient identitycontract.IdentityClient
	orderClient    ordercontract.OrderClient
	closers        []func() error
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
			logger.Log.Warn().Err(err).Msg("Failed to configure trusted proxies for chat service")
		}
	} else if err := r.SetTrustedProxies([]string{"127.0.0.1", "::1"}); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to configure dev trusted proxies")
	}

	if err := db.InitDBWithConfig(cfg, "chat"); err != nil {
		return nil, fmt.Errorf("database init failed: %w", err)
	}

	// Run embedded migrations (chat owns shopping_chat).
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s&x-migrations-table=schema_migrations_chat",
		cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.DBName, cfg.DB.SSLMode)
	if sourceDriver, err := iofs.New(migrations.FS, "."); err != nil {
		logger.Log.Warn().Err(err).Msg("Could not load embedded migrations source for chat service")
	} else if m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, connStr); err != nil {
		logger.Log.Warn().Err(err).Msg("Could not initialize migrate instance for chat service")
	} else {
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			logger.Log.Error().Err(err).Msg("Failed to execute database migrations for chat service")
		} else {
			logger.Log.Info().Msg("Chat service database migrations applied successfully")
		}
		_, _ = m.Close()
	}

	redisClient := config.NewRedisClient()
	cacheRedisService := cache.NewRedisCacheService(redisClient)
	tokenService := auth.NewJWTService(cacheRedisService)

	// Cross-domain gRPC ports. Chat owns no other service's data; identity and
	// order reads go through these clients.
	identityClient, err := identitycontract.NewGRPCIdentityClient(cfg.IdentityGRPCAddr, redisClient)
	if err != nil {
		return nil, fmt.Errorf("identity gRPC client (%s): %w", cfg.IdentityGRPCAddr, err)
	}

	orderTarget := utils.GetEnv("ORDER_GRPC_TARGET", "localhost:50057")
	orderClient, err := ordercontract.NewGRPCOrderClient(orderTarget)
	if err != nil {
		return nil, fmt.Errorf("order gRPC client (%s): %w", orderTarget, err)
	}

	chatRepo := chatrepo.NewSqlChatRepository(chatsqlc.New(db.DBPool))
	userReader := chatservice.NewIdentityUserReader(identityClient)
	orderReader := chatservice.NewOrderServiceClient(orderClient)
	presenceReader := chatservice.NewRedisPresenceReader(ws.NewPresenceStore(redisClient))
	publisher := chatservice.NewRedisWSPublisher(redisClient, *logger.Log)
	txManager := pkgpgx.NewTxManager(db.DBPool)

	chatService := chatservice.NewChatService(chatRepo, userReader, orderReader, presenceReader, publisher, txManager)
	chatroutes.RegisterRoutes(r, chathandler.NewChatHandler(chatService), tokenService, cacheRedisService)

	httpServer := &http.Server{
		Addr:           ":" + utils.GetEnv("CHAT_SERVER_PORT", "8093"),
		Handler:        r,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	grpcAddr := fmt.Sprintf(":%s", utils.GetEnv("CHAT_GRPC_PORT", "50058"))
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logger.Log.Fatal().Err(err).Str("addr", grpcAddr).Msg("Failed to bind gRPC listener for chat service")
		return nil, fmt.Errorf("failed to listen on grpc addr %s: %w", grpcAddr, err)
	}

	grpcServer := grpc.NewServer()
	chatv1.RegisterChatServiceServer(grpcServer, chatgrpc.NewChatGRPCServer(chatRepo))

	app := &App{
		cfg:            cfg,
		server:         httpServer,
		grpcServer:     grpcServer,
		grpcListener:   grpcListener,
		identityClient: identityClient,
		orderClient:    orderClient,
	}
	app.closers = append(app.closers, orderClient.Close)
	return app, nil
}

func (a *App) Run(ctx context.Context) error {
	httpErrors := make(chan error, 1)
	grpcErrors := make(chan error, 1)

	go func() {
		logger.Log.Info().Str("addr", a.server.Addr).Msg("Chat HTTP service listening and serving")
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			httpErrors <- err
		}
	}()

	go func() {
		logger.Log.Info().Str("addr", a.grpcListener.Addr().String()).Msg("Chat gRPC service listening and serving")
		if err := a.grpcServer.Serve(a.grpcListener); err != nil {
			grpcErrors <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-httpErrors:
		return fmt.Errorf("chat HTTP server error: %w", err)
	case err := <-grpcErrors:
		return fmt.Errorf("chat gRPC server error: %w", err)
	case sig := <-quit:
		logger.Log.Info().Str("signal", sig.String()).Msg("Chat service shutting down")
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Log.Info().Msg("Gracefully stopping Chat gRPC server")
	a.grpcServer.GracefulStop()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		logger.Log.Warn().Err(err).Msg("Chat HTTP server forced shutdown")
	}

	for _, closeFn := range a.closers {
		_ = closeFn()
	}
	return nil
}
