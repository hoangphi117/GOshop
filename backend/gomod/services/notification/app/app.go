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

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"goshop/contracts/events"
	identitycontract "goshop/services/identity/contract"
	"goshop/services/notification/internal/consumer"
	notificationsqlc "goshop/services/notification/internal/db/sqlc"
	"goshop/services/notification/internal/handler"
	"goshop/services/notification/internal/migrations"
	"goshop/services/notification/internal/repository"
	notifroutes "goshop/services/notification/internal/routes"
	"goshop/services/notification/internal/service"
	"goshop/shared/config"
	"goshop/shared/pkg/auth"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/db"
	"goshop/shared/pkg/discord"
	"goshop/shared/pkg/kafka"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/mail"
	"goshop/shared/pkg/messaging"
	"goshop/shared/pkg/metrics"
	"goshop/shared/pkg/utils"
	"goshop/shared/pkg/validation"
	"goshop/shared/pkg/ws"
)

// App encapsulates the Notification microservice runtime
type App struct {
	cfg                 *config.Config
	server              *http.Server
	broker              messaging.Broker
	notificationService service.NotificationService
}

// New constructs and wires the Notification application
func New(cfg *config.Config) (*App, error) {
	if err := validation.InitValidator(); err != nil {
		return nil, fmt.Errorf("failed to initialize validator: %w", err)
	}

	r := gin.Default()
	r.GET("/metrics", metrics.Handler(nil))

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

	if err := db.InitDBWithConfig(cfg, "notification"); err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to initialize database for notification service")
		return nil, err
	}

	// Run embedded migrations (processed_events idempotency ledger)
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s&x-migrations-table=schema_migrations_notification",
		cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.DBName, cfg.DB.SSLMode)
	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		logger.Log.Error().Err(err).Msg("Could not load embedded migrations source for notification service")
		return nil, fmt.Errorf("migration source load failed: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, connStr)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Could not initialize migrate instance for notification service")
		return nil, fmt.Errorf("migrate instance init failed: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		logger.Log.Error().Err(err).Msg("Failed to execute database migrations for notification service")
		return nil, fmt.Errorf("notification migrations failed: %w", err)
	}
	logger.Log.Info().Msg("Notification service database migrations applied successfully")
	if sourceErr, dbErr := m.Close(); sourceErr != nil || dbErr != nil {
		logger.Log.Warn().Interface("close_errors", map[string]any{"source": sourceErr, "db": dbErr}).Msg("Error closing migration instance")
	}

	redisClient := config.NewRedisClient()
	cacheRedisService := cache.NewRedisCacheService(redisClient)
	tokenService := auth.NewJWTService(cacheRedisService)

	// Kafka
	brokerLogger := utils.NewLoggerWithPath("notification", "notification_kafka.log", "info")
	broker, err := kafka.NewKafkaBroker(utils.GetEnv("KAFKA_BROKERS", ""),
		brokerLogger,
	)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Kafka not available on boot, will retry in background")
	}

	// Redis Bridge for live WebSocket broadcast
	wsHub := ws.NewHub()
	redisBridge := ws.NewRedisBridge(redisClient, wsHub)

	// Mail & Discord
	mailLogger := utils.NewLoggerWithPath("notification", "notification_mail.log", "info")
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

	discordLogger := utils.NewLoggerWithPath("notification", "notification_discord.log", "info")
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
	eventLedger := repository.NewSqlEventLedgerRepository(dbPool)
	notificationService := service.NewNotificationService(
		notificationRepo,
		identityClient,
		mailService,
		discordNotifier,
		redisBridge,
	)

	// Subscribe to Kafka Topic Exchange (Event-Driven)
	if broker != nil {
		topicConsumer := consumer.NewTopicConsumerWithLedger(
			notificationService,
			mailService,
			discordNotifier,
			brokerLogger,
			eventLedger,
			dbPool,
		)
		topicPatterns := []string{
			events.PatternOrderAll,
			events.PatternOrderReturns,
			events.PatternPaymentAll,
			events.PatternUserAll,
			events.PatternInventoryAll,
			events.QueueNotificationEvents,
		}
		queueName := "notification_service_events_v3"
		go func() {
			if err := broker.SubscribeTopic(context.Background(), queueName, topicPatterns, topicConsumer.HandleEvent); err != nil {
				logger.Log.Error().Err(err).Msg("Failed to subscribe to Kafka topic events")
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

	// Phase-1 observability: /healthz reports this process's consumer lag.
	kafka.RegisterHealthz(r, "notification", broker, dbPool.Ping)

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
		broker:              broker,
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

		if a.broker != nil {
			if err := a.broker.Close(); err != nil {
				logger.Log.Warn().Err(err).Msg("Failed to close Kafka connection cleanly")
			}
		}

		if err := a.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("failed to shutdown notification server gracefully: %w", err)
		}

		logger.Log.Info().Msg("Notification Service stopped gracefully")
	}

	return nil
}
