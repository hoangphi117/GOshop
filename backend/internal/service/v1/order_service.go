package v1service

import (
	"context"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/pkg/cache"
	"goshop/pkg/rabbitmq"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	OrderStatusPending   int32 = 1
	OrderStatusConfirmed int32 = 2
	OrderStatusShipped   int32 = 3
	OrderStatusDelivered int32 = 4
	OrderStatusCancelled int32 = 5
	OrderStatusReturned  int32 = 6
	OrderStatusRefunded  int32 = 6

	OrderPaymentStatusUnpaid            int32 = 1
	OrderPaymentStatusPaid              int32 = 2
	OrderPaymentStatusPartiallyRefunded int32 = 3
	OrderPaymentStatusFullyRefunded     int32 = 4
	OrderPaymentMethodCOD               int32 = 1
	OrderPaymentMethodOnline            int32 = 2
	OrderPaymentMethodEWallet           int32 = 3

	OrderReturnStatusPending   int16 = 1
	OrderReturnStatusApproved  int16 = 2
	OrderReturnStatusRejected  int16 = 3
	OrderReturnStatusCompleted int16 = 4
	OrderReturnStatusCancelled int16 = 5

	RoleAdmin     int32 = 1
	RoleModerator int32 = 2
	RoleMember    int32 = 3

	CouponTypePercentage  int32 = 1
	CouponTypeFixedAmount int32 = 2

	QueueShippingCreate = "shipping.order.create"
)

// orderService is a Strangler Fig Facade composing focused use-case services:
// - OrderCheckoutService (Creation & checkout flow)
// - OrderQueryService (Read models, search, invoice)
// - OrderLifecycleService (Status updates, cancel, returns, payment adjustments)
type orderService struct {
	checkoutService  OrderCheckoutService
	queryService     OrderQueryService
	lifecycleService OrderLifecycleService
	cache            cache.RedisCacheService
	invoiceService   OrderInvoiceService
	adminService     OrderAdminService
}

func NewOrderService(
	txManager db.TxManager,
	pricingService OrderPricingService,
	inventoryService InventoryService,
	orderRepo repository.OrderRepository,
	orderItemRepo repository.OrderItemRepository,
	userRepo repository.UserRepository,
	cartService CartService,
	couponService CouponService,
	trackingRepo repository.TrackingRepository,
	notificationService NotificationService,
	redisClient *redis.Client,
	rabbitMQ rabbitmq.RabbitMQService,
	outboxRepo ...repository.OutboxRepository,
) OrderService {
	var cacheService cache.RedisCacheService
	if redisClient != nil {
		cacheService = cache.NewRedisCacheService(redisClient)
	}

	invoiceService := NewOrderInvoiceService(orderRepo, orderItemRepo, userRepo, cacheService)

	var outboxStore OrderOutboxPort
	if len(outboxRepo) > 0 {
		outboxStore = outboxRepo[0]
	}

	checkoutSvc := NewOrderCheckoutService(
		txManager,
		pricingService,
		inventoryService,
		orderRepo,
		orderItemRepo,
		userRepo,
		cartService,
		couponService,
		trackingRepo,
		notificationService,
		cacheService,
		rabbitMQ,
		outboxStore,
	)

	querySvc := NewOrderQueryService(
		orderRepo,
		orderItemRepo,
		userRepo,
		cacheService,
		invoiceService,
	)

	lifecycleSvc := NewOrderLifecycleService(
		txManager,
		orderRepo,
		orderItemRepo,
		trackingRepo,
		userRepo,
		inventoryService,
		notificationService,
		cacheService,
	)

	svc := &orderService{
		checkoutService:  checkoutSvc,
		queryService:     querySvc,
		lifecycleService: lifecycleSvc,
		cache:            cacheService,
		invoiceService:   invoiceService,
	}
	svc.adminService = NewOrderAdminService(svc)
	return svc
}

// ─── CustomerOrderService implementations ────────────────────────────────────

func (s *orderService) CreateOrder(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateOrderInput) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {
	return s.checkoutService.CreateOrder(ctx, userUUID, input)
}

func (s *orderService) GetMyOrders(ctx context.Context, userUUID uuid.UUID, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error) {
	return s.queryService.GetMyOrders(ctx, userUUID, input)
}

func (s *orderService) GetOrderByUUID(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (sqlc.GetOrderByUUIDRow, []sqlc.GetOrderItemsByOrderIDRow, error) {
	return s.queryService.GetOrderByUUID(ctx, userUUID, orderUUID)
}

func (s *orderService) CancelOrder(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, reason *string) (sqlc.Order, error) {
	return s.lifecycleService.CancelOrder(ctx, userUUID, orderUUID, reason)
}

func (s *orderService) CancelExpiredOrder(ctx context.Context, orderUUID uuid.UUID) error {
	return s.lifecycleService.CancelExpiredOrder(ctx, orderUUID)
}

func (s *orderService) UpdateOrderShippingInfo(ctx context.Context, userUUID uuid.UUID, input sqlc.UpdateOrderShippingInfoParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {
	return s.lifecycleService.UpdateOrderShippingInfo(ctx, userUUID, input)
}

func (s *orderService) UpdateOrderPaymentStatus(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateOrderPaymentStatusInput) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {
	return s.lifecycleService.UpdateOrderPaymentStatus(ctx, userUUID, orderUUID, input)
}

// ─── OrderAdminService implementations ───────────────────────────────────────

func (s *orderService) GetAllOrders(ctx context.Context, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error) {
	return s.queryService.GetAllOrders(ctx, input)
}

func (s *orderService) UpdateOrderStatus(ctx context.Context, input sqlc.UpdateOrderStatusParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {
	return s.lifecycleService.UpdateOrderStatus(ctx, input)
}

func (s *orderService) BulkUpdateStatus(ctx context.Context, input v1dto.BulkUpdateOrderStatusInput) (*v1dto.BulkOperationResultDTO, error) {
	return s.lifecycleService.BulkUpdateStatus(ctx, input)
}

func (s *orderService) BulkCancel(ctx context.Context, userUUID uuid.UUID, input v1dto.BulkCancelOrdersInput) (*v1dto.BulkOperationResultDTO, error) {
	return s.lifecycleService.BulkCancel(ctx, userUUID, input)
}

// ─── OrderInvoiceService implementations ─────────────────────────────────────

func (s *orderService) GetOrderInvoicePDF(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) ([]byte, string, error) {
	return s.queryService.GetOrderInvoicePDF(ctx, userUUID, orderUUID)
}
