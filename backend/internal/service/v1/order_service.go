package v1service

import (
	"context"
	"goshop/internal/db"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/pkg/cache"

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
	catalogPort CatalogPort,
	stockPort OrderStockPort,
	orderRepo repository.OrderRepository,
	orderItemRepo repository.OrderItemRepository,
	userRepo OrderCustomerReader,
	cartService CartService,
	couponPort OrderCouponPort,
	trackingRepo repository.TrackingRepository,
	redisClient *redis.Client,
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
		catalogPort,
		stockPort,
		orderRepo,
		orderItemRepo,
		userRepo,
		cartService,
		couponPort,
		trackingRepo,
		cacheService,
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
		stockPort,
		cacheService,
		outboxStore,
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

// SetCheckoutService allows swapping the underlying checkout engine (e.g. Saga Orchestrator).
func (s *orderService) SetCheckoutService(cs OrderCheckoutService) {
	if cs != nil {
		s.checkoutService = cs
	}
}

// ─── CustomerOrderService implementations ────────────────────────────────────

func (s *orderService) CreateOrder(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateOrderInput) (*v1dto.OrderDTO, error) {
	return s.checkoutService.CreateOrder(ctx, userUUID, input)
}

func (s *orderService) GetMyOrders(ctx context.Context, userUUID uuid.UUID, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error) {
	return s.queryService.GetMyOrders(ctx, userUUID, input)
}

func (s *orderService) GetOrderByUUID(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (*v1dto.OrderDTO, error) {
	return s.queryService.GetOrderByUUID(ctx, userUUID, orderUUID)
}

func (s *orderService) CancelOrder(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, reason *string) (*v1dto.OrderSummaryDTO, error) {
	return s.lifecycleService.CancelOrder(ctx, userUUID, orderUUID, reason)
}

func (s *orderService) CancelExpiredOrder(ctx context.Context, orderUUID uuid.UUID) error {
	return s.lifecycleService.CancelExpiredOrder(ctx, orderUUID)
}

func (s *orderService) UpdateOrderShippingInfo(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateOrderShippingInfoInput) (*v1dto.OrderDTO, error) {
	return s.lifecycleService.UpdateOrderShippingInfo(ctx, userUUID, orderUUID, input)
}

func (s *orderService) UpdateOrderPaymentStatus(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateOrderPaymentStatusInput) (*v1dto.OrderDTO, error) {
	return s.lifecycleService.UpdateOrderPaymentStatus(ctx, userUUID, orderUUID, input)
}

// ─── OrderAdminService implementations ───────────────────────────────────────

func (s *orderService) GetAllOrders(ctx context.Context, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error) {
	return s.queryService.GetAllOrders(ctx, input)
}

func (s *orderService) UpdateOrderStatus(ctx context.Context, orderUUID uuid.UUID, input v1dto.UpdateOrderStatusInput) (*v1dto.OrderDTO, error) {
	return s.lifecycleService.UpdateOrderStatus(ctx, orderUUID, input)
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
