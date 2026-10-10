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
	"goshop/pkg/shipping"
	"goshop/pkg/ws"
	"strconv"
)

type OrderModule struct {
	routes      []routes.Route
	adminRoutes []routes.Route
}

func NewOrderModule(
	ctx *ModuleContext,
	wsHub ws.Hub,
	rabbitmqService rabbitmq.RabbitMQService,
	notificationService v1service.NotificationService,
	locationService v1service.LocationService,
) *OrderModule {

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

	pricingService := v1service.NewOrderPricingService(couponRepo)
	couponService := v1service.NewCouponService(couponRepo, userRepo, ctx.Redis)
	stockMovementRepo := repository.NewSqlStockMovementRepository(ctx.DB)
	inventoryService := v1service.NewInventoryService(productRepo, stockMovementRepo, userRepo, ctx.TxManager, notificationService)

	outboxRepo := repository.NewSqlOutboxRepository(ctx.DB)
	customerReader := v1service.NewOrderCustomerAdapter(userRepo)

	orderService := v1service.NewOrderService(
		ctx.TxManager,
		pricingService,
		inventoryService,
		orderRepo,
		orderItemRepo,
		customerReader,
		cartService,
		couponService,
		trackingRepo,
		ctx.Redis,
		outboxRepo,
	)

	shippingService := v1service.NewOrderShippingService(
		ctx.TxManager,
		orderRepo,
		orderItemRepo,
		trackingRepo,
		paymentRepo,
		shippingFactory,
		webhookRepo,
		customerReader,
		ctx.Redis,
		rabbitmqService,
	)

	cacheService := cache.NewRedisCacheService(ctx.Redis)
	orderReader := v1service.NewOrderReaderAdapter(orderRepo)
	paymentService := v1service.NewPaymentService(
		paymentRepo,
		orderReader,
		userRepo,
		webhookRepo,
		paymentProviders,
		notificationService,
		cacheService,
		rabbitmqService,
		utils.GetEnv("PAYMENT_RETURN_URL", "http://localhost:3001/payment/result"),
		utils.GetEnv("PAYMENT_WEBHOOK_BASE_URL", "http://localhost:8080"),
		outboxRepo,
	)

	returnService := v1service.NewOrderReturnService(
		ctx.TxManager,
		orderRepo,
		orderItemRepo,
		orderReturnRepo,
		inventoryService,
		customerReader,
		trackingRepo,
		paymentService,
		outboxRepo,
		ctx.Redis,
	)

	invoiceService := v1service.NewOrderInvoiceService(orderRepo, orderItemRepo, customerReader, cacheService)
	orderAdminService := v1service.NewOrderAdminService(orderService)
	if locationService == nil {
		locationService = v1service.NewLocationService(shippingFactory, cacheService)
	}

	orderHandler := v1handler.NewOrderHandler(orderService, shippingService, returnService).
		WithInvoiceService(invoiceService).
		WithAdminService(orderAdminService).
		WithLocationService(locationService)

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
