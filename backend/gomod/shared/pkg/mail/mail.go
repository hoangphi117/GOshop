package mail

import (
	"context"
	"goshop/shared/config"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/utils"
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

	// Only an error that happens *before* the message reaches the server is
	// safe to retry. A failure after DATA means the message may already be
	// delivered, and retrying it would send a duplicate OTP or reset link.
	// Providers therefore layer retries internally for transient connect
	// errors and let everything else surface here.
	err := ms.provider.SendEmail(ctx, email)
	if err == nil {
		ms.logger.Info().Str("trace_id", traceID).
			Dur("duration", time.Since(start)).
			Str("operation", "send_mail").
			Interface("to", email.To).
			Str("subject", email.Subject).
			Str("category", email.Category).
			Msg("Email sent successfully")
		return nil
	}

	ms.logger.Error().Str("trace_id", traceID).
		Dur("duration", time.Since(start)).
		Str("operation", "send_mail").
		Err(err).
		Msg("Failed to send email")

	return utils.WrapError(utils.ErrCodeInternal, "Failed to send email", err)
}
