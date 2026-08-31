package app

import (
	"context"
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/mail"
	"goshop/pkg/payment"
	"goshop/pkg/rabbitmq"
)

type PaymentModule struct {
	routes      []routes.Route
	adminRoutes []routes.Route
}

func NewPaymentModule(ctx *ModuleContext, cacheService cache.RedisCacheService, mailService mail.EmailProviderService, rabbitmqService rabbitmq.RabbitMQService) *PaymentModule {
	paymentRepo := repository.NewSqlPaymentRepository(ctx.DB)
	webhookRepo := repository.NewSqlWebhookEventRepository(ctx.DB)
	orderRepo := repository.NewSqlOrderRepository(ctx.DB)
	userRepo := repository.NewSqlUserRepository(ctx.DB)

	paymentConfig := payment.PaymentConfig{
		// VNPay: payment.VNPayConfig{
		// 	TmnCode:    utils.GetEnv("VNPAY_TMN_CODE", ""),
		// 	HashSecret: utils.GetEnv("VNPAY_HASH_SECRET", ""),
		// 	BaseURL:    utils.GetEnv("VNPAY_BASE_URL", ""),
		// 	QueryURL:   utils.GetEnv("VNPAY_QUERY_URL", ""),
		// },
		MoMo: payment.MoMoConfig{
			PartnerCode: utils.GetEnv("MOMO_PARTNER_CODE", ""),
			AccessKey:   utils.GetEnv("MOMO_ACCESS_KEY", ""),
			SecretKey:   utils.GetEnv("MOMO_SECRET_KEY", ""),
			Endpoint:    utils.GetEnv("MOMO_ENDPOINT", ""),
			QueryURL:    utils.GetEnv("MOMO_QUERY_URL", ""),
		},
	}

	providers := make(map[string]payment.PaymentProvider)
	// if vnpayFactory, err := payment.NewProviderFactory(payment.ProviderVNPay, paymentConfig); err == nil {
	// 	if p, err := vnpayFactory.CreateProvider(); err == nil {
	// 		providers["vnpay"] = p
	// 	}
	// }
	if momoFactory, err := payment.NewProviderFactory(payment.ProviderMoMo, paymentConfig); err == nil {
		if p, err := momoFactory.CreateProvider(); err == nil {
			providers["momo"] = p
		}
	}

	returnURL := utils.GetEnv("PAYMENT_RETURN_URL", "http://localhost:3001")
	webhookBaseURL := utils.GetEnv("PAYMENT_WEBHOOK_BASE_URL", "http://localhost:8080")

	paymentService := v1service.NewPaymentService(
		paymentRepo, orderRepo, userRepo, webhookRepo,
		providers, cacheService, rabbitmqService, returnURL, webhookBaseURL,
	)

	go func() {
		err := rabbitmqService.Consume(context.Background(), "payment.webhook", paymentService.ProcessWebhookMessage)
		if err != nil {
			logger.Log.Error().Err(err).Msg("Failed to start RabbitMQ consumer for payment.webhook")
		}
	}()

	paymentHandler := v1handler.NewPaymentHandler(paymentService)
	paymentRoutes := v1routes.NewPaymentRoutes(paymentHandler)
	paymentWebhookRoutes := v1routes.NewPaymentWebhookRoutes(paymentHandler)
	adminPaymentRoutes := v1routes.NewAdminPaymentRoutes(paymentHandler)

	return &PaymentModule{
		routes:      []routes.Route{paymentRoutes, paymentWebhookRoutes},
		adminRoutes: []routes.Route{adminPaymentRoutes},
	}
}

func (m *PaymentModule) Routes() []routes.Route {
	return m.routes
}

func (m *PaymentModule) AdminRoutes() []routes.Route {
	return m.adminRoutes
}
