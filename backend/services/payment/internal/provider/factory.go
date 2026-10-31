package provider

import (
	"fmt"
	"goshop/pkg/utils"
)

type ProviderType string

const (
	ProviderMoMo  ProviderType = "momo"
	ProviderVNPay ProviderType = "vnpay"
)

type MoMoConfig struct {
	PartnerCode string
	AccessKey   string
	SecretKey   string
	Endpoint    string // https://test-payment.momo.vn/v2/gateway/api/create
	QueryURL    string
}

type VNPayConfig struct {
	TmnCode    string
	HashSecret string
	Endpoint   string // https://sandbox.vnpayment.vn/paymentv2/vpcpay.html
	QueryURL   string // https://sandbox.vnpayment.vn/merchant_webapi/api/transaction
}

type PaymentConfig struct {
	MoMo  MoMoConfig
	VNPay VNPayConfig
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

type vnpayFactory struct {
	config VNPayConfig
}

func (f *vnpayFactory) CreateProvider() (PaymentProvider, error) {
	raw := NewVNPayProvider(f.config)
	return NewCircuitBreakerPaymentProvider(raw, "vnpay", DefaultCBConfig()), nil
}

func NewProviderFactory(providerType ProviderType, cfg PaymentConfig) (PaymentProviderFactory, error) {
	switch providerType {
	case ProviderMoMo:
		return &momoFactory{config: cfg.MoMo}, nil
	case ProviderVNPay:
		return &vnpayFactory{config: cfg.VNPay}, nil
	default:
		return nil, utils.NewError(utils.ErrCodeInternal, fmt.Sprintf("Unsupported payment provider: %s", providerType))
	}
}
