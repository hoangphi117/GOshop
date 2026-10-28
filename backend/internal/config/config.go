package config

import (
	"fmt"
	"goshop/pkg/logger"
	"goshop/pkg/storage"
	"goshop/pkg/utils"
	"strconv"
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
}

func NewConfig() *Config {

	mailProviderConfig := make(map[string]any)

	mailProviderType := utils.GetEnv("MAIL_PROVIDER_TYPE", "mailtrap")
	if mailProviderType == "mailtrap" {

		sandboxID := utils.GetEnv("MAILTRAP_SANDBOX_ID", "0")
		sandboxMode := utils.GetEnv("MAILTRAP_USE_SANDBOX", "true")

		mailTrapConfig := map[string]any{
			"api_key":          utils.GetEnv("MAILTRAP_API_KEY", "GOshop Admin"),
			"mail_sender":      utils.GetEnv("MAILTRAP_MAIL_SENDER", "admin@goshop.com"),
			"mail_sender_name": utils.GetEnv("MAILTRAP_MAIL_SENDER_NAME", "Admin"),
			"sandbox_id":       sandboxID,
			"sandbox_mode":     sandboxMode,
		}

		mailProviderConfig["mailtrap"] = mailTrapConfig
	}

	usePathStyle, _ := strconv.ParseBool(utils.GetEnv("S3_USE_PATH_STYLE", "true"))
	useSSL, _ := strconv.ParseBool(utils.GetEnv("S3_USE_SSL", "false"))

	storageConfig := storage.StorageConfig{
		Driver:        storage.StorageDriver(utils.GetEnv("STORAGE_DRIVER", "minio")),
		PublicBaseURL: utils.GetEnv("STORAGE_PUBLIC_BASE_URL", ""),
		S3: storage.S3Config{
			Endpoint:      utils.GetEnv("S3_ENDPOINT", ""),
			Region:        utils.GetEnv("S3_REGION", "auto"),
			Bucket:        utils.GetEnv("S3_BUCKET", "goshop-media"),
			AccessKey:     utils.GetEnv("S3_ACCESS_KEY", ""),
			SecretKey:     utils.GetEnv("S3_SECRET_KEY", ""),
			PublicBaseURL: utils.GetEnv("S3_PUBLIC_BASE_URL", ""),
			UsePathStyle:  usePathStyle,
			ForceSSL:      useSSL,
		},
	}

	return &Config{
		AppEnv:        utils.GetEnv("APP_ENV", "development"),
		ServerAddress: fmt.Sprintf(":%s", utils.GetEnv("SERVER_PORT", "8080")),
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
		StorefrontURL:      utils.GetEnv("STOREFRONT_URL", "http://localhost:3001"),
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
	}
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
