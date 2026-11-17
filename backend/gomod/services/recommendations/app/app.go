package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	catalogcontract "goshop/services/catalog/contract"
	"goshop/services/recommendations/internal/embed"
	recshandler "goshop/services/recommendations/internal/handler"
	"goshop/services/recommendations/internal/migrations"
	recsrepo "goshop/services/recommendations/internal/repository"
	recsroutes "goshop/services/recommendations/internal/routes"
	recssvc "goshop/services/recommendations/internal/service"
	"goshop/shared/config"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/db"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/metrics"
	"goshop/shared/pkg/utils"
)

type App struct {
	cfg    *config.Config
	server *http.Server
}

func New(cfg *config.Config) (*App, error) {
	r := gin.Default()
	r.GET("/metrics", metrics.Handler(nil))

	if cfg.AppEnv == "production" {
		if err := r.SetTrustedProxies([]string{
			"127.0.0.1",
			"::1",
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
		}); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to configure trusted proxies for recommendations service")
		}
	} else if err := r.SetTrustedProxies([]string{"127.0.0.1", "::1"}); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to configure dev trusted proxies")
	}

	if err := db.InitDBWithConfig(cfg, "recommendations"); err != nil {
		return nil, fmt.Errorf("database init failed: %w", err)
	}

	// The foreign tables need the link before they can be read, so the FDW
	// servers and their user mappings are created here, before migrations.
	ensureOrderServer(db.DBPool, cfg)
	ensureCatalogServer(db.DBPool, cfg)
	ensureAnalyticsServer(db.DBPool, cfg)
	runMigrations(cfg)

	catalogTarget := utils.GetEnv("CATALOG_GRPC_TARGET", "")
	if catalogTarget == "" {
		return nil, fmt.Errorf("CATALOG_GRPC_TARGET is required")
	}
	catalogClient, err := catalogcontract.NewGRPCCatalogClient(catalogTarget)
	if err != nil {
		return nil, fmt.Errorf("catalog gRPC client: %w", err)
	}

	catalogURL := utils.GetEnv("CATALOG_SERVICE_URL", "http://localhost:8086")
	fallbackLister, err := recssvc.NewHTTPCatalogLister(catalogURL, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("catalog lister: %w", err)
	}

	repo := recsrepo.NewSqlPopularityRepository(db.DBPool)
	affinityRepo := recsrepo.NewSqlAffinityRepository(db.DBPool)
	embeddingRepo := recsrepo.NewSqlEmbeddingRepository(db.DBPool)
	centroidRepo := recsrepo.NewSqlCentroidRepository(db.DBPool)
	shelfService := recssvc.NewRecommendationService(repo, catalogClient, fallbackLister)

	// Embeddings are optional: without EMBEDDER_URL the vectors stay empty and
	// every vector shelf degrades to best sellers. This is why the sidecar can
	// be absent from a dev box without breaking the shop. The same service also
	// serves the publish path, so a nil embedder makes that route a no-op 503
	// rather than a panic.
	var embeddingSvc *recssvc.EmbeddingService
	// embedder stays nil when the sidecar is absent; hybrid search then serves
	// its keyword half and says so in `source`.
	var embedder embed.Embedder
	embedderURL := utils.GetEnv("EMBEDDER_URL", "")
	if embedderURL != "" {
		client, err := embed.NewHTTPEmbedder(embedderURL, 60*time.Second)
		if err != nil {
			logger.Log.Warn().Err(err).Msg("Embedder client not configured; vector shelves will degrade")
		} else {
			embedder = client
			embeddingSvc = recssvc.NewEmbeddingService(embeddingRepo, client)
		}
	}

	// The keyword half of hybrid search reads the catalog's suggestion route.
	// It points at CATALOG_SERVICE_URL, which in every real topology is the
	// gateway: the gateway injects the API key for this one route, because this
	// service holds none.
	suggester := recssvc.NewCatalogSuggester(
		utils.GetEnv("CATALOG_SERVICE_URL", "http://localhost:8086"),
		utils.GetEnv("CATALOG_API_KEY", ""),
	)

	covisitationRepo := recsrepo.NewSqlCovisitationRepository(db.DBPool)

	personalizedService := recssvc.NewPersonalizedService(
		affinityRepo, embeddingRepo, centroidRepo, repo, catalogClient,
		suggester, embedder, shelfCache(), covisitationRepo,
	)

	recsroutes.RegisterRoutes(
		r,
		recshandler.NewRecommendationHandler(shelfService),
		recshandler.NewPersonalizedHandler(shelfService, personalizedService),
		recshandler.NewReembedHandler(embeddingSvc, personalizedService),
	)

	// Read-model refresh is started only after every migration has landed. It
	// used to start here directly, which on a fresh database raced the
	// migrations and failed with "relation ... does not exist".
	refreshReadModelsAfterMigrations(
		db.DBPool,
		cfg,
		repo,
		covisitationRepo,
		func(ctx context.Context) (int, error) {
			if embeddingSvc == nil {
				return 0, nil
			}
			return embeddingSvc.Backfill(ctx)
		},
	)

	port := utils.GetEnv("RECOMMENDATIONS_SERVER_PORT", "8095")
	return &App{
		cfg: cfg,
		server: &http.Server{
			Addr:           ":" + port,
			Handler:        r,
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			MaxHeaderBytes: 1 << 20,
		},
	}, nil
}

// newShelfCache returns the shelf cache, or nil when Redis is not configured.
//
// Nil is the degraded-but-correct mode: every recommendation read goes to the
// database, which is what this service did before the cache existed. The
// alternative — a boot-time Ping that fails the container — would make Redis a
// dependency of the homepage, and a cache is never worth that.
func shelfCache() cache.RedisCacheService {
	if utils.GetEnv("REDIS_ADDR", "") == "" {
		logger.Log.Info().Msg("REDIS_ADDR not set; recommendation shelves are uncached")
		return nil
	}
	// NewRedisClient pings and Fatals on failure, which is right for services
	// that cannot run without Redis and wrong for this one.
	client := redis.NewClient(&redis.Options{
		Addr:         utils.GetEnv("REDIS_ADDR", "localhost:6379"),
		Username:     utils.GetEnv("REDIS_USERNAME", ""),
		Password:     utils.GetEnv("REDIS_PASSWORD", ""),
		DB:           utils.GetIntEnv("REDIS_DB", 0),
		PoolSize:     10,
		MinIdleConns: 2,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		logger.Log.Warn().Err(err).Msg("Redis unreachable; recommendation shelves are uncached")
		return nil
	}
	logger.Log.Info().Msg("Recommendation shelf cache enabled")
	return cache.NewRedisCacheService(client)
}

// refreshReadModelsAfterMigrations rebuilds the three read models in the order
// their dependencies require, off the request path.
//
// Order matters: the best-seller rollup is independent, embeddings must exist
// before the vectors exist (a centroid is a mean of vectors). Affinity is
// computed on read, so there is nothing to roll up here any more; the backfill
// is the only remaining boot job and it is logged and swallowed: a stale shelf
// is a degraded homepage, not a failed boot.
func refreshReadModelsAfterMigrations(
	pool *pgxpool.Pool,
	cfg *config.Config,
	popularityRepo recsrepo.PopularityRepository,
	covisitationRepo recsrepo.CovisitationRepository,
	backfill func(context.Context) (int, error),
) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if n, err := popularityRepo.Rebuild(ctx, 30); err != nil {
			logger.Log.Warn().Err(err).Msg("Best-seller rollup refresh failed; serving rating fallback")
		} else {
			logger.Log.Info().Int("products", n).Msg("Best-seller rollup refreshed")
		}
	}()

	// Co-visitation is an aggregate over events, so it goes stale as people
	// browse rather than as products change. It is rebuilt at boot and then on a
	// timer: 30 minutes is far shorter than the shelf TTL, so a pair that forms
	// this morning is in the table by lunch, and far longer than an event write,
	// so the table is not rebuilt for one page view.
	go func() {
		covisitationLoop(covisitationRepo, 30*time.Minute)
	}()

	go func() {
		// A full re-embed of 5000 products on one CPU core is minutes, not
		// seconds; the hash check keeps later boots to a single catalog read.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		n, err := backfill(ctx)
		if err != nil {
			logger.Log.Warn().Err(err).Int("written", n).Msg("Embedding backfill failed; will resume on next boot")
		}
	}()
}

// covisitationLoop rebuilds the pair table now and then every interval.
//
// A failure is a warning at every tick, never fatal: the shelf falls back to the
// vector pool, which is what a database without migration 000007 gets too.
func covisitationLoop(repo recsrepo.CovisitationRepository, interval time.Duration) {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		n, err := repo.Rebuild(ctx, 30)
		cancel()
		if err != nil {
			logger.Log.Warn().Err(err).Msg("Co-visitation rebuild failed; shelves use the vector pool only")
		} else {
			logger.Log.Info().Int("pairs", n).Msg("Co-visitation table refreshed")
		}
		time.Sleep(interval)
	}
}

// ensureOrderServer creates the postgres_fdw server and user mapping that the
// foreign tables in 000001_popularity.up.sql point at.
//
// It cannot live in a migration: host and credentials come from the
// environment and PostgreSQL cannot read one. Both objects are recreated every
// start so a rotated password takes effect; the mapping is dropped and rebuilt
// rather than altered so no option has to already exist.
//
// Failures are warnings, not fatal: without the link every rollup rebuild
// errors and the API serves the rating fallback, so the homepage still renders.
func ensureOrderServer(pool *pgxpool.Pool, cfg *config.Config) {
	ensureForeignServer(pool, cfg, foreignServer{
		Name:    "order_db",
		HostEnv: "ORDER_DB_HOST",
		NameEnv: "ORDER_DB_NAME",
		PortEnv: "ORDER_DB_PORT",
		DBName:  "shopping_order",
		Warn:    "Order database link not configured; best sellers will use the rating fallback",
	})
}

// ensureCatalogServer does the same for the catalog mirror used by the
// embedding backfill (000003_catalog_mirror.up.sql).
func ensureCatalogServer(pool *pgxpool.Pool, cfg *config.Config) {
	ensureForeignServer(pool, cfg, foreignServer{
		Name:    "catalog_db",
		HostEnv: "CATALOG_DB_HOST",
		NameEnv: "CATALOG_DB_NAME",
		PortEnv: "CATALOG_DB_PORT",
		DBName:  "shopping_catalog",
		Warn:    "Catalog database link not configured; embeddings cannot be backfilled",
	})
}

// ensureAnalyticsServer links the analytics database for the affinity rollup
// (000004_analytics_mirror.up.sql).
func ensureAnalyticsServer(pool *pgxpool.Pool, cfg *config.Config) {
	ensureForeignServer(pool, cfg, foreignServer{
		Name:    "analytics_db",
		HostEnv: "ANALYTICS_DB_HOST",
		NameEnv: "ANALYTICS_DB_NAME",
		PortEnv: "ANALYTICS_DB_PORT",
		DBName:  "shopping_analytics",
		Warn:    "Analytics database link not configured; personalized shelves will be empty",
	})
}

type foreignServer struct {
	Name    string
	HostEnv string
	NameEnv string
	PortEnv string
	DBName  string
	Warn    string
}

func ensureForeignServer(pool *pgxpool.Pool, cfg *config.Config, s foreignServer) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	host := utils.GetEnv(s.HostEnv, cfg.DB.Host)
	dbName := utils.GetEnv(s.NameEnv, s.DBName)
	port := utils.GetEnv(s.PortEnv, cfg.DB.Port)
	user := utils.GetEnv("ORDER_DB_USER", cfg.DB.User)

	options := func(pairs ...string) string {
		return "OPTIONS (" + strings.Join(pairs, ", ") + ")"
	}

	// DROP SERVER ... CASCADE would take the foreign tables with it, and those
	// are created by migrations that have already run — the mirror would
	// vanish on every restart. ALTER on an existing server plus a replaced user
	// mapping reaches the same end state (rotated password included) and
	// touches no tables.
	statements := []string{
		`CREATE EXTENSION IF NOT EXISTS postgres_fdw`,
		`DO $do$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_foreign_server WHERE srvname = ` + sqlLiteral(s.Name) + `) THEN
        EXECUTE format('ALTER SERVER %I OPTIONS (SET host %L, SET dbname %L, SET port %L)',
            ` + sqlLiteral(s.Name) + `,
            ` + sqlLiteral(host) + `,
            ` + sqlLiteral(dbName) + `,
            ` + sqlLiteral(port) + `);
    ELSE
        EXECUTE format('CREATE SERVER %I FOREIGN DATA WRAPPER postgres_fdw OPTIONS (host %L, dbname %L, port %L)',
            ` + sqlLiteral(s.Name) + `,
            ` + sqlLiteral(host) + `,
            ` + sqlLiteral(dbName) + `,
            ` + sqlLiteral(port) + `);
    END IF;
END
$do$`,
		`DROP USER MAPPING IF EXISTS FOR CURRENT_USER SERVER ` + s.Name,
		`CREATE USER MAPPING FOR CURRENT_USER SERVER ` + s.Name + ` ` + options(
			"user "+sqlLiteral(user),
			"password "+sqlLiteral(cfg.DB.Password),
		),
	}

	for _, stmt := range statements {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			logger.Log.Warn().Err(err).Str("server", s.Name).Msg(s.Warn)
			return
		}
	}
}

func sqlLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func runMigrations(cfg *config.Config) {
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s&x-migrations-table=schema_migrations_recommendations",
		cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.DBName, cfg.DB.SSLMode)

	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Could not load embedded migrations source for recommendations service")
		return
	}
	m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, connStr)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Could not initialize migrate instance for recommendations service")
		return
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			logger.Log.Warn().Err(srcErr).Err(dbErr).Msg("Error closing migration handles")
		}
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		logger.Log.Error().Err(err).Msg("Failed to execute database migrations for recommendations service")
		return
	}
	logger.Log.Info().Msg("Recommendations service database migrations applied successfully")
}

func (a *App) Run(ctx context.Context) error {
	httpErrors := make(chan error, 1)

	go func() {
		logger.Log.Info().Str("addr", a.server.Addr).Msg("Recommendations HTTP service listening and serving")
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			httpErrors <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-httpErrors:
		return fmt.Errorf("recommendations HTTP server error: %w", err)
	case sig := <-quit:
		logger.Log.Info().Str("signal", sig.String()).Msg("Recommendations service shutting down gracefully")
	case <-ctx.Done():
		logger.Log.Info().Msg("Context canceled, shutting down Recommendations service")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("recommendations HTTP server forced shutdown: %w", err)
	}

	logger.Log.Info().Msg("Recommendations service exited properly")
	return nil
}
