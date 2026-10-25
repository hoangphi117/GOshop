package app

import (
	"context"
	"fmt"
	"goshop/internal/client"
	"goshop/internal/config"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	"goshop/internal/routes"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/validation"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/discord"
	"goshop/pkg/logger"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/search"
	"goshop/pkg/shipping"
	"goshop/pkg/storage"
	"goshop/pkg/ws"
	catalogcontract "goshop/services/catalog/contract"
	couponcontract "goshop/services/coupon/contract"
	inventorycontract "goshop/services/inventory/contract"
	ordercontract "goshop/services/order/contract"
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
	DB              sqlc.Querier
	ReadDB          sqlc.Querier
	IdentityDB      sqlc.Querier
	Redis           *redis.Client
	TxManager       db.TxManager
	Storage         storage.StorageService
	Search          *search.Client
	IdentityClient  client.IdentityClient
	RabbitMQ        rabbitmq.RabbitMQService
	InventoryClient inventorycontract.InventoryClient
	CatalogClient   catalogcontract.CatalogClient
	CouponClient    couponcontract.CouponClient
	OrderClient     ordercontract.OrderClient
}

func NewApplication(cfg *config.Config) (*Application, error) {

	if err := validation.InitValidator(); err != nil {
		return nil, fmt.Errorf("failed to initialize validator: %w", err)
	}

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

	if err := db.InitDBWithConfig(cfg, "api"); err != nil {
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

	identityClient, err := client.NewGRPCIdentityClient(cfg.IdentityGRPCAddr, redisClient)
	if err != nil {
		logger.Log.Fatal().Err(err).Str("addr", cfg.IdentityGRPCAddr).Msg("Failed to initialize Identity gRPC client")
		return nil, err
	}
	// Location Domain Initialization
	ghnToken := utils.GetEnv("GHN_TOKEN", "")
	ghnShopID, _ := strconv.Atoi(utils.GetEnv("GHN_SHOP_ID", "0"))
	ghnBaseURL := utils.GetEnv("GHN_BASE_URL", "")
	ghnAdapter := shipping.NewGHNAdapter(ghnToken, ghnShopID, ghnBaseURL)
	shippingFactory := shipping.NewShippingFactory(ghnAdapter)
	locationService := v1service.NewLocationService(shippingFactory, cacheRedisService)

	// Cross-domain Core Service Singletons
	catalogTarget := utils.GetEnv("CATALOG_GRPC_TARGET", "localhost:50055")
	grpcCatalogClient, err := catalogcontract.NewGRPCCatalogClient(catalogTarget)
	if err != nil {
		logger.Log.Warn().Err(err).Str("addr", catalogTarget).Msg("Failed to initialize Catalog gRPC client")
	}

	couponTarget := utils.GetEnv("COUPON_GRPC_TARGET", "localhost:50053")
	grpcCouponClient, err := couponcontract.NewGRPCCouponClient(couponTarget)
	if err != nil {
		logger.Log.Fatal().Err(err).Str("addr", couponTarget).Msg("Failed to initialize Coupon gRPC client")
		return nil, err
	}

	inventoryTarget := utils.GetEnv("INVENTORY_GRPC_TARGET", "localhost:50054")
	grpcInventoryClient, err := inventorycontract.NewGRPCInventoryClient(inventoryTarget)
	if err != nil {
		logger.Log.Fatal().Err(err).Str("addr", inventoryTarget).Msg("Failed to initialize Inventory gRPC client")
		return nil, err
	}

	orderTarget := utils.GetEnv("ORDER_GRPC_TARGET", utils.GetEnv("ORDER_GRPC_ADDR", "localhost:50057"))
	grpcOrderClient, err := ordercontract.NewGRPCOrderClient(orderTarget)
	if err != nil {
		logger.Log.Warn().Err(err).Str("addr", orderTarget).Msg("Failed to initialize Order gRPC client")
	}

	ctx := &ModuleContext{
		DB:              db.DB,
		ReadDB:          db.GetReadDB(),
		IdentityDB:      db.GetIdentityDB(),
		Redis:           redisClient,
		TxManager:       txManager,
		Storage:         storageService,
		Search:          searchClient,
		IdentityClient:  identityClient,
		RabbitMQ:        rabbitmqService,
		InventoryClient: grpcInventoryClient,
		CatalogClient:   grpcCatalogClient,
		CouponClient:    grpcCouponClient,
		OrderClient:     grpcOrderClient,
	}

	var crossDomainOrderReader v1service.OrderReader
	if grpcOrderClient != nil {
		crossDomainOrderReader = client.NewGRPCOrderReader(grpcOrderClient)
	}

	modules := []Module{
		NewReviewModule(ctx, rabbitmqService, crossDomainOrderReader),
		NewWSModule(ctx, wsHub, tokenService, cacheRedisService),
		NewLocationModule(locationService),
		NewUploadModule(ctx),
		NewAnalyticsModule(ctx),
		NewChatModule(ctx, wsHub, redisBridge, crossDomainOrderReader),
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
