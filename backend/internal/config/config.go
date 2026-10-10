package config

import (
	"fmt"
	"goshop/internal/utils"
	"goshop/pkg/storage"
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
		Driver:        storage.StorageDriver(utils.GetEnv("STORAGE_DRIVER", "local")),
		UploadDir:     utils.GetEnv("UPLOAD_DIR", "./uploads"),
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
	return utils.GetEnv("DB_IDENTITY_URL", "")
}

