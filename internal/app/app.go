package app

import (
	"context"
	"goshop/internal/config"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	"goshop/internal/routes"
	"goshop/internal/utils"
	"goshop/internal/validation"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/mail"
	"goshop/pkg/rabbitmq"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type Module interface {
	Routes() []routes.Route
}

type AdminModule interface {
	AdminRoutes() []routes.Route
}

type Application struct {
	config  *config.Config
	router  *gin.Engine
	modules []Module
}

type ModuleContext struct {
	DB    sqlc.Querier
	Redis *redis.Client
}

func NewApplication(cfg *config.Config) (*Application, error) {

	validation.InitValidator()

	r := gin.Default()

	if err := db.InitDB(); err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to initialize database")
		return nil, err
	}

	redisClient := config.NewRedisClient()
	cacheRedisService := cache.NewRedisCacheService(redisClient)
	tokenService := auth.NewJWTService(cacheRedisService)
	mailLogger := utils.NewLoggerWithPath("mail.log", "info")
	factory, err := mail.NewProviderFactory(mail.ProviderMailtrap)
	if err != nil {
		mailLogger.Error().Err(err).Msg("Failed to create mail provider factory")
		return nil, err
	}

	mailService, err := mail.NewMailService(cfg, mailLogger, factory)
	if err != nil {
		mailLogger.Error().Err(err).Msg("Failed to create mail service")
		return nil, err
	}

	rabbitmqLogger := utils.NewLoggerWithPath("worker.log", "info")
	rabbitmqService, err := rabbitmq.NewRabbitMQService(utils.GetEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/"), rabbitmqLogger)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to create RabbitMQ service")
		return nil, err
	}

	ctx := &ModuleContext{
		DB:    db.DB,
		Redis: redisClient,
	}

	modules := []Module{
		NewUserModule(ctx),
		NewProductModule(ctx),
		NewOrderModule(ctx, rabbitmqService),
		NewCartModule(ctx),
		NewCouponModule(ctx),
		NewPaymentModule(ctx, cacheRedisService, rabbitmqService),
		NewAddressModule(ctx),
		NewReviewModule(ctx),
		NewNotificationModule(ctx),
		NewAuthModule(ctx, tokenService, cacheRedisService, mailService, rabbitmqService, cfg.AppURL),
	}

	routes.RegisterRoutes(r, tokenService, cacheRedisService, getModuleRoutes(modules), getAdminModuleRoutes(modules))

	return &Application{
		config:  cfg,
		router:  r,
		modules: modules,
	}, nil
}

func (a *Application) Run() error {
	srv := &http.Server{
		Addr:    a.config.ServerAddress,
		Handler: a.router,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	// 					ctrl + c       | kill service  | reload service

	go func() {
		logger.Log.Info().Msgf("Server is running at: %s", a.config.ServerAddress)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			logger.Log.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	<-quit
	logger.Log.Warn().Msg("Shutdown signal received...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Log.Fatal().Err(err).Msg("⛔ Server forced to shutdown")
	}

	logger.Log.Info().Msg("☑️  Server exited gracefully")

	return nil
}

func getModuleRoutes(modules []Module) []routes.Route {
	routeList := make([]routes.Route, 0)
	for _, module := range modules {
		routeList = append(routeList, module.Routes()...)
	}
	return routeList
}

func getAdminModuleRoutes(modules []Module) []routes.Route {
	routeList := make([]routes.Route, 0)
	for _, module := range modules {
		if adminModule, ok := module.(AdminModule); ok {
			routeList = append(routeList, adminModule.AdminRoutes()...)
		}
	}
	return routeList
}
