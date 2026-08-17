package main

import (
	"context"
	"encoding/json"
	"goshop/internal/config"
	"goshop/internal/utils"
	"goshop/pkg/logger"
	"goshop/pkg/mail"
	"goshop/pkg/rabbitmq"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
)

type Worker struct {
	rabbitMQ    rabbitmq.RabbitMQService
	mailService mail.EmailProviderService
	cfg         *config.Config
	logger      *zerolog.Logger
}

func NewWorker(cfg *config.Config) *Worker {
	log := utils.NewLoggerWithPath("worker.log", "info")

	// Connect to RabbitMQ
	rabbitMQ, err := rabbitmq.NewRabbitMQService(
		utils.GetEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/"), log,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create RabbitMQ service")
	}

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

	return &Worker{
		rabbitMQ:    rabbitMQ,
		mailService: mailService,
		cfg:         cfg,
		logger:      log,
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

	if err := w.rabbitMQ.Consume(ctx, emailQueueName, handler); err != nil {
		w.logger.Error().Err(err).Msg("Failed to consume email queue")
		return err
	}

	w.logger.Info().Msgf("Worker started and consuming email queue: %s", emailQueueName)

	<-ctx.Done()

	w.logger.Info().Msg("Worker stopped consuming due to context cancellation")

	return ctx.Err()
}

func (w *Worker) Shutdown(ctx context.Context) error {
	w.logger.Info().Msg("Shutting down worker...")

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
	wg.Add(1)

	go func() {
		defer wg.Done()
		if err := worker.Start(ctx); err != nil && err != context.Canceled {
			logger.Log.Error().Err(err).Msg("Failed to start worker")
		}
	}()

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
