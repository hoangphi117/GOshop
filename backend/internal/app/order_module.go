package app

import (
	"goshop/internal/client"
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/repository"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	v1service "goshop/internal/service/v1"
	"goshop/pkg/cache"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/shipping"
)

type OrderModule struct {
	routes      []routes.Route
	adminRoutes []routes.Route
}

type OrderModuleDeps struct {
	CartService      v1service.CartService
	CouponService    v1service.CouponService
	InventoryService v1service.InventoryService
	PaymentService   v1service.PaymentService
	LocationService  v1service.LocationService
	ShippingFactory  *shipping.ShippingFactory
	RabbitMQService  rabbitmq.RabbitMQService
	CacheService     cache.RedisCacheService
}

func NewOrderModule(
	ctx *ModuleContext,
	deps OrderModuleDeps,
) *OrderModule {
	orderRepo := repository.NewSqlOrderRepository(ctx.DB)
	orderItemRepo := repository.NewSqlOrderItemRepository(ctx.DB)
	orderReturnRepo := repository.NewSqlOrderReturnsRepository(ctx.DB)
	trackingRepo := repository.NewSqlTrackingRepository(ctx.DB)
	webhookRepo := repository.NewSqlWebhookEventRepository(ctx.DB)
	outboxRepo := repository.NewSqlOutboxRepository(ctx.DB)
	couponRepo := repository.NewSqlCouponRepository(ctx.DB)
	userRepo := repository.NewSqlUserRepository(ctx.DB)
	userReader := client.NewIdentityUserReader(ctx.IdentityClient, userRepo)

	customerReader := v1service.NewOrderCustomerAdapter(userReader)
	pricingService := v1service.NewOrderPricingService(couponRepo)

	orderService := v1service.NewOrderService(
		ctx.TxManager,
		pricingService,
		deps.InventoryService,
		orderRepo,
		orderItemRepo,
		customerReader,
		deps.CartService,
		deps.CouponService,
		trackingRepo,
		ctx.Redis,
		outboxRepo,
	)

	shippingService := v1service.NewOrderShippingService(v1service.OrderShippingServiceDeps{
		TxManager:       ctx.TxManager,
		OrderRepo:       orderRepo,
		OrderItemRepo:   orderItemRepo,
		TrackingRepo:    trackingRepo,
		ShippingFactory: deps.ShippingFactory,
		WebhookRepo:     webhookRepo,
		UserRepo:        customerReader,
		CacheService:    deps.CacheService,
		RedisClient:     ctx.Redis,
		RabbitMQ:        deps.RabbitMQService,
	})

	returnService := v1service.NewOrderReturnService(v1service.OrderReturnServiceDeps{
		TxManager:       ctx.TxManager,
		OrderRepo:       orderRepo,
		OrderItemRepo:   orderItemRepo,
		OrderReturnRepo: orderReturnRepo,
		StockPort:       deps.InventoryService,
		UserRepo:        customerReader,
		TrackingRepo:    trackingRepo,
		PaymentService:  deps.PaymentService,
		OutboxStore:     outboxRepo,
		CacheService:    deps.CacheService,
		RedisClient:     ctx.Redis,
	})

	invoiceService := v1service.NewOrderInvoiceService(orderRepo, orderItemRepo, customerReader, deps.CacheService)
	orderAdminService := v1service.NewOrderAdminService(orderService)

	locationService := deps.LocationService
	if locationService == nil && deps.ShippingFactory != nil {
		locationService = v1service.NewLocationService(deps.ShippingFactory, deps.CacheService)
	}

	orderHandler := v1handler.NewOrderHandler(orderService, shippingService, returnService).
		WithInvoiceService(invoiceService).
		WithAdminService(orderAdminService).
		WithLocationService(locationService)

	orderRoutes := v1routes.NewOrderRoutes(orderHandler)
	adminOrderRoutes := v1routes.NewAdminOrderRoutes(orderHandler)
	adminInventoryRoutes := v1routes.NewAdminInventoryRoutes(v1handler.NewInventoryHandler(deps.InventoryService))
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
