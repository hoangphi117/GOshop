package app

import (
	"context"
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/pkg/logger"
	"goshop/pkg/payment"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/shipping"
	"strconv"
)

type OrderModule struct {
	routes      []routes.Route
	adminRoutes []routes.Route
}

func NewOrderModule(ctx *ModuleContext, rabbitmqService rabbitmq.RabbitMQService) *OrderModule {

	orderRepo := repository.NewSqlOrderRepository(ctx.DB)
	orderItemRepo := repository.NewSqlOrderItemRepository(ctx.DB)
	orderReturnRepo := repository.NewSqlOrderReturnsRepository(ctx.DB)
	trackingRepo := repository.NewSqlTrackingRepository(ctx.DB)

	// Giao Hàng Nhanh (GHN) Adapter
	ghnToken := utils.GetEnv("GHN_TOKEN", "")
	ghnShopID, _ := strconv.Atoi(utils.GetEnv("GHN_SHOP_ID", "0"))
	ghnBaseURL := utils.GetEnv("GHN_BASE_URL", "")
	ghnAdapter := shipping.NewGHNAdapter(ghnToken, ghnShopID, ghnBaseURL)
	shippingFactory := shipping.NewShippingFactory(ghnAdapter)

	productRepo := repository.NewSqlProductRepository(ctx.DB)
	userRepo := repository.NewSqlUserRepository(ctx.DB)

	cartRepo := repository.NewSqlCartRepository(ctx.DB)
	cartService := v1service.NewCartService(cartRepo, userRepo, productRepo, ctx.Redis)

	couponRepo := repository.NewSqlCouponRepository(ctx.DB)
	webhookRepo := repository.NewSqlWebhookEventRepository(ctx.DB)

	paymentRepo := repository.NewSqlPaymentRepository(ctx.DB)
	paymentConfig := payment.PaymentConfig{
		MoMo: payment.MoMoConfig{
			PartnerCode: utils.GetEnv("MOMO_PARTNER_CODE", ""),
			AccessKey:   utils.GetEnv("MOMO_ACCESS_KEY", ""),
			SecretKey:   utils.GetEnv("MOMO_SECRET_KEY", ""),
			Endpoint:    utils.GetEnv("MOMO_ENDPOINT", ""),
			QueryURL:    utils.GetEnv("MOMO_QUERY_URL", ""),
		},
	}

	paymentProviders := make(map[string]payment.PaymentProvider)
	if momoFactory, err := payment.NewProviderFactory(payment.ProviderMoMo, paymentConfig); err == nil {
		if p, err := momoFactory.CreateProvider(); err == nil {
			paymentProviders["momo"] = p
		}
	}

	notificationRepo := repository.NewSqlNotificationRepository(ctx.DB)
	notificationService := v1service.NewNotificationService(notificationRepo, userRepo)

	orderService := v1service.NewOrderService(orderRepo, orderItemRepo, orderReturnRepo, productRepo, userRepo,
		cartService, cartRepo, couponRepo, trackingRepo, webhookRepo,
		paymentRepo, paymentProviders, notificationService, ctx.Redis, shippingFactory, rabbitmqService)

	go func() {
		err := rabbitmqService.Consume(context.Background(), v1service.QueueShippingCreate, orderService.ProcessShippingOrderMessage)
		if err != nil {
			logger.Log.Error().Err(err).Msg("Failed to start RabbitMQ consumer for shipping.order.create")
		}
	}()

	orderHandler := v1handler.NewOrderHandler(orderService)

	orderRoutes := v1routes.NewOrderRoutes(orderHandler)
	adminOrderRoutes := v1routes.NewAdminOrderRoutes(orderHandler)

	shippingWebhookRoutes := v1routes.NewShippingWebhookRoutes(orderHandler)

	return &OrderModule{
		routes:      []routes.Route{orderRoutes, shippingWebhookRoutes},
		adminRoutes: []routes.Route{adminOrderRoutes},
	}

}

func (m *OrderModule) Routes() []routes.Route {
	return m.routes
}

func (m *OrderModule) AdminRoutes() []routes.Route {
	return m.adminRoutes
}
