package config

import (
	"fmt"
	"goshop/internal/utils"
)

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type Config struct {
	AppEnv             string // Environment: "development", "production", "staging"
	ServerAddress      string
	DB                 DatabaseConfig
	AppURL             string
	StorefrontURL      string
	MailProviderType   string
	MailProviderConfig map[string]any
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
		AppURL:             utils.GetEnv("APP_URL", "http://localhost:8080"),
		StorefrontURL:      utils.GetEnv("STOREFRONT_URL", "http://localhost:3001"),
		MailProviderType:   mailProviderType,
		MailProviderConfig: mailProviderConfig,
	}
}

func (c *Config) DNS() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", c.DB.Host, c.DB.Port, c.DB.User, c.DB.Password, c.DB.DBName, c.DB.SSLMode)
}
