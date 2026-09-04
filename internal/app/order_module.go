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

	orderService := v1service.NewOrderService(orderRepo, orderItemRepo, productRepo, userRepo,
		cartService, cartRepo, couponRepo, trackingRepo, ctx.Redis, shippingFactory, rabbitmqService)

	go func() {
		err := rabbitmqService.Consume(context.Background(), v1service.QueueShippingCreate, orderService.ProcessShippingOrderMessage)
		if err != nil {
			logger.Log.Error().Err(err).Msg("Failed to start RabbitMQ consumer for shipping.order.create")
		}
	}()

	orderHandler := v1handler.NewOrderHandler(orderService)

	orderRoutes := v1routes.NewOrderRoutes(orderHandler)
	adminOrderRoutes := v1routes.NewAdminOrderRoutes(orderHandler)

	return &OrderModule{
		routes:      []routes.Route{orderRoutes},
		adminRoutes: []routes.Route{adminOrderRoutes},
	}

}

func (m *OrderModule) Routes() []routes.Route {
	return m.routes
}

func (m *OrderModule) AdminRoutes() []routes.Route {
	return m.adminRoutes
}
