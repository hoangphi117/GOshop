package app

import (
	"fmt"
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
	reader      v1service.OrderReader
}

type OrderModuleDeps struct {
	CartService     v1service.CartService
	PaymentService  v1service.PaymentService
	LocationService v1service.LocationService
	ShippingFactory *shipping.ShippingFactory
	RabbitMQService rabbitmq.RabbitMQService
	CacheService    cache.RedisCacheService
}

func NewOrderModule(
	ctx *ModuleContext,
	deps OrderModuleDeps,
) (*OrderModule, error) {
	if ctx == nil {
		return nil, fmt.Errorf("ModuleContext is required")
	}
	if ctx.CouponClient == nil {
		return nil, fmt.Errorf("coupon client is required in ModuleContext")
	}
	if ctx.InventoryClient == nil {
		return nil, fmt.Errorf("inventory client is required in ModuleContext")
	}

	orderRepo := repository.NewSqlOrderRepository(ctx.DB)
	orderItemRepo := repository.NewSqlOrderItemRepository(ctx.DB)
	orderReturnRepo := repository.NewSqlOrderReturnsRepository(ctx.DB)
	trackingRepo := repository.NewSqlTrackingRepository(ctx.DB)
	webhookRepo := repository.NewSqlWebhookEventRepository(ctx.DB)
	outboxRepo := repository.NewSqlOutboxRepository(ctx.DB)
	userReader := client.NewIdentityUserReader(ctx.IdentityClient)

	orderReader := v1service.NewInprocessOrderReader(orderRepo, orderItemRepo)
	customerReader := v1service.NewOrderCustomerAdapter(userReader)
	catalogPort := v1service.NewOrderCatalogAdapter(ctx.CatalogClient)
	couponPort := v1service.NewOrderCouponGRPCAdapter(ctx.CouponClient)
	stockPort := v1service.NewOrderStockGRPCAdapter(ctx.InventoryClient)

	pricingService := v1service.NewOrderPricingService(couponPort)

	sagaRepo := repository.NewSqlSagaRepository(ctx.DB)
	sagaOrchestrator := v1service.NewOrderSagaOrchestrator(
		sagaRepo,
		ctx.TxManager,
		pricingService,
		catalogPort,
		stockPort,
		orderRepo,
		orderItemRepo,
		customerReader,
		deps.CartService,
		couponPort,
		trackingRepo,
		deps.CacheService,
		outboxRepo,
	)
	if deps.ShippingFactory != nil {
		sagaOrchestrator = sagaOrchestrator.WithShippingValidator(v1service.NewOrderShippingValidator(deps.ShippingFactory))
	}

	orderService := v1service.NewOrderService(
		ctx.TxManager,
		pricingService,
		catalogPort,
		stockPort,
		orderRepo,
		orderItemRepo,
		customerReader,
		deps.CartService,
		couponPort,
		trackingRepo,
		ctx.Redis,
		outboxRepo,
	)
	if setter, ok := orderService.(interface{ SetCheckoutService(v1service.OrderCheckoutService) }); ok {
		setter.SetCheckoutService(sagaOrchestrator)
	}

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
		StockPort:       stockPort,
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
	shippingWebhookRoutes := v1routes.NewShippingWebhookRoutes(orderHandler)

	return &OrderModule{
		routes:      []routes.Route{orderRoutes, shippingWebhookRoutes},
		adminRoutes: []routes.Route{adminOrderRoutes},
		reader:      orderReader,
	}, nil
}

func (m *OrderModule) OrderReader() v1service.OrderReader {
	return m.reader
}

func (m *OrderModule) Routes() []routes.Route {
	return m.routes
}

func (m *OrderModule) AdminRoutes() []routes.Route {
	return m.adminRoutes
}
