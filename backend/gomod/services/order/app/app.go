package app

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc"
	"net"

	"goshop/contracts/events"
	cartcontract "goshop/services/cart/contract"
	catalogcontract "goshop/services/catalog/contract"
	couponcontract "goshop/services/coupon/contract"
	identitycontract "goshop/services/identity/contract"
	inventorycontract "goshop/services/inventory/contract"
	"goshop/services/order/contract"
	"goshop/services/order/internal/db/sqlc"
	v1dto "goshop/services/order/internal/dto"
	ordergrpc "goshop/services/order/internal/grpc"
	orderhandler "goshop/services/order/internal/handler"
	"goshop/services/order/internal/migrations"
	orderrepo "goshop/services/order/internal/repository"
	orderroutes "goshop/services/order/internal/routes"
	orderservice "goshop/services/order/internal/service"
	orderworker "goshop/services/order/internal/worker"
	paymentcontract "goshop/services/payment/contract"
	"goshop/services/payment/money"
	"goshop/shared/config"
	"goshop/shared/pkg/auth"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/db"
	"goshop/shared/pkg/kafka"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/messaging"
	"goshop/shared/pkg/middleware"
	"goshop/shared/pkg/shipping"
	"goshop/shared/pkg/telemetry"
	"goshop/shared/pkg/utils"
	"goshop/shared/pkg/validation"
	orderv1 "goshop/shared/proto/order/v1"
)

type App struct {
	cfg              *config.Config
	server           *http.Server
	grpcServer       *grpc.Server
	grpcListener     net.Listener
	orderHandler     *orderhandler.OrderHandler
	orderService     orderservice.OrderService
	sagaOrchestrator orderservice.OrderSagaOrchestrator
	shippingService  orderservice.OrderShippingService
	locationService  orderservice.LocationService
	returnService    orderservice.OrderReturnService
	adminService     orderservice.OrderAdminService
	broker           messaging.Broker
	outboxWorker     *orderworker.OutboxPublisherWorker
	sagaWorker       *orderworker.SagaRecoveryWorker
	paymentClient    *paymentcontract.GRPCPaymentClient
}

type paymentRefundAdapter struct {
	client *paymentcontract.GRPCPaymentClient
}

// RefundPayment forwards the refund in the payment service's native unit.
// The order side keeps whole dong in float64 (its own ledger); convert once
// here (dong → xu), and convert the returned minor units back once, so neither
// side does arithmetic on the float.
func (a paymentRefundAdapter) RefundPayment(ctx context.Context, idempotencyKey string, input v1dto.RefundPaymentInput) (v1dto.PaymentTransactionDTO, error) {
	amount, err := money.FromWholeDong(int64(math.Round(input.Amount)))
	if err != nil {
		return v1dto.PaymentTransactionDTO{}, fmt.Errorf("refund amount %v cannot be sent exactly: %w", input.Amount, err)
	}

	txn, err := a.client.RefundPayment(ctx, paymentcontract.RefundPaymentInput{
		IdempotencyKey: idempotencyKey,
		OrderUUID:      input.OrderUUID,
		Amount:         amount.MinorUnits(),
		Reason:         input.Reason,
	})
	if err != nil {
		return v1dto.PaymentTransactionDTO{}, err
	}

	txnAmount := money.FromMinorUnits(txn.Amount).ToFloat()
	var refundAmount *float64
	if txn.RefundAmount != nil {
		refunded := money.FromMinorUnits(*txn.RefundAmount).ToFloat()
		refundAmount = &refunded
	}

	return v1dto.PaymentTransactionDTO{
		UUID:          txn.UUID,
		OrderUUID:     txn.OrderUUID,
		Provider:      txn.Provider,
		ProviderTxnID: txn.ProviderTxnID,
		Amount:        txnAmount,
		Currency:      txn.Currency,
		Status:        txn.Status,
		RefundOf:      txn.RefundOf,
		RefundAmount:  refundAmount,
		PaidAt:        txn.PaidAt,
	}, nil
}

type orderProviderAdapter struct {
	querier *sqlc.Queries
}

func (a *orderProviderAdapter) GetOrderSummary(ctx context.Context, orderUUID uuid.UUID) (*contract.OrderSummary, error) {
	order, err := a.querier.GetOrderByUUID(ctx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	var deliveredAt *time.Time
	if order.OrderDeliveredAt.Valid {
		t := order.OrderDeliveredAt.Time
		deliveredAt = &t
	}

	return &contract.OrderSummary{
		OrderID:       order.OrderID,
		OrderUUID:     order.OrderUuid,
		OrderNumber:   order.OrderNumber,
		UserID:        order.OrderUserID,
		UserUUID:      order.UserUuid,
		CustomerName:  order.OrderCustomerName,
		CustomerEmail: order.OrderCustomerEmail,
		Status:        order.OrderStatus,
		TotalAmount:   utils.NumericToFloat(order.OrderTotalAmount),
		DeliveredAt:   deliveredAt,
		UpdatedAt:     order.OrderUpdatedAt,
	}, nil
}

func (a *orderProviderAdapter) VerifyPurchase(ctx context.Context, orderUUID uuid.UUID, userID, productID int32, userUUID, productUUID *uuid.UUID) (*contract.PurchaseVerification, error) {
	order, err := a.querier.GetOrderByUUID(ctx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if order.OrderUserID != userID {
		return nil, fmt.Errorf("user is not authorized for this order")
	}

	const maxReviewWindow = 30 * 24 * time.Hour
	canReview := false
	var deliveredAt *time.Time

	if order.OrderDeliveredAt.Valid {
		t := order.OrderDeliveredAt.Time
		deliveredAt = &t
		if time.Since(t) <= maxReviewWindow {
			canReview = true
		}
	} else if order.OrderStatus == 4 && !order.OrderUpdatedAt.IsZero() && time.Since(order.OrderUpdatedAt) <= maxReviewWindow {
		canReview = true
	}

	isPurchased := false
	items, err := a.querier.GetOrderItemsByOrderID(ctx, order.OrderID)
	if err == nil {
		for _, it := range items {
			if it.OrderItemProductID == productID {
				isPurchased = true
				break
			}
		}
	}

	return &contract.PurchaseVerification{
		OrderID:       order.OrderID,
		OrderUUID:     order.OrderUuid,
		UserID:        order.OrderUserID,
		IsPurchased:   isPurchased,
		CanReview:     canReview,
		DeliveredAt:   deliveredAt,
		CustomerName:  order.OrderCustomerName,
		CustomerEmail: order.OrderCustomerEmail,
		ShippingName:  order.OrderShippingName,
	}, nil
}

func (a *orderProviderAdapter) GetOrderForPayment(ctx context.Context, orderUUID uuid.UUID) (*contract.OrderPaymentInfo, error) {
	order, err := a.querier.GetOrderByUUID(ctx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &contract.OrderPaymentInfo{
		OrderUUID:   order.OrderUuid,
		UserID:      order.OrderUserID,
		UserUUID:    order.UserUuid,
		TotalAmount: utils.NumericToFloat(order.OrderTotalAmount),
		Status:      order.OrderStatus,
	}, nil
}

func New(cfg *config.Config) (*App, error) {
	if err := validation.InitValidator(); err != nil {
		return nil, fmt.Errorf("failed to initialize validator: %w", err)
	}

	r := gin.Default()
	r.Use(middleware.TraceMiddleware(), middleware.StructuredAccessLog(logger.Log))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "order",
		})
	})
	r.GET("/healthz/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive", "service": "order"})
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
	txManager := db.NewTxManager(dbPool)

	// Repositories
	orderRepo := orderrepo.NewSqlOrderRepository(orderQuerier, dbPool)
	orderItemRepo := orderrepo.NewSqlOrderItemRepository(orderQuerier)
	trackingRepo := orderrepo.NewSqlTrackingRepository(orderQuerier)
	returnRepo := orderrepo.NewSqlOrderReturnsRepository(orderQuerier)
	webhookRepo := orderrepo.NewSqlWebhookEventRepository(orderQuerier)
	sagaRepo := orderrepo.NewSqlSagaRepository(orderQuerier)
	outboxRepo := orderrepo.NewSqlOutboxRepository(orderQuerier)

	redisClient := config.NewRedisClient()
	cacheService := cache.NewRedisCacheService(redisClient)

	// Cross-domain gRPC clients
	catalogTarget := utils.GetEnv("CATALOG_GRPC_ADDR", "localhost:50055")
	catalogClient, err := catalogcontract.NewGRPCCatalogClient(catalogTarget)
	if err != nil {
		logger.Log.Warn().Err(err).Str("addr", catalogTarget).Msg("Failed to connect to Catalog gRPC service")
	}

	inventoryTarget := utils.GetEnv("INVENTORY_GRPC_ADDR", "localhost:50054")
	inventoryClient, err := inventorycontract.NewGRPCInventoryClient(inventoryTarget)
	if err != nil {
		logger.Log.Warn().Err(err).Str("addr", inventoryTarget).Msg("Failed to connect to Inventory gRPC service")
	}

	couponTarget := utils.GetEnv("COUPON_GRPC_ADDR", "localhost:50053")
	couponClient, err := couponcontract.NewGRPCCouponClient(couponTarget)
	if err != nil {
		logger.Log.Warn().Err(err).Str("addr", couponTarget).Msg("Failed to connect to Coupon gRPC service")
	}

	cartTarget := utils.GetEnv("CART_GRPC_ADDR", "localhost:50056")
	cartClient, err := cartcontract.NewGRPCCartClient(cartTarget)
	if err != nil {
		logger.Log.Warn().Err(err).Str("addr", cartTarget).Msg("Failed to connect to Cart gRPC service")
	}

	identityTarget := utils.GetEnv("IDENTITY_GRPC_ADDR", "localhost:50051")
	identityClient, err := identitycontract.NewGRPCIdentityClient(identityTarget, redisClient)
	if err != nil {
		logger.Log.Warn().Err(err).Str("addr", identityTarget).Msg("Failed to connect to Identity gRPC service")
	}

	// Adapters
	catalogPort := orderservice.NewOrderCatalogAdapter(catalogClient)
	stockPort := orderservice.NewOrderStockGRPCAdapter(inventoryClient)
	couponPort := orderservice.NewOrderCouponGRPCAdapter(couponClient)
	cartPort := orderservice.NewOrderCartAdapter(cartClient)
	customerReader := orderservice.NewOrderCustomerAdapter(identityClient)

	// Shipping Provider
	ghnToken := utils.GetEnv("GHN_TOKEN", "")
	ghnShopID, _ := strconv.Atoi(utils.GetEnv("GHN_SHOP_ID", "0"))
	ghnBaseURL := utils.GetEnv("GHN_BASE_URL", "")
	ghnAdapter := shipping.NewGHNAdapter(ghnToken, ghnShopID, ghnBaseURL)
	shippingFactory := shipping.NewShippingFactory(ghnAdapter)

	// Services
	pricingService := orderservice.NewOrderPricingService(couponPort)
	sagaOrchestrator := orderservice.NewOrderSagaOrchestrator(
		sagaRepo,
		txManager,
		pricingService,
		catalogPort,
		stockPort,
		orderRepo,
		orderItemRepo,
		customerReader,
		cartPort,
		couponPort,
		trackingRepo,
		cacheService,
		outboxRepo,
	)
	orderService := orderservice.NewOrderService(
		txManager,
		pricingService,
		catalogPort,
		stockPort,
		orderRepo,
		orderItemRepo,
		customerReader,
		cartPort,
		couponPort,
		trackingRepo,
		redisClient,
		outboxRepo,
	)

	// Kafka & Workers
	brokerLogger := utils.NewLoggerWithPath("order", "order_kafka.log", "info")
	broker, err := kafka.NewKafkaBroker(utils.GetEnv("KAFKA_BROKERS", ""),
		brokerLogger,
	)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Kafka not available on boot for order service, will retry in background")
	}
	r.GET("/healthz/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := dbPool.Ping(ctx); err != nil {
			logger.Log.Warn().Err(err).Str("trace_id", c.GetString("trace_id")).Msg("Order readiness database check failed")
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "service": "order"})
			return
		}
		if broker == nil || !broker.IsHealthy() {
			logger.Log.Warn().Str("trace_id", c.GetString("trace_id")).Msg("Order readiness Kafka check failed")
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "service": "order"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready", "service": "order"})
	})

	shippingService := orderservice.NewOrderShippingService(orderservice.OrderShippingServiceDeps{
		TxManager:       txManager,
		OrderRepo:       orderRepo,
		OrderItemRepo:   orderItemRepo,
		TrackingRepo:    trackingRepo,
		ShippingFactory: shippingFactory,
		WebhookRepo:     webhookRepo,
		UserRepo:        customerReader,
		RedisClient:     redisClient,
		CacheService:    cacheService,
		Broker:          broker,
	})
	locationService := orderservice.NewLocationService(shippingFactory, cacheService)
	var paymentClient *paymentcontract.GRPCPaymentClient
	var paymentRefundService orderservice.OrderPaymentRefundPort
	paymentTarget := utils.GetEnv("PAYMENT_GRPC_ADDR", "")
	if paymentTarget == "" {
		paymentTarget = utils.GetEnv("PAYMENT_GRPC_TARGET", "")
	}
	if paymentTarget == "" {
		paymentTarget = cfg.PaymentGRPCAddr
	}
	if paymentTarget != "" {
		paymentClient, err = paymentcontract.NewGRPCPaymentClient(paymentTarget)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize payment gRPC client: %w", err)
		}
		paymentRefundService = paymentRefundAdapter{client: paymentClient}
	}
	returnService := orderservice.NewOrderReturnService(orderservice.OrderReturnServiceDeps{
		TxManager:       txManager,
		OrderRepo:       orderRepo,
		OrderItemRepo:   orderItemRepo,
		OrderReturnRepo: returnRepo,
		StockPort:       stockPort,
		UserRepo:        customerReader,
		TrackingRepo:    trackingRepo,
		OutboxStore:     outboxRepo,
		PaymentService:  paymentRefundService,
		RedisClient:     redisClient,
		CacheService:    cacheService,
	})
	adminService := orderservice.NewOrderAdminService(orderService)

	orderHandler := orderhandler.NewOrderHandler(
		orderService,
		shippingService,
		returnService,
	).WithAdminService(adminService).
		WithLocationService(locationService).
		WithInvoiceService(orderService)

	tokenService := auth.NewJWTService(cacheService)
	orderroutes.RegisterRoutes(r, orderHandler, tokenService, cacheService)

	var outboxWorker *orderworker.OutboxPublisherWorker
	if broker != nil {
		outboxWorker = orderworker.NewOutboxPublisherWorkerWithAggregates(
			outboxRepo,
			broker,
			1*time.Second,
			50,
			logger.Log,
			[]string{"order"},
		)

		paymentSucceededConsumer := orderworker.NewPaymentSucceededConsumer(orderService, logger.Log)
		paymentExpiredConsumer := orderworker.NewPaymentExpiredConsumer(orderService, logger.Log)
		shippingConsumer := orderworker.NewShippingConsumer(shippingService, logger.Log)

		go func() {
			if err := broker.Consume(context.Background(), events.QueuePaymentSucceeded, paymentSucceededConsumer.Handle); err != nil {
				logger.Log.Error().Err(err).Msg("Failed to start PaymentSucceeded consumer")
			}
		}()
		go func() {
			if err := broker.Consume(context.Background(), events.QueuePaymentExpired, paymentExpiredConsumer.Handle); err != nil {
				logger.Log.Error().Err(err).Msg("Failed to start PaymentExpired consumer")
			}
		}()
		go func() {
			if err := broker.Consume(context.Background(), orderservice.QueueShippingCreate, shippingConsumer.Handle); err != nil {
				logger.Log.Error().Err(err).Msg("Failed to start Shipping consumer")
			}
		}()
	}

	sagaWorker := orderworker.NewSagaRecoveryWorker(
		sagaRepo,
		sagaOrchestrator,
		30*time.Second,
		50,
		logger.Log,
	)

	provider := &orderProviderAdapter{
		querier: orderQuerier,
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

	grpcServer := grpc.NewServer(telemetry.ServerOptions()...)
	orderv1.RegisterOrderServiceServer(grpcServer, ordergrpc.NewOrderGRPCServer(provider))

	return &App{
		cfg:              cfg,
		server:           server,
		grpcServer:       grpcServer,
		grpcListener:     grpcLis,
		orderHandler:     orderHandler,
		orderService:     orderService,
		sagaOrchestrator: sagaOrchestrator,
		shippingService:  shippingService,
		locationService:  locationService,
		returnService:    returnService,
		adminService:     adminService,
		broker:           broker,
		outboxWorker:     outboxWorker,
		sagaWorker:       sagaWorker,
		paymentClient:    paymentClient,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	errChan := make(chan error, 2)

	// 1. Start HTTP Server
	go func() {
		logger.Log.Info().Msgf("Order HTTP Server running on %s", a.server.Addr)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- fmt.Errorf("order http server error: %w", err)
		}
	}()

	// 2. Start gRPC Server
	go func() {
		logger.Log.Info().Msgf("Order gRPC Server running on %s", a.grpcListener.Addr().String())
		if err := a.grpcServer.Serve(a.grpcListener); err != nil {
			errChan <- fmt.Errorf("order grpc server error: %w", err)
		}
	}()

	// 3. Start Background Workers
	if a.sagaWorker != nil {
		go a.sagaWorker.Start(ctx)
	}
	if a.outboxWorker != nil {
		go a.outboxWorker.Start(ctx)
	}

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

	if a.broker != nil {
		_ = a.broker.Close()
	}

	logger.Log.Info().Msg("Order service stopped cleanly")
	return nil
}
