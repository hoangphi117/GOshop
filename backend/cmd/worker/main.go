package main

import (
	"context"
	"encoding/json"
	"fmt"
	"goshop/internal/client"
	"goshop/internal/config"
	"goshop/internal/db"
	"goshop/internal/repository"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/worker"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/mail"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/search"
	"goshop/pkg/shipping"
	"goshop/pkg/ws"
	couponcontract "goshop/services/coupon/contract"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type Worker struct {
	rabbitMQ             rabbitmq.RabbitMQService
	mailService          mail.EmailProviderService
	cache                cache.RedisCacheService
	redisClient          *redis.Client
	cfg                      *config.Config
	logger                   *zerolog.Logger
	orderConsumer            *worker.OrderConsumer
	shippingConsumer         *worker.ShippingConsumer
	paymentExpiredConsumer   *worker.PaymentExpiredConsumer
	paymentSucceededConsumer *worker.PaymentSucceededConsumer
	syncWorker             *worker.MeilisearchSyncWorker
	outboxPublisherWorker  *worker.OutboxPublisherWorker
	shippingService        v1service.OrderShippingService
	paymentService         v1service.PaymentService
	userSyncConsumer       *worker.UserSyncConsumer
	redisBridge            ws.RedisBridge
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

	userRepo := repository.NewSqlUserRepository(db.DB)
	cartRepo := repository.NewSqlCartRepository(db.DB)
	productRepo := repository.NewSqlProductRepository(db.DB)
	orderRepo := repository.NewSqlOrderRepository(db.DB)
	orderItemRepo := repository.NewSqlOrderItemRepository(db.DB)
	trackingRepo := repository.NewSqlTrackingRepository(db.DB)
	webhookRepo := repository.NewSqlWebhookEventRepository(db.DB)
	txManager := db.NewTxManager(db.DBPool)

	redisBridge := ws.NewRedisBridge(redisClient, nil)

	cartService := v1service.NewCartService(cartRepo, userRepo, productRepo, redisClient)
	orderConsumer := worker.NewOrderConsumer(cartService, nil, log)

	// Shipping Factory & Service
	ghnToken := utils.GetEnv("GHN_TOKEN", "")
	ghnShopID, _ := strconv.Atoi(utils.GetEnv("GHN_SHOP_ID", "0"))
	ghnBaseURL := utils.GetEnv("GHN_BASE_URL", "")
	ghnAdapter := shipping.NewGHNAdapter(ghnToken, ghnShopID, ghnBaseURL)
	shippingFactory := shipping.NewShippingFactory(ghnAdapter)

	customerReader := v1service.NewOrderCustomerAdapter(userRepo)

	shippingService := v1service.NewOrderShippingService(v1service.OrderShippingServiceDeps{
		TxManager:       txManager,
		OrderRepo:       orderRepo,
		OrderItemRepo:   orderItemRepo,
		TrackingRepo:    trackingRepo,
		ShippingFactory: shippingFactory,
		WebhookRepo:     webhookRepo,
		UserRepo:        customerReader,
		RedisClient:     redisClient,
		RabbitMQ:        rabbitMQ,
	})
	shippingConsumer := worker.NewShippingConsumer(shippingService, log)

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

	outboxRepo := repository.NewSqlOutboxRepository(db.DB)

	// Order Service & Payment Expired Consumer
	couponTarget := utils.GetEnv("COUPON_GRPC_TARGET", "localhost:50053")
	grpcCouponClient, err := couponcontract.NewGRPCCouponClient(couponTarget)
	if err != nil {
		log.Fatal().Err(err).Str("target", couponTarget).Msg("Failed to initialize Coupon gRPC client")
	}
	defer grpcCouponClient.Close()
	couponPort := v1service.NewOrderCouponGRPCAdapter(grpcCouponClient)

	stockMovementRepo := repository.NewSqlStockMovementRepository(db.DB)
	inventoryService := v1service.NewInventoryService(productRepo, stockMovementRepo, userRepo, txManager, rabbitMQ)

	catalogPort := v1service.NewOrderCatalogAdapter(productRepo)
	stockPort := v1service.NewOrderStockAdapter(inventoryService)
	pricingService := v1service.NewOrderPricingService(couponPort)

	orderService := v1service.NewOrderService(
		txManager,
		pricingService,
		catalogPort,
		stockPort,
		orderRepo,
		orderItemRepo,
		customerReader,
		cartService,
		couponPort,
		trackingRepo,
		redisClient,
		outboxRepo,
	)
	paymentExpiredConsumer := worker.NewPaymentExpiredConsumer(orderService, log)
	paymentSucceededConsumer := worker.NewPaymentSucceededConsumer(orderService, log)

	// Transactional Outbox Publisher Worker (Relay for order & payment events)
	outboxPublisherWorker := worker.NewOutboxPublisherWorker(outboxRepo, rabbitMQ, 1*time.Second, 50, log)

	// Meilisearch Sync Worker
	var syncWorker *worker.MeilisearchSyncWorker
	searchClient, err := search.NewClient(cfg.Meilisearch.Host, cfg.Meilisearch.MasterKey, cfg.Meilisearch.IndexProducts)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to initialize Meilisearch client in worker")
	} else if searchClient != nil {
		syncWorker = worker.NewMeilisearchSyncWorker(outboxRepo, searchClient, 2*time.Second, 50, log)
	}

	userSyncConsumer := worker.NewUserSyncConsumer(redisClient, db.DBPool)

	return &Worker{
		rabbitMQ:                 rabbitMQ,
		mailService:              mailService,
		cache:                    cacheService,
		redisClient:              redisClient,
		cfg:                      cfg,
		logger:                   log,
		orderConsumer:            orderConsumer,
		shippingConsumer:         shippingConsumer,
		paymentExpiredConsumer:   paymentExpiredConsumer,
		paymentSucceededConsumer: paymentSucceededConsumer,
		syncWorker:             syncWorker,
		outboxPublisherWorker:  outboxPublisherWorker,
		shippingService:        shippingService,
		paymentService:         paymentService,
		userSyncConsumer:       userSyncConsumer,
		redisBridge:            redisBridge,
	}
}

func (w *Worker) Start(ctx context.Context) error {
	const emailQueueName = "auth_email_queue"

	handler := func(headers amqp.Table, body []byte) error {
		w.logger.Debug().Msgf("Received message: %s", string(body))

		ctx := context.Background()
		if traceID, ok := headers["trace_id"].(string); ok {
			ctx = context.WithValue(ctx, "trace_id", traceID)
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

	// Consume Shipping Order Create Events
	if w.shippingConsumer != nil {
		shippingHandler := w.withIdempotency(v1service.QueueShippingCreate, w.shippingConsumer.Handle)
		if err := w.rabbitMQ.Consume(ctx, v1service.QueueShippingCreate, shippingHandler); err != nil {
			w.logger.Error().Err(err).Msg("Failed to consume shipping order create events")
		} else {
			w.logger.Info().Msgf("Worker consuming queue: %s", v1service.QueueShippingCreate)
		}
	}

	// Consume Payment Expired Events (Order cancellation & Inventory restoration)
	if w.paymentExpiredConsumer != nil {
		expiredHandler := w.withIdempotency(rabbitmq.QueuePaymentExpired, w.paymentExpiredConsumer.Handle)
		if err := w.rabbitMQ.Consume(ctx, rabbitmq.QueuePaymentExpired, expiredHandler); err != nil {
			w.logger.Error().Err(err).Msg("Failed to consume payment expired events")
		} else {
			w.logger.Info().Msgf("Worker consuming queue: %s", rabbitmq.QueuePaymentExpired)
		}
	}

	// Consume Payment Succeeded Events (Order payment status update)
	if w.paymentSucceededConsumer != nil {
		succeededHandler := w.withIdempotency(rabbitmq.QueuePaymentSucceeded, w.paymentSucceededConsumer.Handle)
		if err := w.rabbitMQ.Consume(ctx, rabbitmq.QueuePaymentSucceeded, succeededHandler); err != nil {
			w.logger.Error().Err(err).Msg("Failed to consume payment succeeded events")
		} else {
			w.logger.Info().Msgf("Worker consuming queue: %s", rabbitmq.QueuePaymentSucceeded)
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

	// Meilisearch Transactional Outbox Sync Worker
	if w.syncWorker != nil {
		go w.syncWorker.Start(ctx)
		w.logger.Info().Msg("Meilisearch Outbox Sync Worker started in worker process")
	}

	// Transactional Outbox Publisher Worker (Order & Payment Relay)
	if w.outboxPublisherWorker != nil {
		go w.outboxPublisherWorker.Start(ctx)
		w.logger.Info().Msg("Transactional Outbox Publisher Worker started in worker process")
	}

	// Periodic Shipping Tracking Sync (Cronjob with Redis Distributed Lock)
	if w.shippingService != nil {
		go func() {
			intervalMinutes, _ := strconv.Atoi(utils.GetEnv("SHIPPING_SYNC_INTERVAL_MINUTES", "30"))
			if intervalMinutes <= 0 {
				intervalMinutes = 30
			}
			ticker := time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
			defer ticker.Stop()
			w.logger.Info().Int("interval_minutes", intervalMinutes).Msg("Shipping tracking sync cron job started (with Redis lock)")

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					runWithDistributedLock(ctx, w.redisClient, "lock:cron:shipping_sync", time.Duration(intervalMinutes-1)*time.Minute, func() {
						if _, err := w.shippingService.SyncActiveShippingOrders(ctx); err != nil {
							w.logger.Error().Err(err).Msg("Error during periodic shipping sync cron job")
						}
					})
				}
			}
		}()
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

func runWithDistributedLock(ctx context.Context, redisClient *redis.Client, lockKey string, ttl time.Duration, task func()) {
	if redisClient == nil {
		task()
		return
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	ok, err := redisClient.SetNX(ctx, lockKey, "1", ttl).Result()
	if err != nil || !ok {
		return // Lock held by another worker instance
	}
	task()
}
