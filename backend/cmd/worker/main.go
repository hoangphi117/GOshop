package main

import (
	"context"
	"encoding/json"
	"fmt"
	"goshop/internal/config"
	"goshop/internal/db"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/mail"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/storage"
	"goshop/pkg/utils"
	"goshop/pkg/ws"
	catalogcontract "goshop/services/catalog/contract"
	identityworker "goshop/services/identity/worker"
	mediaworker "goshop/services/media/worker"
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
	rabbitMQ          rabbitmq.RabbitMQService
	mailService       mail.EmailProviderService
	cache             cache.RedisCacheService
	redisClient       *redis.Client
	cfg               *config.Config
	logger            *zerolog.Logger
	userSyncConsumer  *identityworker.UserSyncConsumer
	mediaProcessor    *mediaworker.MediaProcessor
	redisBridge       ws.RedisBridge
	grpcCatalogClient *catalogcontract.GRPCCatalogClient
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

	// Branch F: cart cleanup (Cart), coupon release (Coupon) and stock
	// release/restock (Inventory) consumers moved to their owning services.
	// The worker keeps cross-cutting jobs: auth emails, user sync, analytics
	// projections, Meilisearch sync and the outbox publisher.

	catalogTarget := utils.GetEnv("CATALOG_GRPC_TARGET", "localhost:50055")
	grpcCatalogClient, err := catalogcontract.NewGRPCCatalogClient(catalogTarget)
	if err != nil {
		log.Warn().Err(err).Str("target", catalogTarget).Msg("Failed to initialize Catalog gRPC client")
	}

	userSyncConsumer := identityworker.NewUserSyncConsumer(redisClient, db.DBPool)

	// Media post-processing: resize/re-encode images right after a direct upload.
	storageService, err := storage.NewStorageService(context.Background(), cfg.Storage, log)
	if err != nil {
		log.Error().Err(err).Msg("Storage unavailable, media post-processing disabled")
	}
	var mediaProcessor *mediaworker.MediaProcessor
	if storageService != nil {
		mediaProcessor = mediaworker.NewMediaProcessor(storageService, log)
	}

	return &Worker{
		rabbitMQ:          rabbitMQ,
		mailService:       mailService,
		cache:             cacheService,
		redisClient:       redisClient,
		cfg:               cfg,
		logger:            log,
		userSyncConsumer:  userSyncConsumer,
		mediaProcessor:    mediaProcessor,
		redisBridge:       redisBridge,
		grpcCatalogClient: grpcCatalogClient,
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

	// Analytics projection moved to services/analytics, which owns its own
	// topic subscription (analytics.order_events).

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

	// Consume Media Uploaded Events (image resize/re-encode)
	if w.mediaProcessor != nil {
		mediaHandler := w.withIdempotency(rabbitmq.QueueMediaUploaded, w.mediaProcessor.Handle)
		if err := w.rabbitMQ.Consume(ctx, rabbitmq.QueueMediaUploaded, mediaHandler); err != nil {
			w.logger.Error().Err(err).Str("queue", rabbitmq.QueueMediaUploaded).Msg("Failed to consume media queue")
		} else {
			w.logger.Info().Msgf("Worker consuming queue: %s", rabbitmq.QueueMediaUploaded)
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

	if w.grpcCatalogClient != nil {
		if err := w.grpcCatalogClient.Close(); err != nil {
			w.logger.Warn().Err(err).Msg("Failed to close Catalog gRPC client")
		}
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
		logger.Log.Warn().Msg("No .env found, continuing with environment variables")
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
