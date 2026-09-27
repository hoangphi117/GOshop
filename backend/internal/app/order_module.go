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
	"goshop/pkg/payment"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/shipping"
	"goshop/pkg/ws"
	"strconv"
	"time"
)

type OrderModule struct {
	routes      []routes.Route
	adminRoutes []routes.Route
}

func NewOrderModule(ctx *ModuleContext, wsHub ws.Hub, rabbitmqService rabbitmq.RabbitMQService) *OrderModule {

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
	notificationService := v1service.NewNotificationService(notificationRepo, userRepo, wsHub)

	pricingService := v1service.NewOrderPricingService(couponRepo)
	stockMovementRepo := repository.NewSqlStockMovementRepository(ctx.DB)
	inventoryService := v1service.NewInventoryService(productRepo, stockMovementRepo, userRepo, ctx.TxManager, notificationService)

	orderService := v1service.NewOrderService(
		ctx.TxManager,
		pricingService,
		inventoryService,
		orderRepo,
		orderItemRepo,
		productRepo,
		userRepo,
		cartService,
		cartRepo,
		couponRepo,
		trackingRepo,
		paymentRepo,
		notificationService,
		ctx.Redis,
	)

	shippingService := v1service.NewOrderShippingService(
		ctx.TxManager,
		orderRepo,
		orderItemRepo,
		trackingRepo,
		paymentRepo,
		shippingFactory,
		webhookRepo,
		notificationService,
		userRepo,
		ctx.Redis,
		rabbitmqService,
	)

	cacheService := cache.NewRedisCacheService(ctx.Redis)
	paymentService := v1service.NewPaymentService(
		paymentRepo,
		orderRepo,
		userRepo,
		webhookRepo,
		paymentProviders,
		notificationService,
		cacheService,
		rabbitmqService,
		utils.GetEnv("PAYMENT_RETURN_URL", "http://localhost:3001/payment/result"),
		utils.GetEnv("PAYMENT_WEBHOOK_BASE_URL", "http://localhost:8080"),
	)

	returnService := v1service.NewOrderReturnService(
		ctx.TxManager,
		orderRepo,
		orderItemRepo,
		orderReturnRepo,
		productRepo,
		userRepo,
		trackingRepo,
		paymentService,
		notificationService,
		ctx.Redis,
	)

	go func() {
		err := rabbitmqService.Consume(context.Background(), v1service.QueueShippingCreate, shippingService.ProcessShippingOrderMessage)
		if err != nil {
			logger.Log.Error().Err(err).Msg("Failed to start RabbitMQ consumer for shipping.order.create")
		}
	}()

	// Background worker đồng bộ trạng thái đơn vận chuyển từ GHN định kỳ (Cronjob)
	go func() {
		intervalMinutes, _ := strconv.Atoi(utils.GetEnv("SHIPPING_SYNC_INTERVAL_MINUTES", "30"))
		if intervalMinutes <= 0 {
			intervalMinutes = 30
		}
		interval := time.Duration(intervalMinutes) * time.Minute

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		logger.Log.Info().
			Int("interval_minutes", intervalMinutes).
			Msg("Shipping tracking sync cronjob worker started")

		for range ticker.C {
			if _, err := shippingService.SyncActiveShippingOrders(context.Background()); err != nil {
				logger.Log.Error().Err(err).Msg("Error occurred during periodic shipping sync cronjob")
			}
		}
	}()

	orderHandler := v1handler.NewOrderHandler(orderService, shippingService, returnService)

	orderRoutes := v1routes.NewOrderRoutes(orderHandler)
	adminOrderRoutes := v1routes.NewAdminOrderRoutes(orderHandler)
	adminInventoryRoutes := v1routes.NewAdminInventoryRoutes(v1handler.NewInventoryHandler(inventoryService))

	shippingWebhookRoutes := v1routes.NewShippingWebhookRoutes(orderHandler)

	return &OrderModule{
		routes:      []routes.Route{orderRoutes, shippingWebhookRoutes},
		adminRoutes: []routes.Route{adminOrderRoutes, adminInventoryRoutes},
	}

}

func (m *OrderModule) Routes() []routes.Route {
	return m.routes
}

func (m *OrderModule) AdminRoutes() []routes.Route {
	return m.adminRoutes
}
