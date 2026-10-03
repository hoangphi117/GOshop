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
	"goshop/pkg/ws"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	config      *config.Config
	router      *gin.Engine
	modules     []Module
	wsHub       ws.Hub
	redisBridge ws.RedisBridge
}

type ModuleContext struct {
	DB        sqlc.Querier
	Redis     *redis.Client
	TxManager db.TxManager
}

func NewApplication(cfg *config.Config) (*Application, error) {

	validation.InitValidator()

	r := gin.Default()

	// =========================================================================
	// CONFIGURE TRUSTED PROXIES FOR GIN ENGINE (REVERSE PROXY / LOAD BALANCER)
	// =========================================================================
	// Architectural & Security Rationale:
	// When the system is deployed behind a Reverse Proxy / Load Balancer (Nginx, Cloudflare, AWS ALB),
	// the TCP connection directly to Gin originates from the Proxy's internal IP address.
	//
	// If SetTrustedProxies is not configured:
	// 1. c.ClientIP() will read the wrong IP and return the Proxy's IP (e.g., 172.x.x.x or 127.0.0.1).
	// 2. This causes the Rate Limiting feature (Redis-based by IP) to incorrectly block all users.
	// 3. The Suspicious Login Detection feature (alerts for unusual locations/IPs) will record the wrong client IP.
	// 4. The X-Forwarded-For header can be spoofed (IP Spoofing) if arbitrary proxies are trusted.
	//
	// Configuration:
	// - Production: Trust only secure internal network ranges (Docker bridge / private subnets / localhost).
	// - Development: Trust the localhost loopback addresses (127.0.0.1, ::1).
	if cfg.AppEnv == "production" {
		_ = r.SetTrustedProxies([]string{
			"127.0.0.1",      // Local loopback
			"::1",            // IPv6 loopback
			"10.0.0.0/8",     // RFC 1918 Private Class A (K8s/Cloud internal VPC)
			"172.16.0.0/12",  // RFC 1918 Private Class B (Docker default bridge network)
			"192.168.0.0/16", // RFC 1918 Private Class C (LAN / Custom Docker network)
		})
	} else {
		_ = r.SetTrustedProxies([]string{
			"127.0.0.1",
			"::1",
		})
	}

	r.MaxMultipartMemory = 8 << 20 // 8MB memory buffer for multipart forms (prevents OOM DoS)

	// Protect the static /uploads directory against MIME sniffing and unauthorized script execution
	r.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/uploads") {
			c.Header("X-Content-Options", "nosniff")
			c.Header("Content-Security-Policy", "default-src 'none'; img-src 'self'; media-src 'self'")
		}
		c.Next()
	})

	r.Static("/uploads", "./uploads")

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

	wsHub := ws.NewHub()
	redisBridge := ws.NewRedisBridge(redisClient, wsHub)
	go redisBridge.StartSubscriber(context.Background())

	txManager := db.NewTxManager(db.DBPool)

	ctx := &ModuleContext{
		DB:        db.DB,
		Redis:     redisClient,
		TxManager: txManager,
	}

	modules := []Module{
		NewUserModule(ctx, tokenService),
		NewProductModule(ctx),
		NewOrderModule(ctx, wsHub, rabbitmqService),
		NewCartModule(ctx),
		NewCouponModule(ctx),
		NewPaymentModule(ctx, cacheRedisService, rabbitmqService, wsHub),
		NewAddressModule(ctx),
		NewReviewModule(ctx, wsHub),
		NewNotificationModule(ctx, wsHub, tokenService, cacheRedisService, rabbitmqService, redisBridge),
		NewAuthModule(ctx, tokenService, cacheRedisService, mailService, rabbitmqService, wsHub, redisBridge, cfg.StorefrontURL),
		NewUploadModule(ctx),
		NewBannerModule(ctx, cacheRedisService),
		NewAnalyticsModule(ctx),
		NewChatModule(ctx, wsHub, redisBridge),
	}

	routes.RegisterRoutes(r, tokenService, cacheRedisService, getModuleRoutes(modules), getAdminModuleRoutes(modules))

	return &Application{
		config:      cfg,
		router:      r,
		modules:     modules,
		wsHub:       wsHub,
		redisBridge: redisBridge,
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

	if a.redisBridge != nil {
		a.redisBridge.Stop()
	}

	if a.wsHub != nil {
		a.wsHub.Shutdown()
	}

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
