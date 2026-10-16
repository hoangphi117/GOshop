package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"goshop/internal/config"
	"goshop/internal/db"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/discord"
	"goshop/pkg/logger"
	"goshop/pkg/mail"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/utils"
	"goshop/pkg/validation"
	"goshop/pkg/ws"
	identitycontract "goshop/services/identity/contract"
	"goshop/services/notification/internal/consumer"
	notificationsqlc "goshop/services/notification/internal/db/sqlc"
	"goshop/services/notification/internal/handler"
	"goshop/services/notification/internal/repository"
	notifroutes "goshop/services/notification/internal/routes"
	"goshop/services/notification/internal/service"

	"github.com/gin-gonic/gin"
)

// App encapsulates the Notification microservice runtime
type App struct {
	cfg                 *config.Config
	server              *http.Server
	rabbitMQ            rabbitmq.RabbitMQService
	notificationService service.NotificationService
}

// New constructs and wires the Notification application
func New(cfg *config.Config) (*App, error) {
	validation.InitValidator()

	r := gin.Default()

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

	if err := db.InitDBWithConfig(cfg); err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to initialize database for notification service")
		return nil, err
	}

	redisClient := config.NewRedisClient()
	cacheRedisService := cache.NewRedisCacheService(redisClient)
	tokenService := auth.NewJWTService(cacheRedisService)

	// RabbitMQ
	rabbitmqLogger := utils.NewLoggerWithPath("notification_rabbitmq.log", "info")
	rabbitmqURL := utils.GetEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
	rabbitmqService, err := rabbitmq.NewRabbitMQService(rabbitmqURL, rabbitmqLogger)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("RabbitMQ not available on boot, will retry in background")
	}

	// Redis Bridge for live WebSocket broadcast
	wsHub := ws.NewHub()
	redisBridge := ws.NewRedisBridge(redisClient, wsHub)

	// Mail & Discord
	mailLogger := utils.NewLoggerWithPath("notification_mail.log", "info")
	factory, err := mail.NewProviderFactory(mail.ProviderMailtrap)
	var mailService mail.EmailProviderService
	if err == nil {
		var mailErr error
		if mailService, mailErr = mail.NewMailService(cfg, mailLogger, factory); mailErr != nil {
			logger.Log.Warn().Err(mailErr).Msg("Failed to initialize mail service, email delivery disabled")
		}
	} else {
		logger.Log.Warn().Err(err).Msg("Failed to initialize mail factory, email delivery disabled")
	}
	
	discordLogger := utils.NewLoggerWithPath("notification_discord.log", "info")
	discordWebhookURL := utils.GetEnv("DISCORD_WEBHOOK_URL", "")
	discordNotifier := discord.NewDiscordNotifier(discordWebhookURL, discordLogger)

	// Identity Client (gRPC for user lookup)
	identityGRPCAddr := utils.GetEnv("IDENTITY_GRPC_ADDR", "localhost:50051")
	identityClient, err := identitycontract.NewGRPCIdentityClient(identityGRPCAddr, redisClient)
	if err != nil {
		logger.Log.Warn().Err(err).Str("addr", identityGRPCAddr).Msg("Could not connect to Identity gRPC service on boot")
	}

	// Notification Repository & Service
	dbPool := db.DBPool
	notificationQuerier := notificationsqlc.New(dbPool)
	notificationRepo := repository.NewSqlNotificationRepository(notificationQuerier)
	notificationService := service.NewNotificationService(
		notificationRepo,
		identityClient,
		mailService,
		discordNotifier,
		redisBridge,
	)

	// Subscribe to RabbitMQ Topic Exchange (Event-Driven)
	if rabbitmqService != nil {
		topicConsumer := consumer.NewTopicConsumer(
			notificationService,
			mailService,
			discordNotifier,
			rabbitmqLogger,
			dbPool,
		)
		topicPatterns := []string{
			rabbitmq.PatternOrderAll,
			rabbitmq.PatternPaymentAll,
			rabbitmq.PatternUserAll,
			rabbitmq.PatternInventoryAll,
			rabbitmq.QueueNotificationEvents,
		}
		queueName := "notification_service_events"
		go func() {
			if err := rabbitmqService.SubscribeTopic(context.Background(), queueName, topicPatterns, topicConsumer.HandleEvent); err != nil {
				logger.Log.Error().Err(err).Msg("Failed to subscribe to RabbitMQ topic events")
			} else {
				logger.Log.Info().Str("queue", queueName).Msg("Notification Service successfully subscribed to topic events")
			}
		}()
	}

	// Handlers & Routes
	notifHandler := handler.NewNotificationHandler(notificationService)
	userRoutes := []notifroutes.Route{notifroutes.NewNotificationRoutes(notifHandler)}
	adminRoutes := []notifroutes.Route{notifroutes.NewAdminNotificationRoutes(notifHandler)}

	notifroutes.RegisterRoutes(r, tokenService, cacheRedisService, userRoutes, adminRoutes)

	server := &http.Server{
		Addr:           cfg.ServerAddress,
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	return &App{
		cfg:                 cfg,
		server:              server,
		rabbitMQ:            rabbitmqService,
		notificationService: notificationService,
	}, nil
}

// Run starts the Notification HTTP server and listens for OS shutdown signals
func (a *App) Run(ctx context.Context) error {
	serverErrors := make(chan error, 1)

	go func() {
		logger.Log.Info().Str("addr", a.server.Addr).Msg("Notification service listening and serving HTTP")
		serverErrors <- a.server.ListenAndServe()
	}()

	// Periodic notification retention purge job (runs every 24h)
	retentionDays := utils.GetIntEnv("NOTIFICATION_RETENTION_DAYS", 30)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		logger.Log.Info().Int("retention_days", retentionDays).Msg("Notification retention job started (interval: 24h)")

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if a.notificationService != nil {
					if count, err := a.notificationService.PurgeStaleNotifications(context.Background(), retentionDays); err != nil {
						logger.Log.Error().Err(err).Msg("Failed to execute retention purge job")
					} else {
						logger.Log.Info().Int64("purged_count", count).Msg("Retention purge completed successfully")
					}
				}
			}
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("notification server error: %w", err)
		}
	case sig := <-shutdown:
		logger.Log.Info().Str("signal", sig.String()).Msg("Starting graceful shutdown of Notification Service")

		shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		if a.rabbitMQ != nil {
			if err := a.rabbitMQ.Close(); err != nil {
				logger.Log.Warn().Err(err).Msg("Failed to close RabbitMQ connection cleanly")
			}
		}

		if err := a.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("failed to shutdown notification server gracefully: %w", err)
		}

		logger.Log.Info().Msg("Notification Service stopped gracefully")
	}

	return nil
}
