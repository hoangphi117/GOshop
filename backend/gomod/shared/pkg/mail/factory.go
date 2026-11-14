package mail

import (
	"fmt"

	"goshop/shared/pkg/utils"
)

type ProviderType string

const (
	ProviderSMTP ProviderType = "smtp"
)

type ProviderFactory interface {
	CreateProvider(config *MailConfig) (EmailProviderService, error)
}

type smtpProviderFactory struct{}

func (f *smtpProviderFactory) CreateProvider(config *MailConfig) (EmailProviderService, error) {
	return NewSMTPProvider(config)
}

// NewProviderFactory maps MAIL_PROVIDER_TYPE to a concrete provider. An unknown
// value is a startup error rather than a silent fallback: a mail provider that
// is not the one configured would only surface when a user never gets the OTP.
func NewProviderFactory(providerType ProviderType) (ProviderFactory, error) {
	switch providerType {
	case ProviderSMTP:
		return &smtpProviderFactory{}, nil
	default:
		return nil, utils.NewError(utils.ErrCodeInternal, fmt.Sprintf("Unsupported provider type: %s", providerType))
	}
}

// ProviderTypeFromConfig turns the raw env value into a ProviderType, applying
// the same default as config.NewConfig so an empty value never fails startup.
func ProviderTypeFromConfig(raw string) ProviderType {
	if raw == "" {
		return ProviderSMTP
	}
	return ProviderType(raw)
}
