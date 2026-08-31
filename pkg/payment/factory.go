package payment

import (
	"fmt"
	"goshop/internal/utils"
)

type ProviderType string

const (
	ProviderVNPay ProviderType = "vnpay"
	ProviderMoMo  ProviderType = "momo"
)

// type VNPayConfig struct {
// 	TmnCode    string // Mã website VNPay cấp
// 	HashSecret string // Chuỗi bí mật tạo chữ ký
// 	BaseURL    string // https://sandbox.vnpayment.vn/paymentv2/vpcpay.html
// 	QueryURL   string // https://sandbox.vnpayment.vn/merchant_webapi/api/transaction
// }

type MoMoConfig struct {
	PartnerCode string
	AccessKey   string
	SecretKey   string
	Endpoint    string // https://test-payment.momo.vn/v2/gateway/api/create
	QueryURL    string
}

type PaymentConfig struct {
	// VNPay VNPayConfig
	MoMo MoMoConfig
}

type PaymentProviderFactory interface {
	CreateProvider() (PaymentProvider, error)
}

// type vnpayFactory struct {
// 	config VNPayConfig
// }

// func (f *vnpayFactory) CreateProvider() (PaymentProvider, error) {
// 	return NewVNPayProvider(f.config), nil
// }

type momoFactory struct {
	config MoMoConfig
}

func (f *momoFactory) CreateProvider() (PaymentProvider, error) {
	return NewMoMoProvider(f.config), nil
}

func NewProviderFactory(providerType ProviderType, cfg PaymentConfig) (PaymentProviderFactory, error) {
	switch providerType {
	// case ProviderVNPay:
	// 	return &vnpayFactory{config: cfg.VNPay}, nil
	case ProviderMoMo:
		return &momoFactory{config: cfg.MoMo}, nil
	default:
		return nil, utils.NewError(utils.ErrCodeInternal, fmt.Sprintf("Unsupported payment provider: %s", providerType))
	}
}
