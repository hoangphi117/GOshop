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
	"goshop/internal/db/sqlc"
	"goshop/internal/repository"
	v1service "goshop/internal/service/v1"
	"goshop/pkg/logger"
	"goshop/pkg/utils"
	"goshop/pkg/validation"
	orderv1 "goshop/proto/order/v1"
	"goshop/services/order/contract"
	ordergrpc "goshop/services/order/internal/grpc"
	"goshop/services/order/internal/migrations"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"google.golang.org/grpc"
)

type App struct {
	cfg          *config.Config
	server       *http.Server
	grpcServer   *grpc.Server
	grpcListener net.Listener
}

type orderProviderAdapter struct {
	reader v1service.OrderReader
}

func (a *orderProviderAdapter) GetOrderSummary(ctx context.Context, orderUUID uuid.UUID) (*contract.OrderSummary, error) {
	s, err := a.reader.GetOrderSummary(ctx, orderUUID)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, nil
	}
	return &contract.OrderSummary{
		OrderID:       s.OrderID,
		OrderUUID:     s.OrderUUID,
		OrderNumber:   s.OrderNumber,
		UserID:        s.UserID,
		UserUUID:      s.UserUUID,
		CustomerName:  s.CustomerName,
		CustomerEmail: s.CustomerEmail,
		Status:        s.Status,
		TotalAmount:   s.TotalAmount,
		DeliveredAt:   s.DeliveredAt,
		UpdatedAt:     s.UpdatedAt,
	}, nil
}

func (a *orderProviderAdapter) VerifyPurchase(ctx context.Context, orderUUID uuid.UUID, userID, productID int32, userUUID, productUUID *uuid.UUID) (*contract.PurchaseVerification, error) {
	v, err := a.reader.VerifyPurchase(ctx, orderUUID, userID, productID)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	return &contract.PurchaseVerification{
		OrderID:       v.OrderID,
		OrderUUID:     v.OrderUUID,
		UserID:        v.UserID,
		IsPurchased:   v.IsPurchased,
		CanReview:     v.CanReview,
		DeliveredAt:   v.DeliveredAt,
		CustomerName:  v.CustomerName,
		CustomerEmail: v.CustomerEmail,
		ShippingName:  v.ShippingName,
	}, nil
}

func (a *orderProviderAdapter) GetOrderForPayment(ctx context.Context, orderUUID uuid.UUID) (*contract.OrderPaymentInfo, error) {
	s, err := a.reader.GetOrderSummary(ctx, orderUUID)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, nil
	}
	return &contract.OrderPaymentInfo{
		OrderUUID:   s.OrderUUID,
		UserID:      s.UserID,
		UserUUID:    s.UserUUID,
		TotalAmount: s.TotalAmount,
		Status:      s.Status,
	}, nil
}

func New(cfg *config.Config) (*App, error) {
	validation.InitValidator()

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "order",
		})
	})

	if err := db.InitDBWithConfig(cfg, "order"); err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to initialize database for order service")
		return nil, fmt.Errorf("database init failed: %w", err)
	}

	// Run embedded migrations if needed
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s&x-migrations-table=schema_migrations_order",
		cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.DBName, cfg.DB.SSLMode)
	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Could not load embedded migrations source for order service")
	} else {
		m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, connStr)
		if err != nil {
			logger.Log.Warn().Err(err).Msg("Could not initialize migrate instance for order service")
		} else {
			if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
				logger.Log.Warn().Err(err).Msg("Order service database migrations status")
			} else {
				logger.Log.Info().Msg("Order service database migrations checked successfully")
			}
			_, _ = m.Close()
		}
	}

	dbPool := db.DBPool
	orderQuerier := sqlc.New(dbPool)
	orderRepo := repository.NewSqlOrderRepository(orderQuerier)
	orderItemRepo := repository.NewSqlOrderItemRepository(orderQuerier)
	inprocessReader := v1service.NewInprocessOrderReader(orderRepo, orderItemRepo)

	provider := &orderProviderAdapter{
		reader: inprocessReader,
	}

	port := utils.GetEnv("ORDER_SERVER_PORT", "8088")
	server := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	grpcPort := utils.GetEnv("ORDER_GRPC_PORT", "50057")
	grpcLis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		logger.Log.Fatal().Err(err).Msgf("Failed to listen on gRPC port %s", grpcPort)
		return nil, fmt.Errorf("failed to listen on gRPC port %s: %w", grpcPort, err)
	}

	grpcServer := grpc.NewServer()
	orderv1.RegisterOrderServiceServer(grpcServer, ordergrpc.NewOrderGRPCServer(provider))

	return &App{
		cfg:          cfg,
		server:       server,
		grpcServer:   grpcServer,
		grpcListener: grpcLis,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	errChan := make(chan error, 2)

	go func() {
		logger.Log.Info().Msgf("Order HTTP Server running on %s", a.server.Addr)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- fmt.Errorf("order http server error: %w", err)
		}
	}()

	go func() {
		logger.Log.Info().Msgf("Order gRPC Server running on %s", a.grpcListener.Addr().String())
		if err := a.grpcServer.Serve(a.grpcListener); err != nil {
			errChan <- fmt.Errorf("order grpc server error: %w", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return err
	case <-quit:
		logger.Log.Info().Msg("Shutting down Order service...")
	case <-ctx.Done():
		logger.Log.Info().Msg("Context cancelled, shutting down Order service...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error().Err(err).Msg("Order HTTP server forced to shutdown")
	}

	a.grpcServer.GracefulStop()
	logger.Log.Info().Msg("Order service stopped cleanly")
	return nil
}
