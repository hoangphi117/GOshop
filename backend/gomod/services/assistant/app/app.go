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

	assistanthandler "goshop/services/assistant/internal/handler"
	assistantroutes "goshop/services/assistant/internal/routes"
	assistantservice "goshop/services/assistant/internal/service"
	catalogcontract "goshop/services/catalog/contract"
	"goshop/shared/config"
	"goshop/shared/pkg/auth"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/llm"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/metrics"
	"goshop/shared/pkg/utils"
	"goshop/shared/pkg/validation"
)

// Dependencies (adapters for the two catalog ports) live in app/adapters.go:
// keeping catalog imports out of this package is what makes the service
// unit-testable with fakes and nothing else.
type App struct {
	server  *http.Server
	closers []func() error
}

func New(cfg *config.Config) (*App, error) {
	if err := validation.InitValidator(); err != nil {
		return nil, fmt.Errorf("failed to initialize validator: %w", err)
	}

	r := gin.Default()
	r.GET("/metrics", metrics.Handler(nil))
	if cfg.AppEnv == "production" {
		if err := r.SetTrustedProxies([]string{
			"127.0.0.1", "::1", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
		}); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to configure trusted proxies for assistant service")
		}
	} else if err := r.SetTrustedProxies([]string{"127.0.0.1", "::1"}); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to configure dev trusted proxies")
	}

	ac := cfg.Assistant
	redisClient := config.NewRedisClient()
	cacheRedisService := cache.NewRedisCacheService(redisClient)
	tokenService := auth.NewJWTService(cacheRedisService)

	var closers []func() error

	opts := assistantservice.Options{
		Sessions: assistantservice.NewRedisSessionStore(
			redisClient,
			time.Duration(ac.SessionTTL)*time.Second,
			ac.SessionTurns*2,
		),
		Model:     ac.Model,
		MaxTokens: ac.MaxTokens,
		// Guests get a smaller bucket than signed-in users; the service only
		// knows the key it was handed, so the split happens here.
		Limiter: newCallerLimiter(redisClient, int64(ac.DailyPerUser)),
	}

	// The three catalogs ports. Each is optional: a building that only answers
	// FAQs is better than a service that refuses to boot because Meilisearch is
	// down.
	if search, err := catalogcontract.NewHTTPProductSearchClient(
		utils.GetEnv("CATALOG_BASE_URL", "http://localhost:8086"),
		5*time.Second,
	); err != nil {
		logger.Log.Warn().Err(err).Msg("Product search disabled")
	} else {
		opts.Products = productSearchAdapter{search: search}
	}

	if faqs, err := newFAQSearchAdapter(
		utils.GetEnv("CATALOG_BASE_URL", "http://localhost:8086"),
		5*time.Second,
	); err != nil {
		logger.Log.Warn().Err(err).Msg("FAQ search disabled")
	} else {
		opts.FAQs = faqs
	}

	catalogTarget := utils.GetEnv("CATALOG_GRPC_TARGET", "localhost:50055")
	if catalogClient, err := catalogcontract.NewGRPCCatalogClient(catalogTarget); err != nil {
		logger.Log.Warn().Err(err).Str("target", catalogTarget).Msg("Product reader disabled")
	} else {
		opts.Reader = productReaderAdapter{client: catalogClient}
		closers = append(closers, catalogClient.Close)
	}

	if ac.Enabled {
		client, err := llm.NewOpenAIClient(llm.OpenAIConfig{
			BaseURL:     ac.BaseURL,
			APIKey:      ac.APIKey,
			Model:       ac.Model,
			MaxTokens:   ac.MaxTokens,
			Temperature: ac.Temperature,
			Timeout:     time.Duration(ac.TimeoutSec) * time.Second,
		})
		if err != nil {
			// Config.Never panics: an invalid LLM setup disables the feature
			// while the service still boots and answers 503.
			logger.Log.Warn().Err(err).Msg("Assistant LLM disabled")
		} else {
			opts.LLM = client
		}
	} else {
		logger.Log.Warn().Msg("Assistant disabled: ASSISTANT_API_KEY or ASSISTANT_MODEL is empty")
	}

	svc := assistantservice.NewChatService(opts)
	h := assistanthandler.NewAssistantHandler(svc, ac.Enabled, ac.SessionTTL)
	assistantroutes.RegisterRoutes(r, h, tokenService, cacheRedisService)

	app := &App{
		server: &http.Server{
			Addr:        ":" + utils.GetEnv("ASSISTANT_SERVER_PORT", "8097"),
			Handler:     r,
			ReadTimeout: 15 * time.Second,
			// The model may take up to TimeoutSec; the write deadline must be
			// longer than one full tool loop (4 rounds).
			WriteTimeout:   time.Duration(ac.TimeoutSec*max(1, 4)) * time.Second,
			MaxHeaderBytes: 1 << 20,
		},
		closers: closers,
	}
	return app, nil
}

func (a *App) Run(ctx context.Context) error {
	httpErrors := make(chan error, 1)
	go func() {
		logger.Log.Info().Str("addr", a.server.Addr).Msg("Assistant HTTP service listening and serving")
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			httpErrors <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-httpErrors:
		return fmt.Errorf("assistant HTTP server error: %w", err)
	case sig := <-quit:
		logger.Log.Info().Str("signal", sig.String()).Msg("Assistant service shutting down")
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.server.Shutdown(shutdownCtx); err != nil {
		logger.Log.Warn().Err(err).Msg("Assistant HTTP server forced shutdown")
	}
	for _, closeFn := range a.closers {
		_ = closeFn()
	}
	return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
