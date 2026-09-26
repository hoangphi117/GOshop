package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	"goshop/pkg/ws"
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
	rabbitMQ             rabbitmq.RabbitMQService
	mailService          mail.EmailProviderService
	cache                cache.RedisCacheService
	cfg                  *config.Config
	logger               *zerolog.Logger
	notificationConsumer *worker.NotificationConsumer
	retentionJob         *worker.NotificationRetentionJob
	redisBridge          ws.RedisBridge
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
	if err := db.InitDB(); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database for worker")
	}

	notificationRepo := repository.NewSqlNotificationRepository(db.DB)
	userRepo := repository.NewSqlUserRepository(db.DB)
	notificationService := v1service.NewNotificationService(notificationRepo, userRepo, nil)
	redisBridge := ws.NewRedisBridge(redisClient, nil)
	if svc, ok := notificationService.(interface{ SetRedisBridge(ws.RedisBridge) }); ok {
		svc.SetRedisBridge(redisBridge)
	}
	notificationConsumer := worker.NewNotificationConsumer(notificationService, log)

	retentionJob := worker.NewNotificationRetentionJob(notificationService,
		log,
		24*time.Hour,
		utils.GetIntEnv("NOTIFICATION_RETENTION_DAYS", 30))

	return &Worker{
		rabbitMQ:             rabbitMQ,
		mailService:          mailService,
		cache:                cacheService,
		cfg:                  cfg,
		logger:               log,
		notificationConsumer: notificationConsumer,
		retentionJob:         retentionJob,
		redisBridge:          redisBridge,
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

	// Consume Unified Notification Events (Customer & Admin)
	notifHandler := w.withIdempotency(rabbitmq.QueueNotificationEvents, w.notificationConsumer.Handle)
	if err := w.rabbitMQ.Consume(ctx, rabbitmq.QueueNotificationEvents, notifHandler); err != nil {
		w.logger.Error().Err(err).Msg("Failed to consume notification events")
		return err
	}
	w.logger.Info().Msgf("Worker consuming queue: %s", rabbitmq.QueueNotificationEvents)

	if w.retentionJob != nil {
		go w.retentionJob.Start(ctx)
		w.logger.Info().Msg("Notification retention job started (interval: 24h, retention: 30d)")
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
