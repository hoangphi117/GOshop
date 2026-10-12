package app

import (
	"context"
	"goshop/internal/client"
	"goshop/internal/config"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	"goshop/internal/repository"
	"goshop/internal/routes"
	"goshop/internal/utils"
	"goshop/internal/validation"
	v1service "goshop/internal/service/v1"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/payment"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/discord"
	"goshop/pkg/search"
	"goshop/pkg/shipping"
	"fmt"
	"goshop/pkg/storage"
	"goshop/pkg/ws"
	"net/http"
	"os"
	"os/signal"
	"strconv"
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
	DB             sqlc.Querier
	ReadDB         sqlc.Querier
	IdentityDB     sqlc.Querier
	Redis          *redis.Client
	TxManager      db.TxManager
	Storage        storage.StorageService
	Search         *search.Client
	IdentityClient client.IdentityClient
	RabbitMQ       rabbitmq.RabbitMQService
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

	// Media Storage Decoupling:
	// In local driver mode, serve the static /uploads directory with security headers.
	// In remote object storage mode (S3, MinIO, Cloudflare R2), local disk is decoupled
	// and requests to legacy /uploads/:filepath are redirected to the configured public CDN/S3 URL.
	uploadDir := cfg.Storage.UploadDir
	if uploadDir == "" {
		uploadDir = "./uploads"
	}

	if cfg.Storage.Driver == storage.DriverLocal {
		r.Use(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/uploads") {
				c.Header("X-Content-Options", "nosniff")
				c.Header("Content-Security-Policy", "default-src 'none'; img-src 'self'; media-src 'self'")
			}
			c.Next()
		})
		r.Static("/uploads", uploadDir)
	} else {
		remoteBaseURL := cfg.Storage.S3.PublicBaseURL
		if remoteBaseURL == "" {
			remoteBaseURL = cfg.Storage.PublicBaseURL
		}
		if remoteBaseURL == "" && cfg.Storage.S3.Endpoint != "" {
			if cfg.Storage.S3.UsePathStyle {
				remoteBaseURL = fmt.Sprintf("%s/%s", strings.TrimRight(cfg.Storage.S3.Endpoint, "/"), cfg.Storage.S3.Bucket)
			} else {
				remoteBaseURL = cfg.Storage.S3.Endpoint
			}
		}
		if remoteBaseURL != "" {
			remoteBaseURL = strings.TrimRight(remoteBaseURL, "/")
			r.GET("/uploads/*filepath", func(c *gin.Context) {
				fp := strings.TrimPrefix(c.Param("filepath"), "/")
				c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("%s/%s", remoteBaseURL, fp))
			})
		}
	}

	if err := db.InitDBWithConfig(cfg); err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to initialize database")
		return nil, err
	}

	redisClient := config.NewRedisClient()
	cacheRedisService := cache.NewRedisCacheService(redisClient)
	tokenService := auth.NewJWTService(cacheRedisService)

	storageLogger := utils.NewLoggerWithPath("storage.log", "info")
	storageService, err := storage.NewStorageService(context.Background(), cfg.Storage, storageLogger)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to initialize storage service")
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

	searchClient, err := search.NewClient(cfg.Meilisearch.Host, cfg.Meilisearch.MasterKey, cfg.Meilisearch.IndexProducts)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to initialize Meilisearch client")
	}

	userRepo := repository.NewSqlUserRepository(db.DB)
	identityClient, err := client.NewGRPCIdentityClient(cfg.IdentityGRPCAddr, redisClient)
	if err != nil {
		logger.Log.Fatal().Err(err).Str("addr", cfg.IdentityGRPCAddr).Msg("Failed to initialize Identity gRPC client")
		return nil, err
	}
	userReader := client.NewIdentityUserReader(identityClient, userRepo)

	// Location Domain Initialization
	ghnToken := utils.GetEnv("GHN_TOKEN", "")
	ghnShopID, _ := strconv.Atoi(utils.GetEnv("GHN_SHOP_ID", "0"))
	ghnBaseURL := utils.GetEnv("GHN_BASE_URL", "")
	ghnAdapter := shipping.NewGHNAdapter(ghnToken, ghnShopID, ghnBaseURL)
	shippingFactory := shipping.NewShippingFactory(ghnAdapter)
	locationService := v1service.NewLocationService(shippingFactory, cacheRedisService)

	// Cross-domain Core Repositories & Service Singletons
	productRepo := repository.NewSqlProductRepository(db.DB)
	cartRepo := repository.NewSqlCartRepository(db.DB)
	cartService := v1service.NewCartService(cartRepo, userReader, productRepo, redisClient)

	couponRepo := repository.NewSqlCouponRepository(db.DB)
	couponService := v1service.NewCouponService(couponRepo, userReader, redisClient)

	stockMovementRepo := repository.NewSqlStockMovementRepository(db.DB)
	inventoryService := v1service.NewInventoryService(productRepo, stockMovementRepo, userReader, txManager)

	paymentRepo := repository.NewSqlPaymentRepository(db.DB)
	webhookRepo := repository.NewSqlWebhookEventRepository(db.DB)
	orderRepo := repository.NewSqlOrderRepository(db.DB)
	orderReader := v1service.NewOrderReaderAdapter(orderRepo)
	outboxRepo := repository.NewSqlOutboxRepository(db.DB)

	paymentConfig := payment.PaymentConfig{
		MoMo: payment.MoMoConfig{
			PartnerCode: utils.GetEnv("MOMO_PARTNER_CODE", ""),
			AccessKey:   utils.GetEnv("MOMO_ACCESS_KEY", ""),
			SecretKey:   utils.GetEnv("MOMO_SECRET_KEY", ""),
			Endpoint:    utils.GetEnv("MOMO_ENDPOINT", ""),
			QueryURL:    utils.GetEnv("MOMO_QUERY_URL", ""),
		},
	}
	paymentProviders := make(map[string]payment.PaymentProvider)
	if momoFactory, err := payment.NewProviderFactory(payment.ProviderMoMo, paymentConfig); err == nil {
		if p, err := momoFactory.CreateProvider(); err == nil {
			paymentProviders["momo"] = p
		}
	}
	paymentReturnURL := utils.GetEnv("PAYMENT_RETURN_URL", "http://localhost:3001/payment/result")
	paymentWebhookBaseURL := utils.GetEnv("PAYMENT_WEBHOOK_BASE_URL", "http://localhost:8080")
	paymentService := v1service.NewPaymentService(
		paymentRepo, orderReader, webhookRepo,
		paymentProviders, cacheRedisService, rabbitmqService,
		paymentReturnURL, paymentWebhookBaseURL,
		outboxRepo,
	)

	ctx := &ModuleContext{
		DB:             db.DB,
		ReadDB:         db.GetReadDB(),
		IdentityDB:     db.GetIdentityDB(),
		Redis:          redisClient,
		TxManager:      txManager,
		Storage:        storageService,
		Search:         searchClient,
		IdentityClient: identityClient,
		RabbitMQ:       rabbitmqService,
	}

	orderModuleDeps := OrderModuleDeps{
		CartService:     cartService,
		CouponService:   couponService,
		InventoryService: inventoryService,
		PaymentService:  paymentService,
		LocationService: locationService,
		ShippingFactory: shippingFactory,
		RabbitMQService: rabbitmqService,
		CacheService:    cacheRedisService,
	}

	modules := []Module{
		NewProductModule(ctx),
		NewOrderModule(ctx, orderModuleDeps),
		NewCartModule(ctx, cartService),
		NewCouponModule(ctx, couponService),
		NewPaymentModule(ctx, cacheRedisService, rabbitmqService, wsHub, paymentService),
		NewReviewModule(ctx, rabbitmqService),
		NewWSModule(ctx, wsHub, tokenService, cacheRedisService),
		NewLocationModule(locationService),
		NewUploadModule(ctx),
		NewBannerModule(ctx, cacheRedisService),
		NewAnalyticsModule(ctx),
		NewChatModule(ctx, wsHub, redisBridge),
		NewHealthModule(db.DBPool, redisClient, rabbitmqService, storageService, discord.NewDiscordNotifier(utils.GetEnv("DISCORD_WEBHOOK_URL", ""), logger.Log), logger.Log),
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

func getModuleRoutes(modules []Module) []routes.Route {
	var moduleRoutes []routes.Route
	for _, module := range modules {
		moduleRoutes = append(moduleRoutes, module.Routes()...)
	}
	return moduleRoutes
}

func getAdminModuleRoutes(modules []Module) []routes.Route {
	var adminModuleRoutes []routes.Route
	for _, module := range modules {
		if adminModule, ok := module.(AdminModule); ok {
			adminModuleRoutes = append(adminModuleRoutes, adminModule.AdminRoutes()...)
		}
	}
	return adminModuleRoutes
}

func (a *Application) Run() error {
	srv := &http.Server{
		Addr:    a.config.ServerAddress,
		Handler: a.router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Fatal().Err(err).Msg("Server failed to start")
		}
	}()

	logger.Log.Info().Msgf("Server is running on %s", a.config.ServerAddress)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Log.Info().Msg("Server is shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Log.Fatal().Err(err).Msg("Server forced to shutdown")
		return err
	}

	logger.Log.Info().Msg("Server exited successfully")
	return nil
}


