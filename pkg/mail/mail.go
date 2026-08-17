package mail

import (
	"context"
	"goshop/internal/config"
	"goshop/internal/utils"
	"goshop/pkg/logger"
	"time"

	"github.com/rs/zerolog"
)

type Email struct {
	From     Address   `json:"from"`
	To       []Address `json:"to"`
	Subject  string    `json:"subject"`
	Text     string    `json:"text"`
	HTML     string    `json:"html,omitempty"`
	Category string    `json:"category"`
}

type Address struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type MailConfig struct {
	ProviderConfig map[string]any
	ProviderType   string
	MaxRetries     int
	Timeout        time.Duration
	Logger         *zerolog.Logger
}

type MailService struct {
	provider EmailProviderService
	config   *MailConfig
	logger   *zerolog.Logger
}

func NewMailService(cfg *config.Config, logger *zerolog.Logger, providerFactory ProviderFactory) (EmailProviderService, error) {

	mailConfig := &MailConfig{
		ProviderType:   cfg.MailProviderType,
		ProviderConfig: cfg.MailProviderConfig,
		MaxRetries:     3,
		Timeout:        10 * time.Second,
		Logger:         logger,
	}

	provider, err := providerFactory.CreateProvider(mailConfig)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to create email provider", err)
	}

	return &MailService{
		provider: provider,
		config:   mailConfig,
		logger:   logger,
	}, nil
}

func (ms *MailService) SendEmail(ctx context.Context, email *Email) error {

	traceID := logger.GetTraceId(ctx)
	start := time.Now()

	var lastErr error

	maxRetries := 3
	if ms.config.MaxRetries > 0 {
		maxRetries = ms.config.MaxRetries
	}
	totalAttempts := maxRetries + 1

	for attempt := 1; attempt <= totalAttempts; attempt++ {
		startAttempt := time.Now()
		err := ms.provider.SendEmail(ctx, email)
		if err == nil {
			ms.logger.Info().Str("trace_id", traceID).
				Dur("duration", time.Since(startAttempt)).
				Str("operation", "send_mail").
				Interface("to", email.To).
				Str("subject", email.Subject).
				Str("category", email.Category).
				Int("attempt", attempt).
				Msg("Email send successfully")

			return nil
		}

		lastErr = err
		if attempt == totalAttempts {
			break
		}

		delay := time.Duration(attempt) * time.Second
		ms.logger.Warn().Str("trace_id", traceID).
			Dur("duration", time.Since(startAttempt)).
			Str("operation", "send_mail").
			Int("attempt", attempt).
			Int("max_retries", maxRetries).
			Dur("retry_in", delay).
			Err(err).
			Msg("Failed to send email, retrying...")

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	ms.logger.Error().Str("trace_id", traceID).
		Dur("duration", time.Since(start)).
		Str("operation", "send_mail").
		Int("attempts", totalAttempts).
		Err(lastErr).
		Msg("Failed to send email after all retries")

	return utils.WrapError(utils.ErrCodeInternal, "Failed to send email after all retries", lastErr)
}
