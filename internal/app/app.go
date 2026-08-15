package app

import (
	"context"
	"goshop/internal/config"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	"goshop/internal/routes"
	"goshop/internal/validation"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type Module interface {
	Routes() routes.Route
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

func NewApplication(cfg *config.Config) *Application {

	validation.InitValidator()

	r := gin.Default()

	if err := db.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	redisClient := config.NewRedisClient()
	cacheRedisService := cache.NewRedisCacheService(redisClient)
	tokenService := auth.NewJWTService(cacheRedisService)

	ctx := &ModuleContext{
		DB:    db.DB,
		Redis: redisClient,
	}

	modules := []Module{
		NewUserModule(ctx),
		NewAuthModule(ctx, tokenService, cacheRedisService),
	}

	routes.RegisterRoutes(r, tokenService, cacheRedisService, getModuleRoutes(modules)...)

	return &Application{
		config:  cfg,
		router:  r,
		modules: modules,
	}
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
		log.Printf("✅ Server is running at: %s", a.config.ServerAddress)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("⛔ Failed to start server: %v", err)
		}
	}()

	<-quit
	log.Println("⚠️  Shutdown signal received ...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("⛔ Server forced to shutdown: %v", err)
	}

	log.Println("☑️  Server exited gracefully")

	return nil
}

func getModuleRoutes(modules []Module) []routes.Route {
	routeList := make([]routes.Route, len(modules))
	for i, module := range modules {
		routeList[i] = module.Routes()
	}
	return routeList
}
