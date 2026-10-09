package app

import (
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/payment"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/ws"
)

type PaymentModule struct {
	routes      []routes.Route
	adminRoutes []routes.Route
}

func NewPaymentModule(
	ctx *ModuleContext,
	cacheService cache.RedisCacheService,
	rabbitmqService rabbitmq.RabbitMQService,
	wsHub ws.Hub,
	notificationService v1service.NotificationService,
) *PaymentModule {
	paymentRepo := repository.NewSqlPaymentRepository(ctx.DB)
	webhookRepo := repository.NewSqlWebhookEventRepository(ctx.DB)
	orderRepo := repository.NewSqlOrderRepository(ctx.DB)
	userRepo := repository.NewSqlUserRepository(ctx.DB)

	paymentConfig := payment.PaymentConfig{
		MoMo: payment.MoMoConfig{
			PartnerCode: utils.GetEnv("MOMO_PARTNER_CODE", ""),
			AccessKey:   utils.GetEnv("MOMO_ACCESS_KEY", ""),
			SecretKey:   utils.GetEnv("MOMO_SECRET_KEY", ""),
			Endpoint:    utils.GetEnv("MOMO_ENDPOINT", ""),
			QueryURL:    utils.GetEnv("MOMO_QUERY_URL", ""),
		},
	}

	providers := make(map[string]payment.PaymentProvider)
	if momoFactory, err := payment.NewProviderFactory(payment.ProviderMoMo, paymentConfig); err == nil {
		if p, err := momoFactory.CreateProvider(); err == nil {
			providers["momo"] = p
		}
	}

	returnURL := utils.GetEnv("PAYMENT_RETURN_URL", "http://localhost:3001/payment/result")
	webhookBaseURL := utils.GetEnv("PAYMENT_WEBHOOK_BASE_URL", "http://localhost:8080")

	paymentService := v1service.NewPaymentService(
		paymentRepo, orderRepo, userRepo, webhookRepo,
		providers, notificationService, cacheService, rabbitmqService, returnURL, webhookBaseURL,
	)

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
