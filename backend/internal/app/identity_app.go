package app

import (
	"context"
	"goshop/internal/config"
	"goshop/internal/db"
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/middleware"
	"goshop/internal/repository"
	"goshop/internal/routes"
	"goshop/internal/utils"
	"goshop/internal/validation"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/mail"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/ws"

	"github.com/gin-gonic/gin"
)

// NewIdentityApplication creates an application instance hosting only Identity domain modules
// (AuthModule, UserModule, AddressModule) on a dedicated port (e.g. 8081).
func NewIdentityApplication(cfg *config.Config) (*Application, error) {
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
		_ = r.SetTrustedProxies([]string{
			"127.0.0.1",
			"::1",
		})
	}

	r.MaxMultipartMemory = 8 << 20

	if err := db.InitDBWithConfig(cfg); err != nil {
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

	wsHub := ws.NewHub()
	redisBridge := ws.NewRedisBridge(redisClient, wsHub)
	go redisBridge.StartSubscriber(context.Background())

	ctx := &ModuleContext{
		DB:         db.DB,
		ReadDB:     db.GetReadDB(),
		IdentityDB: db.GetIdentityDB(),
		Redis:      redisClient,
		RabbitMQ:   rabbitmqService,
	}

	modules := []Module{
		NewAuthModule(ctx, tokenService, cacheRedisService, mailService, rabbitmqService, wsHub, redisBridge, cfg.StorefrontURL),
		NewUserModule(ctx, tokenService),
		NewAddressModule(ctx),
	}

	routes.RegisterRoutes(r, tokenService, cacheRedisService, getModuleRoutes(modules), getAdminModuleRoutes(modules))

	// Internal Inter-Service REST Endpoints (Lightweight REST IPC)
	internalGroup := r.Group("/internal/v1")
	internalGroup.Use(middleware.InternalAuthMiddleware(cfg.InternalSecret))
	userRepo := repository.NewSqlUserRepository(ctx.IdentityDB)
	internalUserHandler := v1handler.NewInternalUserHandler(userRepo, ctx.Redis)
	internalGroup.POST("/users/batch", internalUserHandler.GetBatchUsers)
	internalGroup.GET("/users/:id", internalUserHandler.GetUserByID)
	internalGroup.GET("/users/uuid/:uuid", internalUserHandler.GetUserByUUID)
	internalGroup.GET("/users/admins", internalUserHandler.GetActiveAdmins)

	addressRepo := repository.NewSqlAddressRepository(ctx.IdentityDB, db.GetIdentityDBPool())
	internalAddressHandler := v1handler.NewInternalAddressHandler(addressRepo, ctx.Redis)
	internalGroup.GET("/addresses/:uuid", internalAddressHandler.GetAddressByUUID)
	internalGroup.GET("/users/:id/addresses", internalAddressHandler.GetAddressesByUserID)

	return &Application{
		config:      cfg,
		router:      r,
		modules:     modules,
		wsHub:       wsHub,
		redisBridge: redisBridge,
	}, nil
}
