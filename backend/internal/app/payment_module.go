package app

import (
	"context"
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/worker"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/payment"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/ws"
	"time"
)

type PaymentModule struct {
	routes      []routes.Route
	adminRoutes []routes.Route
}

func NewPaymentModule(ctx *ModuleContext, cacheService cache.RedisCacheService, rabbitmqService rabbitmq.RabbitMQService, wsHub ws.Hub) *PaymentModule {
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

	notificationRepo := repository.NewSqlNotificationRepository(ctx.DB)
	notificationService := v1service.NewNotificationService(notificationRepo, userRepo, wsHub)

	orderItemRepo := repository.NewSqlOrderItemRepository(ctx.DB)
	trackingRepo := repository.NewSqlTrackingRepository(ctx.DB)
	productRepo := repository.NewSqlProductRepository(ctx.DB)
	stockMovementRepo := repository.NewSqlStockMovementRepository(ctx.DB)
	inventoryService := v1service.NewInventoryService(productRepo, stockMovementRepo, userRepo, ctx.TxManager, notificationService)

	paymentService := v1service.NewPaymentService(
		paymentRepo, orderRepo, userRepo, webhookRepo,
		providers, notificationService, cacheService, rabbitmqService, returnURL, webhookBaseURL,
		v1service.PaymentOrderDependencies{
			InventoryService: inventoryService,
			OrderItemRepo:    orderItemRepo,
			TrackingRepo:     trackingRepo,
		},
	)

	paymentConsumer := worker.NewPaymentConsumer(paymentService, logger.Log)
	go func() {
		err := rabbitmqService.Consume(context.Background(), "payment.webhook", paymentConsumer.Handle)
		if err != nil {
			logger.Log.Error().Err(err).Msg("Failed to start RabbitMQ consumer for payment.webhook")
		}
	}()

	paymentHandler := v1handler.NewPaymentHandler(paymentService)
	paymentRoutes := v1routes.NewPaymentRoutes(paymentHandler)
	paymentWebhookRoutes := v1routes.NewPaymentWebhookRoutes(paymentHandler)
	adminPaymentRoutes := v1routes.NewAdminPaymentRoutes(paymentHandler)

	// Background worker đối soát giao dịch định kỳ 5 phút
	go func() {
		ticker := time.NewTicker(5 * time.Minute) // 5 phút
		defer ticker.Stop()

		logger.Log.Info().Msg("Payment reconciliation background worker started (interval: 5m)")

		for range ticker.C {
			if err := paymentService.ReconcilePendingPayments(context.Background()); err != nil {
				logger.Log.Error().Err(err).Msg("Error occurred during periodic payment reconciliation")
			}
		}
	}()

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
