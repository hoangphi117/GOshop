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
	"goshop/pkg/logger"
	"goshop/pkg/utils"
	"goshop/pkg/validation"
	inventoryv1 "goshop/proto/inventory/v1"
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
	"google.golang.org/grpc"
)

type App struct {
	cfg              *config.Config
	server           *http.Server
	grpcServer       *grpc.Server
	grpcListener     net.Listener
	inventoryService inventoryservice.InventoryService
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
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
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

	grpcServer := grpc.NewServer()
	inventoryv1.RegisterInventoryServiceServer(grpcServer, inventorygrpc.NewInventoryGRPCServer(svc))

	return &App{
		cfg:              cfg,
		server:           server,
		grpcServer:       grpcServer,
		grpcListener:     grpcLis,
		inventoryService: svc,
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

	a.grpcServer.GracefulStop()
	logger.Log.Info().Msg("Inventory service stopped cleanly")
	return nil
}
