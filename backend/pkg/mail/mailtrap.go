package mail

import (
	"context"
	"fmt"
	"goshop/internal/utils"
	"strconv"

	mailtrapsdk "github.com/mailtrap/mailtrap-go"

	"github.com/rs/zerolog"
)

type MailtrapConfig struct {
	MailSender     string
	MailSenderName string
	APIKey         string
	SandboxID      int64
	SandboxMode    bool
}

type MailtrapProvider struct {
	client *mailtrapsdk.Client
	config *MailtrapConfig
	logger *zerolog.Logger
}

func NewMailtrapProvider(config *MailConfig) (EmailProviderService, error) {

	mailtrapConfig, ok := config.ProviderConfig["mailtrap"].(map[string]any)
	if !ok {
		return nil, utils.NewError(utils.ErrCodeInternal, "Invalid mailtrap configuration")
	}

	apiKey, _ := mailtrapConfig["api_key"].(string)
	mailSender, _ := mailtrapConfig["mail_sender"].(string)
	mailSenderName, _ := mailtrapConfig["mail_sender_name"].(string)

	sandboxIDStr, _ := mailtrapConfig["sandbox_id"].(string)
	sandboxID, _ := strconv.ParseInt(sandboxIDStr, 10, 64)

	sandboxModeStr, _ := mailtrapConfig["sandbox_mode"].(string)
	sandboxMode := sandboxModeStr == "true"

	if apiKey == "" || mailSender == "" || mailSenderName == "" {
		return nil, utils.NewError(utils.ErrCodeInternal, "Missing required mailtrap configuration")
	}

	client, err := mailtrapsdk.NewClient(
		apiKey,
		mailtrapsdk.WithSandbox(sandboxMode),
		mailtrapsdk.WithSandboxID(sandboxID),
	)

	if err != nil {
		return nil, utils.NewError(utils.ErrCodeInternal, fmt.Sprintf("Failed to create mailtrap client: %s", err))
	}

	return &MailtrapProvider{
		client: client,
		config: &MailtrapConfig{
			MailSender:     mailSender,
			MailSenderName: mailSenderName,
			APIKey:         apiKey,
			SandboxID:      sandboxID,
			SandboxMode:    sandboxMode,
		},
		logger: config.Logger,
	}, nil
}

func (p *MailtrapProvider) SendEmail(ctx context.Context, email *Email) error {
	if len(email.To) == 0 {
		return utils.NewError(utils.ErrCodeBadRequest, "email recipient is required")
	}

	to := make([]mailtrapsdk.Address, 0, len(email.To))
	for _, addr := range email.To {
		if addr.Email == "" {
			return utils.NewError(utils.ErrCodeBadRequest, "email recipient address is required")
		}

		to = append(to, mailtrapsdk.Address{
			Email: addr.Email,
			Name:  addr.Name,
		})
	}

	from := mailtrapsdk.Address{
		Email: p.config.MailSender,
		Name:  p.config.MailSenderName,
	}

	_, _, err := p.client.Send(ctx, &mailtrapsdk.SendRequest{
		From:     from,
		To:       to,
		Subject:  email.Subject,
		Text:     email.Text,
		HTML:     email.HTML,
		Category: email.Category,
	})

	if err != nil {
		if p.logger != nil {
			p.logger.Error().Err(err).Str("to", email.To[0].Email).Msg("failed to send email via mailtrap")
		}
		return utils.WrapError(utils.ErrCodeInternal, "Failed to send email", err)
	}

	return nil
}
