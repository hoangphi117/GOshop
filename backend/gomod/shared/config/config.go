package config

import (
	"fmt"
	"strconv"
	"strings"

	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/storage"
	"goshop/shared/pkg/utils"
)

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type MeilisearchConfig struct {
	Host          string
	MasterKey     string
	IndexProducts string
}

// AssistantConfig holds the LLM settings for the shopping assistant service.
// The API key is a server-side secret only; never expose it to a browser
// bundle (NEXT_PUBLIC_*/VITE_*). Missing key disables the feature instead of
// failing startup, because every service builds this config.
type AssistantConfig struct {
	Enabled      bool
	Provider     string // free-form label, used in logs only
	BaseURL      string
	APIKey       string
	Model        string
	MaxTokens    int
	Temperature  float64
	TimeoutSec   int
	DailyPerUser int
	SessionTTL   int
	SessionTurns int
}

type Config struct {
	AppEnv             string // Environment: "development", "production", "staging"
	ServerAddress      string
	DB                 DatabaseConfig
	ReadDB             DatabaseConfig
	Meilisearch        MeilisearchConfig
	AppURL             string
	StorefrontURL      string
	MailProviderType   string
	MailProviderConfig map[string]any
	Storage            storage.StorageConfig
	IdentityServiceURL string
	IdentityGRPCAddr   string
	PaymentServiceURL  string
	PaymentGRPCAddr    string
	InternalSecret     string
	Assistant          AssistantConfig
}

func NewConfig() *Config {

	// Single source of truth for the browser-facing origin (the edge). Env
	// files only switch this one value between dev/prod; everything derived
	// (storefront links, payment return URL, media base) follows. Explicit
	// per-var overrides still win when set.
	appOrigin := strings.TrimRight(utils.GetEnv("APP_ORIGIN", ""), "/")

	mailProviderConfig := make(map[string]any)

	// The sender identity is provider-neutral on purpose: switching providers
	// must not force a rename of the variables in every env file.
	mailProviderType := utils.GetEnv("MAIL_PROVIDER_TYPE", "smtp")
	switch mailProviderType {
	case "smtp":
		mailProviderConfig["smtp"] = map[string]any{
			"host":     utils.GetEnv("SMTP_HOST", "smtp.gmail.com"),
			"port":     utils.GetEnv("SMTP_PORT", "587"),
			"username": utils.GetEnv("SMTP_USERNAME", ""),
			// Gmail shows the App Password in four groups of four; copying it
			// with those spaces is the most common reason authentication fails.
			"password":         strings.ReplaceAll(utils.GetEnv("SMTP_PASSWORD", ""), " ", ""),
			"mail_sender":      utils.GetEnv("MAIL_SENDER", ""),
			"mail_sender_name": utils.GetEnv("MAIL_SENDER_NAME", "GOshop Admin"),
		}
	}

	usePathStyle, _ := strconv.ParseBool(utils.GetEnv("S3_USE_PATH_STYLE", "true"))
	useSSL, _ := strconv.ParseBool(utils.GetEnv("S3_USE_SSL", "false"))

	// Derived defaults: only fill when APP_ORIGIN is set and the specific var
	// is absent, so an explicit value always wins.
	s3PublicBaseDefault := ""
	if appOrigin != "" {
		s3PublicBaseDefault = appOrigin + "/goshop-media"
	}
	storefrontURLDefault := "http://localhost:3001"
	if appOrigin != "" {
		storefrontURLDefault = appOrigin
	}

	storageConfig := storage.StorageConfig{
		Driver:        storage.StorageDriver(utils.GetEnv("STORAGE_DRIVER", "minio")),
		PublicBaseURL: utils.GetEnv("STORAGE_PUBLIC_BASE_URL", ""),
		S3: storage.S3Config{
			Endpoint:      utils.GetEnv("S3_ENDPOINT", ""),
			Region:        utils.GetEnv("S3_REGION", "auto"),
			Bucket:        utils.GetEnv("S3_BUCKET", "goshop-media"),
			AccessKey:     utils.GetEnv("S3_ACCESS_KEY", ""),
			SecretKey:     utils.GetEnv("S3_SECRET_KEY", ""),
			PublicBaseURL: utils.GetEnv("S3_PUBLIC_BASE_URL", s3PublicBaseDefault),
			UsePathStyle:  usePathStyle,
			ForceSSL:      useSSL,
		},
	}

	return &Config{
		AppEnv:        utils.GetEnv("APP_ENV", "development"),
		ServerAddress: (&Config{}).serverAddress(),
		DB: DatabaseConfig{
			Host:     utils.GetEnv("DB_HOST", "localhost"),
			Port:     utils.GetEnv("DB_PORT", "5432"),
			User:     utils.GetEnv("DB_USER", "postgres"),
			Password: utils.GetEnv("DB_PASSWORD", "postgres"),
			DBName:   utils.GetEnv("DB_NAME", "my-app"),
			SSLMode:  utils.GetEnv("DB_SSL_MODE", "disable"),
		},
		ReadDB: DatabaseConfig{
			Host:     utils.GetEnv("DB_READ_HOST", utils.GetEnv("DB_HOST", "localhost")),
			Port:     utils.GetEnv("DB_READ_PORT", utils.GetEnv("DB_PORT", "5432")),
			User:     utils.GetEnv("DB_READ_USER", utils.GetEnv("DB_USER", "postgres")),
			Password: utils.GetEnv("DB_READ_PASSWORD", utils.GetEnv("DB_PASSWORD", "postgres")),
			DBName:   utils.GetEnv("DB_READ_NAME", utils.GetEnv("DB_NAME", "my-app")),
			SSLMode:  utils.GetEnv("DB_READ_SSL_MODE", utils.GetEnv("DB_SSL_MODE", "disable")),
		},
		Meilisearch: MeilisearchConfig{
			Host:          utils.GetEnv("MEILISEARCH_HOST", "http://localhost:7700"),
			MasterKey:     utils.GetEnv("MEILISEARCH_MASTER_KEY", "masterKey123"),
			IndexProducts: utils.GetEnv("MEILISEARCH_INDEX_PRODUCTS", "products"),
		},
		AppURL:             utils.GetEnv("APP_URL", "http://localhost:8080"),
		StorefrontURL:      utils.GetEnv("STOREFRONT_URL", storefrontURLDefault),
		MailProviderType:   mailProviderType,
		MailProviderConfig: mailProviderConfig,
		Storage:            storageConfig,
		IdentityServiceURL: utils.GetEnv("IDENTITY_SERVICE_URL", "http://localhost:8081"),
		IdentityGRPCAddr:   utils.GetEnv("IDENTITY_GRPC_ADDR", "localhost:50051"),
		PaymentServiceURL:  utils.GetEnv("PAYMENT_SERVICE_URL", "http://localhost:8083"),
		PaymentGRPCAddr:    utils.GetEnv("PAYMENT_GRPC_ADDR", "localhost:50052"),
		InternalSecret: func() string {
			secret := utils.GetEnv("INTERNAL_API_SECRET", "")
			if utils.GetEnv("APP_ENV", "development") == "production" && secret == "" {
				logger.Log.Fatal().Msg("FATAL: INTERNAL_API_SECRET must be explicitly configured in production environment")
			}
			if secret == "" {
				return "goshop-internal-secret-token"
			}
			return secret
		}(),
		Assistant: newAssistantConfig(),
	}
}

func newAssistantConfig() AssistantConfig {
	cfg := AssistantConfig{
		Provider:     utils.GetEnv("ASSISTANT_PROVIDER", "river"),
		BaseURL:      utils.GetEnv("ASSISTANT_BASE_URL", "https://api.river.ai/v1"),
		APIKey:       utils.GetEnv("ASSISTANT_API_KEY", ""),
		Model:        utils.GetEnv("ASSISTANT_MODEL", "deepseek-ai/DeepSeek-V4.1-Flash-1017"),
		MaxTokens:    envInt("ASSISTANT_MAX_TOKENS", 1024),
		Temperature:  envFloat("ASSISTANT_TEMPERATURE", 0.3),
		TimeoutSec:   envInt("ASSISTANT_TIMEOUT_SEC", 120),
		DailyPerUser: envInt("ASSISTANT_DAILY_PER_USER", 40),
		SessionTTL:   envInt("ASSISTANT_SESSION_TTL", 3600),
		SessionTurns: envInt("ASSISTANT_SESSION_TURNS", 6),
	}

	// Explicit ASSISTANT_ENABLED wins; otherwise the feature turns itself on
	// only when both a key and a model are present.
	enabledDefault := cfg.APIKey != "" && cfg.Model != ""
	cfg.Enabled = strings.EqualFold(utils.GetEnv("ASSISTANT_ENABLED", strconv.FormatBool(enabledDefault)), "true")

	if cfg.TimeoutSec <= 0 {
		cfg.TimeoutSec = 120
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 1024
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = 3600
	}
	if cfg.SessionTurns <= 0 {
		cfg.SessionTurns = 6
	}

	return cfg
}

func envInt(key string, fallback int) int {
	v, err := strconv.Atoi(utils.GetEnv(key, ""))
	if err != nil {
		return fallback
	}
	return v
}

func envFloat(key string, fallback float64) float64 {
	v, err := strconv.ParseFloat(utils.GetEnv(key, ""), 64)
	if err != nil {
		return fallback
	}
	return v
}

// Port normalises a port value taken from the environment.
//
// Env values pick up stray characters from a copy-paste or a dashboard form; a
// trailing backtick produced
//
//	listen tcp: lookup tcp/8081`: unknown port
//
// on Render, after the listener had already announced that it started, so the
// failure looked like a crash instead of a bad value. Anything that is not
// digits after trimming falls back to the caller's default.
func Port(raw string) string {
	cleaned := strings.Trim(raw, " \t\r\n`'\"")
	cleaned = strings.TrimPrefix(cleaned, ":")
	if cleaned == "" {
		return "8080"
	}
	for _, r := range cleaned {
		if r < '0' || r > '9' {
			return "8080"
		}
	}
	return cleaned
}

// serverAddress is the listen address every service builds from SERVER_PORT.
func (c *Config) serverAddress() string {
	return ":" + Port(utils.GetEnv("SERVER_PORT", "8080"))
}

// MigrationsEnabled reports whether a service should run its schema migrations
// at boot. It defaults to true so nothing changes for local compose.
//
// The reason to turn it off exists only in a deployment that runs every service
// in one container against a session-limited pooler. Each service opens its
// database pool first and golang-migrate takes a second connection on top, so
// seventeen services booting at once want roughly twice as many session slots
// as the database has. The losers die with
//
//	pq: remaining connection slots are reserved for roles with the SUPERUSER attribute
//
// which reads like a permissions problem and is really exhaustion. Migrating
// once and then setting RUN_MIGRATIONS=false leaves the pool as each service's
// only connection.
func MigrationsEnabled() bool {
	return utils.GetEnv("SKIP_MIGRATIONS", "false") != "true"
}

// MigrationPort is the port migrations must use, which is not always the one
// the services use. golang-migrate takes a session-level advisory lock and
// relies on prepared statements; a transaction pooler (Supavisor :6543,
// pgbouncer) breaks both, failing with
//
//	bind message supplies 1 parameters, but prepared statement "" requires 2
//
// DB_DIRECT_PORT points at the session-mode port (:5432) where that works.
//
// An existing schema hides this: migrate only takes the lock when it has
// pending work, so a stack migrated locally keeps booting against the pooler
// right up until someone deploys against an empty database.
func (c *Config) MigrationPort() string {
	return utils.GetEnv("DB_DIRECT_PORT", c.DB.Port)
}

func (c *Config) MigrationHost() string {
	return utils.GetEnv("DB_DIRECT_HOST", c.DB.Host)
}

// MigrationURL is the postgres:// DSN for golang-migrate, pointed at the
// session-mode host/port. Every migration set - the service's own and the
// event ledger's - must use it: migrate takes a session advisory lock, which a
// transaction pooler cannot hold, and on a full pool the pooler answers
//
//	pq: remaining connection slots are reserved for roles with the SUPERUSER attribute
//
// which reads like a permissions bug but is only exhaustion.
//
// searchPath and migrationsTable are per-database because several embedded
// migration sets share one database and would otherwise collide on the
// version table.
//
// With SKIP_MIGRATIONS=true this returns an empty string on purpose. Every
// caller already treats an unusable DSN as "log a warning and carry on", so an
// empty one is the cheapest way to keep the migration code untouched while
// removing the extra connection it would otherwise take. See MigrationsEnabled.
func (c *Config) MigrationURL(searchPath, migrationsTable string) string {
	if !MigrationsEnabled() {
		return ""
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s&search_path=%s&x-migrations-table=%s",
		c.DB.User, c.DB.Password, c.MigrationHost(), c.MigrationPort(), c.DB.DBName, c.DB.SSLMode, searchPath, migrationsTable)
}

func (c *Config) DNS() string {
	if dbURL := utils.GetEnv("DATABASE_URL", utils.GetEnv("DB_URL", "")); dbURL != "" {
		return dbURL
	}
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", c.DB.Host, c.DB.Port, c.DB.User, c.DB.Password, c.DB.DBName, c.DB.SSLMode)
}

func (c *Config) ReadDNS() string {
	if replicaURL := utils.GetEnv("DB_READ_REPLICA_URL", ""); replicaURL != "" {
		return replicaURL
	}
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", c.ReadDB.Host, c.ReadDB.Port, c.ReadDB.User, c.ReadDB.Password, c.ReadDB.DBName, c.ReadDB.SSLMode)
}

func (c *Config) IdentityDNS() string {
	if identityURL := utils.GetEnv("IDENTITY_DB_URL", utils.GetEnv("DB_IDENTITY_URL", "")); identityURL != "" {
		return identityURL
	}
	identityDBName := utils.GetEnv("IDENTITY_DB_NAME", utils.GetEnv("DB_IDENTITY_NAME", ""))
	if identityDBName == "" {
		return ""
	}
	host := utils.GetEnv("IDENTITY_DB_HOST", utils.GetEnv("DB_IDENTITY_HOST", c.DB.Host))
	port := utils.GetEnv("IDENTITY_DB_PORT", utils.GetEnv("DB_IDENTITY_PORT", c.DB.Port))
	user := utils.GetEnv("IDENTITY_DB_USER", utils.GetEnv("DB_IDENTITY_USER", c.DB.User))
	password := utils.GetEnv("IDENTITY_DB_PASSWORD", utils.GetEnv("DB_IDENTITY_PASSWORD", c.DB.Password))
	sslMode := utils.GetEnv("IDENTITY_DB_SSL_MODE", utils.GetEnv("DB_IDENTITY_SSL_MODE", c.DB.SSLMode))
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", host, port, user, password, identityDBName, sslMode)
}
