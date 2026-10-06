package payment

import (
	"fmt"
	"goshop/internal/utils"
)

type ProviderType string

const (
	ProviderMoMo ProviderType = "momo"
)

type MoMoConfig struct {
	PartnerCode string
	AccessKey   string
	SecretKey   string
	Endpoint    string // https://test-payment.momo.vn/v2/gateway/api/create
	QueryURL    string
}

type PaymentConfig struct {
	MoMo MoMoConfig
}

type PaymentProviderFactory interface {
	CreateProvider() (PaymentProvider, error)
}

type momoFactory struct {
	config MoMoConfig
}

func (f *momoFactory) CreateProvider() (PaymentProvider, error) {
	raw := NewMoMoProvider(f.config)
	return NewCircuitBreakerPaymentProvider(raw, "momo", DefaultCBConfig()), nil
}

func NewProviderFactory(providerType ProviderType, cfg PaymentConfig) (PaymentProviderFactory, error) {
	switch providerType {
	case ProviderMoMo:
		return &momoFactory{config: cfg.MoMo}, nil
	default:
		return nil, utils.NewError(utils.ErrCodeInternal, fmt.Sprintf("Unsupported payment provider: %s", providerType))
	}
}
