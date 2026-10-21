package main

import (
	"context"
	"encoding/json"
	"fmt"
	"goshop/internal/client"
	"goshop/internal/config"
	"goshop/internal/db"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/worker"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/mail"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/ws"
	catalogcontract "goshop/services/catalog/contract"
	couponcontract "goshop/services/coupon/contract"
	inventorycontract "goshop/services/inventory/contract"
	ordercontract "goshop/services/order/contract"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type Worker struct {
	rabbitMQ                        rabbitmq.RabbitMQService
	mailService                     mail.EmailProviderService
	cache                           cache.RedisCacheService
	redisClient                     *redis.Client
	cfg                             *config.Config
	logger                          *zerolog.Logger
	orderConsumer                   *worker.OrderConsumer
	inventoryOrderCancelledConsumer *worker.InventoryOrderCancelledConsumer
	inventoryReturnRestockConsumer  *worker.InventoryReturnRestockConsumer
	couponOrderCancelledConsumer    *worker.CouponOrderCancelledConsumer
	paymentService                  v1service.PaymentService
	userSyncConsumer                *worker.UserSyncConsumer
	orderProjectionConsumer         *worker.OrderProjectionConsumer
	redisBridge                     ws.RedisBridge
	grpcCouponClient                *couponcontract.GRPCCouponClient
	grpcCatalogClient               *catalogcontract.GRPCCatalogClient
	grpcInventoryClient             *inventorycontract.GRPCInventoryClient
}

func NewWorker(cfg *config.Config) *Worker {
	log := utils.NewLoggerWithPath("worker.log", "info")

	rabbitMQ, err := rabbitmq.NewRabbitMQService(
		utils.GetEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/"), log,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create RabbitMQ service")
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     utils.GetEnv("REDIS_ADDR", "localhost:6379"),
		Username: utils.GetEnv("REDIS_USERNAME", ""),
		Password: utils.GetEnv("REDIS_PASSWORD", ""),
		DB:       utils.GetIntEnv("REDIS_DB", 0),
	})
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Warn().Err(err).Msg("Redis unavailable, idempotency disabled")
	}
	cacheService := cache.NewRedisCacheService(redisClient)

	// Init MailService
	mailLogger := utils.NewLoggerWithPath("mail.log", "info")
	factory, err := mail.NewProviderFactory(mail.ProviderMailtrap)
	if err != nil {
		mailLogger.Error().Err(err).Msg("Failed to create mail provider factory")
		return nil
	}

	mailService, err := mail.NewMailService(cfg, mailLogger, factory)
	if err != nil {
		mailLogger.Error().Err(err).Msg("Failed to create mail service")
		return nil
	}

	// Add DB & NotificationConsumer initialization into worker
	if err := db.InitDB("worker"); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database for worker")
	}

	redisBridge := ws.NewRedisBridge(redisClient, nil)

	cartTarget := utils.GetEnv("CART_GRPC_TARGET", utils.GetEnv("CART_GRPC_ADDR", "localhost:50056"))
	grpcCartClient, err := client.NewGRPCCartClient(cartTarget)
	if err != nil {
		log.Warn().Err(err).Str("addr", cartTarget).Msg("Failed to initialize Cart gRPC client in worker")
	}
	cartService := client.NewCartServiceAdapter(grpcCartClient)
	notificationPublisher := worker.NewRabbitMQNotificationPublisher(rabbitMQ)
	orderConsumer := worker.NewOrderConsumerForBackground(cartService, notificationPublisher, log)

	// Payment Service via gRPC
	var paymentService v1service.PaymentService
	if cfg.PaymentGRPCAddr != "" {
		grpcPaymentClient, err := client.NewGRPCPaymentClient(cfg.PaymentGRPCAddr)
		if err != nil {
			log.Warn().Err(err).Str("addr", cfg.PaymentGRPCAddr).Msg("Failed to initialize Payment gRPC client in worker")
		} else {
			paymentService = client.NewPaymentServiceAdapter(grpcPaymentClient)
		}
	}

	couponTarget := utils.GetEnv("COUPON_GRPC_TARGET", "localhost:50053")
	grpcCouponClient, err := couponcontract.NewGRPCCouponClient(couponTarget)
	if err != nil {
		log.Fatal().Err(err).Str("target", couponTarget).Msg("Failed to initialize Coupon gRPC client")
	}
	couponPort := v1service.NewOrderCouponGRPCAdapter(grpcCouponClient)

	catalogTarget := utils.GetEnv("CATALOG_GRPC_TARGET", "localhost:50055")
	grpcCatalogClient, err := catalogcontract.NewGRPCCatalogClient(catalogTarget)
	if err != nil {
		log.Warn().Err(err).Str("target", catalogTarget).Msg("Failed to initialize Catalog gRPC client")
	}
	inventoryTarget := utils.GetEnv("INVENTORY_GRPC_TARGET", "localhost:50054")
	grpcInventoryClient, err := inventorycontract.NewGRPCInventoryClient(inventoryTarget)
	if err != nil {
		log.Fatal().Err(err).Str("target", inventoryTarget).Msg("Failed to initialize Inventory gRPC client")
	}
	stockPort := v1service.NewOrderStockGRPCAdapter(grpcInventoryClient)

	userSyncConsumer := worker.NewUserSyncConsumer(redisClient, db.DBPool)
	inventoryCancelledConsumer := worker.NewInventoryOrderCancelledConsumer(stockPort, log)
	inventoryReturnConsumer := worker.NewInventoryReturnRestockConsumer(stockPort, log)
	couponCancelledConsumer := worker.NewCouponOrderCancelledConsumer(couponPort, log)

	orderTarget := utils.GetEnv("ORDER_GRPC_TARGET", utils.GetEnv("ORDER_GRPC_ADDR", "localhost:50057"))
	var orderReader v1service.OrderReader
	grpcOrderClient, err := ordercontract.NewGRPCOrderClient(orderTarget)
	if err != nil {
		log.Warn().Err(err).Str("target", orderTarget).Msg("Failed to connect to Order gRPC service in worker")
	} else {
		orderReader = client.NewGRPCOrderReader(grpcOrderClient)
	}
	orderProjectionConsumer := worker.NewOrderProjectionConsumer(db.DBPool, orderReader, log)

	return &Worker{
		rabbitMQ:                        rabbitMQ,
		mailService:                     mailService,
		cache:                           cacheService,
		redisClient:                     redisClient,
		cfg:                             cfg,
		logger:                          log,
		orderConsumer:                   orderConsumer,
		inventoryOrderCancelledConsumer: inventoryCancelledConsumer,
		inventoryReturnRestockConsumer:  inventoryReturnConsumer,
		couponOrderCancelledConsumer:    couponCancelledConsumer,
		paymentService:                  paymentService,
		userSyncConsumer:                userSyncConsumer,
		orderProjectionConsumer:         orderProjectionConsumer,
		redisBridge:                     redisBridge,
		grpcCouponClient:                grpcCouponClient,
		grpcCatalogClient:               grpcCatalogClient,
		grpcInventoryClient:             grpcInventoryClient,
	}
}

func (w *Worker) Start(ctx context.Context) error {
	const emailQueueName = "auth_email_queue"

	handler := func(headers amqp.Table, body []byte) error {
		w.logger.Debug().Msgf("Received message: %s", string(body))

		ctx := context.Background()
		if traceID, ok := headers["trace_id"].(string); ok {
			ctx = logger.WithTraceID(ctx, traceID)
		}

		var email mail.Email
		if err := json.Unmarshal(body, &email); err != nil {
			w.logger.Error().Err(err).Msg("Failed to unmarshal email message")
			return err
		}

		if err := w.mailService.SendEmail(ctx, &email); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to send email", err)
		}

		w.logger.Info().Msgf("Email sent to: %s", email.To)

		return nil
	}

	deduped := w.withIdempotency(emailQueueName, handler)

	if err := w.rabbitMQ.Consume(ctx, emailQueueName, deduped); err != nil {
		w.logger.Error().Err(err).Msg("Failed to consume email queue")
		return err
	}

	w.logger.Info().Msgf("Worker started and consuming email queue: %s", emailQueueName)

	// Consume Order Created Events (Cart cleanup & Order notifications)
	if w.orderConsumer != nil {
		orderHandler := w.withIdempotency(rabbitmq.QueueOrderCreated, w.orderConsumer.Handle)
		if err := w.rabbitMQ.Consume(ctx, rabbitmq.QueueOrderCreated, orderHandler); err != nil {
			w.logger.Error().Err(err).Msg("Failed to consume order created events")
			return err
		}
		w.logger.Info().Msgf("Worker consuming queue: %s", rabbitmq.QueueOrderCreated)

		// Consume Order Cancelled Events
		cancelHandler := w.withIdempotency(rabbitmq.QueueOrderCancelled, w.orderConsumer.HandleOrderCancelled)
		if err := w.rabbitMQ.Consume(ctx, rabbitmq.QueueOrderCancelled, cancelHandler); err != nil {
			w.logger.Error().Err(err).Msg("Failed to consume order cancelled events")
		} else {
			w.logger.Info().Msgf("Worker consuming queue: %s", rabbitmq.QueueOrderCancelled)
		}

		// Consume Order Status Changed Events
		statusChangedHandler := w.withIdempotency(rabbitmq.QueueOrderStatusChanged, w.orderConsumer.HandleOrderStatusChanged)
		if err := w.rabbitMQ.Consume(ctx, rabbitmq.QueueOrderStatusChanged, statusChangedHandler); err != nil {
			w.logger.Error().Err(err).Msg("Failed to consume order status changed events")
		} else {
			w.logger.Info().Msgf("Worker consuming queue: %s", rabbitmq.QueueOrderStatusChanged)
		}

		// Consume Order Return Requested Events
		returnReqHandler := w.withIdempotency(rabbitmq.QueueOrderReturnRequested, w.orderConsumer.HandleOrderReturnRequested)
		if err := w.rabbitMQ.Consume(ctx, rabbitmq.QueueOrderReturnRequested, returnReqHandler); err != nil {
			w.logger.Error().Err(err).Msg("Failed to consume order return requested events")
		} else {
			w.logger.Info().Msgf("Worker consuming queue: %s", rabbitmq.QueueOrderReturnRequested)
		}

		// Consume Order Return Updated Events
		returnUpdatedHandler := w.withIdempotency(rabbitmq.QueueOrderReturnUpdated, w.orderConsumer.HandleOrderReturnUpdated)
		if err := w.rabbitMQ.Consume(ctx, rabbitmq.QueueOrderReturnUpdated, returnUpdatedHandler); err != nil {
			w.logger.Error().Err(err).Msg("Failed to consume order return updated events")
		} else {
			w.logger.Info().Msgf("Worker consuming queue: %s", rabbitmq.QueueOrderReturnUpdated)
		}
	}

	// Topic Pub/Sub: Inventory Order Events (Stock restoration on cancellation)
	if w.inventoryOrderCancelledConsumer != nil {
		invHandler := w.withIdempotency(rabbitmq.QueueInventoryOrderEvents, w.inventoryOrderCancelledConsumer.Handle)
		if err := w.rabbitMQ.SubscribeTopic(ctx, rabbitmq.QueueInventoryOrderEvents, []string{rabbitmq.RoutingKeyOrderCancelled}, invHandler); err != nil {
			w.logger.Error().Err(err).Msg("Failed to subscribe inventory order events topic")
		} else {
			w.logger.Info().Msgf("Worker subscribed topic queue: %s (routing: %s)", rabbitmq.QueueInventoryOrderEvents, rabbitmq.RoutingKeyOrderCancelled)
		}
	}

	// Topic Pub/Sub: Coupon Order Events (Coupon release on cancellation)
	if w.couponOrderCancelledConsumer != nil {
		couponHandler := w.withIdempotency(rabbitmq.QueueCouponOrderEvents, w.couponOrderCancelledConsumer.Handle)
		if err := w.rabbitMQ.SubscribeTopic(ctx, rabbitmq.QueueCouponOrderEvents, []string{rabbitmq.RoutingKeyOrderCancelled}, couponHandler); err != nil {
			w.logger.Error().Err(err).Msg("Failed to subscribe coupon order events topic")
		} else {
			w.logger.Info().Msgf("Worker subscribed topic queue: %s (routing: %s)", rabbitmq.QueueCouponOrderEvents, rabbitmq.RoutingKeyOrderCancelled)
		}
	}

	// Topic Pub/Sub: Inventory Return Restock Events (Restock on return completed)
	if w.inventoryReturnRestockConsumer != nil {
		returnRestockHandler := w.withIdempotency(rabbitmq.QueueInventoryReturnEvents, w.inventoryReturnRestockConsumer.Handle)
		if err := w.rabbitMQ.SubscribeTopic(ctx, rabbitmq.QueueInventoryReturnEvents, []string{rabbitmq.RoutingKeyOrderReturnCompleted}, returnRestockHandler); err != nil {
			w.logger.Error().Err(err).Msg("Failed to subscribe inventory return events topic")
		} else {
			w.logger.Info().Msgf("Worker subscribed topic queue: %s (routing: %s)", rabbitmq.QueueInventoryReturnEvents, rabbitmq.RoutingKeyOrderReturnCompleted)
		}
	}

	// Topic Pub/Sub: Analytics Order Events (Reporting Read Model Projection)
	if w.orderProjectionConsumer != nil {
		const queueAnalyticsOrders = "analytics.order_events"
		routingKeys := []string{
			rabbitmq.RoutingKeyOrderCreated,
			rabbitmq.RoutingKeyOrderCancelled,
			rabbitmq.RoutingKeyOrderStatusChanged,
			rabbitmq.RoutingKeyOrderShipped,
			rabbitmq.RoutingKeyOrderDelivered,
			rabbitmq.RoutingKeyPaymentSucceeded,
			rabbitmq.RoutingKeyOrderReturnRequested,
			rabbitmq.RoutingKeyOrderReturnUpdated,
		}
		analyticsHandler := w.withIdempotency(queueAnalyticsOrders, func(headers amqp.Table, body []byte) error {
			rk, _ := headers["x-routing-key"].(string)
			switch rk {
			case rabbitmq.RoutingKeyOrderCancelled:
				return w.orderProjectionConsumer.HandleOrderCancelled(headers, body)
			case rabbitmq.RoutingKeyOrderStatusChanged, rabbitmq.RoutingKeyOrderShipped, rabbitmq.RoutingKeyOrderDelivered:
				return w.orderProjectionConsumer.HandleOrderStatusChanged(headers, body)
			case rabbitmq.RoutingKeyPaymentSucceeded:
				return w.orderProjectionConsumer.HandlePaymentSucceeded(headers, body)
			case rabbitmq.RoutingKeyOrderReturnRequested:
				return w.orderProjectionConsumer.HandleOrderReturnRequested(headers, body)
			case rabbitmq.RoutingKeyOrderReturnUpdated:
				return w.orderProjectionConsumer.HandleOrderReturnUpdated(headers, body)
			default:
				return w.orderProjectionConsumer.HandleOrderCreated(headers, body)
			}
		})
		if err := w.rabbitMQ.SubscribeTopic(ctx, queueAnalyticsOrders, routingKeys, analyticsHandler); err != nil {
			w.logger.Error().Err(err).Msg("Failed to subscribe analytics order events topic")
		} else {
			w.logger.Info().Msgf("Worker subscribed topic queue: %s for analytics projection", queueAnalyticsOrders)
		}
	}

	// Consume User Sync Events (Cache invalidation & cross-service sync)
	if w.userSyncConsumer != nil {
		for _, queueName := range []string{rabbitmq.QueueUserCreated, rabbitmq.QueueUserUpdated, rabbitmq.QueueUserDeleted} {
			syncHandler := w.withIdempotency(queueName, w.userSyncConsumer.Handle)
			if err := w.rabbitMQ.Consume(ctx, queueName, syncHandler); err != nil {
				w.logger.Error().Err(err).Str("queue", queueName).Msg("Failed to consume user sync queue")
				return err
			}
			w.logger.Info().Msgf("Worker consuming queue: %s", queueName)
		}
	}

	<-ctx.Done()

	w.logger.Info().Msg("Worker stopped consuming due to context cancellation")

	return ctx.Err()
}

func (w *Worker) withIdempotency(queue string, next func(amqp.Table, []byte) error) func(amqp.Table, []byte) error {
	const dedupTTL = 24 * time.Hour

	return func(headers amqp.Table, body []byte) error {
		messageID, _ := headers["x-message-id"].(string)
		if messageID == "" {
			return next(headers, body)
		}

		key := fmt.Sprintf("dedup:%s:%s", queue, messageID)

		if exists, err := w.cache.Exists(key); err == nil && exists {
			w.logger.Info().Str("message_id", messageID).Msg("Duplicate message, skipping")
			return nil
		}

		if err := next(headers, body); err != nil {
			return err
		}

		if _, err := w.cache.SetNX(key, "1", dedupTTL); err != nil {
			w.logger.Error().Err(err).Str("message_id", messageID).Msg("Failed to mark message as processed")
		}

		return nil
	}
}

func (w *Worker) Shutdown(ctx context.Context) error {
	w.logger.Info().Msg("Shutting down worker...")

	if w.redisBridge != nil {
		w.redisBridge.Stop()
	}

	if w.grpcInventoryClient != nil {
		_ = w.grpcInventoryClient.Close()
	}
	if w.grpcCouponClient != nil {
		_ = w.grpcCouponClient.Close()
	}
	if w.grpcCatalogClient != nil {
		_ = w.grpcCatalogClient.Close()
	}

	if err := w.rabbitMQ.Close(); err != nil {
		w.logger.Error().Err(err).Msg("Failed to close RabbitMQ connection")
		return err
	}

	w.logger.Info().Msg("RabbitMQ shutdown completed")

	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			w.logger.Warn().Msg("Shutdown timed out")
		}
	default:
	}

	w.logger.Info().Msg("Worker shutdown completed")

	return nil
}

func main() {
	rootDir := utils.MustGetwd()

	logFile := filepath.Join(rootDir, "internal/logs/app.log")
	logger.InitLogger(logger.LoggerConfig{
		Level:      "info",
		Filename:   logFile,
		MaxSize:    1,
		MaxBackups: 5,
		MaxAge:     5,
		Compress:   true,
		IsDev:      utils.GetEnv("APP_ENV", "development"),
	})

	if err := godotenv.Load(filepath.Join(rootDir, ".env")); err != nil {
		logger.Log.Warn().Msg("⚠️ No .env found")
	} else {
		logger.Log.Info().Msg("Loaded .env successfully in worker!")
	}

	cfg := config.NewConfig()

	worker := NewWorker(cfg)
	if worker == nil {
		logger.Log.Fatal().Msg("Failed to initialize worker")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()

	var wg sync.WaitGroup

	wg.Go(func() {
		if err := worker.Start(ctx); err != nil && err != context.Canceled {
			logger.Log.Error().Err(err).Msg("Failed to start worker")
		}
	})

	<-ctx.Done()
	logger.Log.Info().Msg("Shutdown signal received...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := worker.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to shutdown worker")
	}

	wg.Wait()
	logger.Log.Info().Msg("Main process terminated")
}
