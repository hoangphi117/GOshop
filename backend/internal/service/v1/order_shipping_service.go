package v1service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/shipping"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

type orderShippingService struct {
	txManager       db.TxManager
	orderRepo       OrderStorePort
	orderItemRepo   OrderItemStorePort
	trackingRepo    OrderTrackingPort
	paymentRepo     ShippingPaymentPort
	shippingFactory *shipping.ShippingFactory
	webhookRepo     ShippingWebhookPort
	userRepo        OrderCustomerReader
	cache           cache.RedisCacheService
	rabbitmq        rabbitmq.RabbitMQService
	locationService LocationService
}

// OrderShippingServiceDeps groups the dependencies required by OrderShippingService.
type OrderShippingServiceDeps struct {
	TxManager       db.TxManager
	OrderRepo       OrderStorePort
	OrderItemRepo   OrderItemStorePort
	TrackingRepo    OrderTrackingPort
	PaymentRepo     ShippingPaymentPort
	ShippingFactory *shipping.ShippingFactory
	WebhookRepo     ShippingWebhookPort
	UserRepo        OrderCustomerReader
	RedisClient     *redis.Client
	CacheService    cache.RedisCacheService
	RabbitMQ        rabbitmq.RabbitMQService
}

func NewOrderShippingService(deps OrderShippingServiceDeps) OrderShippingService {
	cacheService := deps.CacheService
	if cacheService == nil && deps.RedisClient != nil {
		cacheService = cache.NewRedisCacheService(deps.RedisClient)
	}
	return &orderShippingService{
		txManager:       deps.TxManager,
		orderRepo:       deps.OrderRepo,
		orderItemRepo:   deps.OrderItemRepo,
		trackingRepo:    deps.TrackingRepo,
		paymentRepo:     deps.PaymentRepo,
		shippingFactory: deps.ShippingFactory,
		webhookRepo:     deps.WebhookRepo,
		userRepo:        deps.UserRepo,
		cache:           cacheService,
		rabbitmq:        deps.RabbitMQ,
		locationService: NewLocationService(deps.ShippingFactory, cacheService),
	}
}

func (s *orderShippingService) handleUserNotFoundError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return utils.NewError(utils.ErrCodeNotFound, "User not found")
	}
	return utils.WrapError(utils.ErrCodeInternal, "Failed to get user", err)
}

func (s *orderShippingService) getCustomer(ctx context.Context, userUUID uuid.UUID) (*OrderCustomer, error) {
	if cust, ok := GetOrderCustomerFromContext(ctx); ok && cust.UserUUID == userUUID && cust.UserID > 0 {
		return cust, nil
	}
	if s.userRepo == nil {
		return nil, utils.NewError(utils.ErrCodeNotFound, "User not found")
	}
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		return nil, s.handleUserNotFoundError(err)
	}
	return user, nil
}

func (s *orderShippingService) handleOrderNotFoundError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return utils.NewError(utils.ErrCodeNotFound, "Order not found")
	}
	return utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
}

func (s *orderShippingService) CreateShipping(ctx context.Context, orderUUID uuid.UUID, input v1dto.CreateShippingInput) (v1dto.AsyncShippingResponseDTO, error) {
	order, err := s.orderRepo.GetByUUID(ctx, orderUUID)
	if err != nil {
		return v1dto.AsyncShippingResponseDTO{}, s.handleOrderNotFoundError(err)
	}

	if order.OrderStatus != OrderStatusConfirmed {
		return v1dto.AsyncShippingResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest,
			fmt.Sprintf("Cannot create shipping for order with status %d. Order must be Confirmed.", order.OrderStatus))
	}

	if order.OrderShippingCode != nil && strings.TrimSpace(*order.OrderShippingCode) != "" {
		return v1dto.AsyncShippingResponseDTO{},
			utils.NewError(utils.ErrCodeConflict, "Shipping already created for this order")
	}

	msg := v1dto.CreateShippingQueueMessage{
		OrderUUID:  orderUUID.String(),
		Input:      input,
		EnqueuedAt: time.Now(),
	}

	if err := s.rabbitmq.Publish(ctx, QueueShippingCreate, msg); err != nil {
		logger.Log.Error().Err(err).Str("order_uuid", orderUUID.String()).Msg("Failed to enqueue shipping creation")
		return v1dto.AsyncShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to enqueue shipping creation", err)
	}

	return v1dto.AsyncShippingResponseDTO{
		OrderUUID: orderUUID.String(),
		Status:    "processing",
		Message:   "Shipping order creation request received and is being processed by the system",
	}, nil
}

// ProcessShippingOrderMessage adapts raw AMQP messages for backward compatibility.
func (s *orderShippingService) ProcessShippingOrderMessage(headers amqp.Table, body []byte) error {
	var msg v1dto.CreateShippingQueueMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to unmarshal shipping creation message")
		return err
	}
	return s.ProcessShippingOrder(context.Background(), msg)
}

// ProcessShippingOrder processes asynchronous shipping creation using pure domain DTOs.
func (s *orderShippingService) ProcessShippingOrder(ctx context.Context, msg v1dto.CreateShippingQueueMessage) error {
	if ctx == nil {
		ctx = context.Background()
	}

	orderUUID, err := uuid.Parse(msg.OrderUUID)
	if err != nil {
		return err
	}

	order, err := s.orderRepo.GetByUUID(ctx, orderUUID)
	if err != nil {
		return err
	}

	if order.OrderStatus != OrderStatusConfirmed {
		logger.Log.Warn().Str("order_uuid", msg.OrderUUID).Msg("Order is no longer Confirmed, skipping shipping creation")
		return nil
	}

	orderItems, err := s.orderItemRepo.GetByOrderID(ctx, order.OrderID)
	if err != nil {
		return err
	}

	shippingItems := make([]shipping.ShippingItem, 0, len(orderItems))
	for _, item := range orderItems {
		shippingItems = append(shippingItems, shipping.ShippingItem{
			Name:     item.OrderItemProductName,
			Quantity: int(item.OrderItemQuantity),
			Price:    utils.NumericToFloat(item.OrderItemPrice),
			Weight:   msg.Input.Weight / len(orderItems),
		})
	}

	var codAmount float64
	if order.OrderPaymentMethod == OrderPaymentMethodCOD && order.OrderPaymentStatus == OrderPaymentStatusUnpaid {
		codAmount = utils.NumericToFloat(order.OrderTotalAmount)
	}
	provider, err := s.shippingFactory.GetProvider(msg.Input.Provider)
	if err != nil {
		return err
	}
	ghnReq := shipping.CreateOrderRequest{
		ToName:       order.OrderShippingName,
		ToPhone:      order.OrderShippingPhone,
		ToAddress:    order.OrderShippingAddress,
		ToDistrictID: msg.Input.DistrictID,
		ToWardCode:   msg.Input.WardCode,
		Weight:       msg.Input.Weight,
		Length:       msg.Input.Length,
		Width:        msg.Input.Width,
		Height:       msg.Input.Height,
		Items:        shippingItems,
		Note:         msg.Input.Note,
		RequiredNote: msg.Input.RequiredNote,
		CODAmount:    codAmount,
		CallbackURL:  utils.GetEnv("GHN_CALLBACK_URL", ""),
	}

	ghnResp, err := provider.CreateShippingOrder(ctx, ghnReq)
	if err != nil {
		logger.Log.Error().Err(err).Str("order_uuid", msg.OrderUUID).Msg("GHN API creation failed")
		if s.trackingRepo != nil {
			failNote := fmt.Sprintf("GHN shipping creation failed: %s", err.Error())
			_, _ = s.trackingRepo.InsertEvent(ctx, nil, sqlc.InsertTrackingEventParams{
				EventOrderID: order.OrderID,
				EventStatus:  "shipping_failed",
				EventNote:    &failNote,
				EventSource:  "system",
			})
		}
		return err
	}

	var expectedDelivery pgtype.Timestamptz
	if ghnResp.ExpectedDelivery != nil {
		expectedDelivery = pgtype.Timestamptz{
			Time:  *ghnResp.ExpectedDelivery,
			Valid: true,
		}
	}

	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		tx, _ := db.GetTx(txCtx)
		_, err = s.orderRepo.UpdateShippingAndStatus(txCtx, tx, orderUUID, sqlc.UpdateOrderShippingAndStatusParams{
			ShippingProvider: &msg.Input.Provider,
			ShippingCode:     &ghnResp.ShippingCode,
			ShippingFee:      utils.FloatToNumeric(ghnResp.ShippingFee),
			ExpectedDelivery: expectedDelivery,
			OrderStatus:      OrderStatusShipped,
			OrderUuid:        orderUUID,
		})
		if err != nil {
			return err
		}

		if s.trackingRepo != nil {
			eventNote := fmt.Sprintf("Created shipping order via %s successfully. Tracking code: %s, Fee: %.0f VND",
				strings.ToUpper(msg.Input.Provider), ghnResp.ShippingCode, ghnResp.ShippingFee)
			_, err = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
				EventOrderID: order.OrderID,
				EventStatus:  "shipped",
				EventNote:    &eventNote,
				EventSource:  "admin",
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		_ = provider.CancelShippingOrder(ctx, ghnResp.ShippingCode)
		return err
	}

	if s.cache != nil {
		_ = s.cache.Clear(fmt.Sprintf("order:%s", orderUUID.String()))
	}
	logger.Log.Info().Str("order_uuid", msg.OrderUUID).Str("shipping_code", ghnResp.ShippingCode).Msg("Shipping order created successfully via RabbitMQ")

	if s.rabbitmq != nil {
		evt := rabbitmq.NewOrderStatusChangedEvent(
			uuid.New().String(),
			orderUUID.String(),
			order.OrderNumber,
			order.UserUuid.String(),
			order.OrderID,
			order.OrderUserID,
			order.OrderStatus,
			OrderStatusShipped,
			ghnResp.ShippingCode,
		)
		_ = s.rabbitmq.Publish(ctx, rabbitmq.QueueOrderStatusChanged, evt)
	}

	return nil
}

func (s *orderShippingService) CancelShipping(ctx context.Context, orderUUID uuid.UUID, input v1dto.CancelShippingInput) (v1dto.CancelShippingResponseDTO, error) {
	order, err := s.orderRepo.GetByUUID(ctx, orderUUID)
	if err != nil {
		return v1dto.CancelShippingResponseDTO{}, s.handleOrderNotFoundError(err)
	}

	if order.OrderStatus != OrderStatusShipped {
		return v1dto.CancelShippingResponseDTO{}, utils.NewError(
			utils.ErrCodeBadRequest,
			fmt.Sprintf("Cannot cancel shipping for order with status %s. Order must be in Shipped status.", v1dto.MapOrderStatusToString(order.OrderStatus)),
		)
	}

	if order.OrderShippingCode == nil || strings.TrimSpace(*order.OrderShippingCode) == "" {
		return v1dto.CancelShippingResponseDTO{}, utils.NewError(
			utils.ErrCodeBadRequest,
			"Order does not have a valid shipping code",
		)
	}

	shippingCode := *order.OrderShippingCode
	providerName := "ghn"
	if order.OrderShippingProvider != nil && strings.TrimSpace(*order.OrderShippingProvider) != "" {
		providerName = *order.OrderShippingProvider
	}

	provider, err := s.shippingFactory.GetProvider(providerName)
	if err != nil {
		return v1dto.CancelShippingResponseDTO{}, utils.NewError(
			utils.ErrCodeBadRequest,
			fmt.Sprintf("Unsupported shipping provider: %s", providerName),
		)
	}

	if err := provider.CancelShippingOrder(ctx, shippingCode); err != nil {
		logger.Log.Error().
			Err(err).
			Str("provider", providerName).
			Str("shipping_code", shippingCode).
			Msg("Failed to cancel shipping order with provider")
		return v1dto.CancelShippingResponseDTO{}, utils.WrapError(
			utils.ErrCodeInternal,
			"Failed to cancel shipping order with provider",
			err,
		)
	}

	var updatedOrder sqlc.Order
	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		tx, _ := db.GetTx(txCtx)
		var err error
		updatedOrder, err = s.orderRepo.CancelShipping(txCtx, tx, orderUUID)
		if err != nil {
			return err
		}

		if s.trackingRepo != nil {
			eventNote := fmt.Sprintf("Cancelled waybill %s (%s) successfully.", strings.ToUpper(providerName), shippingCode)
			if input.Reason != "" {
				eventNote += fmt.Sprintf(" Reason: %s", input.Reason)
			}

			_, err = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
				EventOrderID: order.OrderID,
				EventStatus:  "shipping_cancelled",
				EventNote:    &eventNote,
				EventSource:  "admin",
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return v1dto.CancelShippingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update order status after shipping cancellation", err)
	}

	if s.cache != nil {
		_ = s.cache.Clear(fmt.Sprintf("order:%s", orderUUID.String()))
	}

	logger.Log.Info().
		Str("order_number", order.OrderNumber).
		Str("shipping_code", shippingCode).
		Msg("Order shipping cancelled successfully")

	return v1dto.CancelShippingResponseDTO{
		OrderUUID:    orderUUID.String(),
		OrderNumber:  updatedOrder.OrderNumber,
		ShippingCode: shippingCode,
		OrderStatus:  "Confirmed",
		Message:      "Shipping waybill cancelled successfully. Order status reverted to Confirmed.",
	}, nil
}

func (s *orderShippingService) ConfirmOrderDelivery(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.ConfirmDeliveryResponseDTO, error) {
	user, err := s.getCustomer(ctx, userUUID)
	if err != nil {
		return v1dto.ConfirmDeliveryResponseDTO{}, err
	}

	order, err := s.orderRepo.GetByUUID(ctx, orderUUID)
	if err != nil {
		return v1dto.ConfirmDeliveryResponseDTO{}, s.handleOrderNotFoundError(err)
	}

	isAdminOrMod := user.UserLevel == RoleAdmin || user.UserLevel == RoleModerator
	if order.OrderUserID != user.UserID && !isAdminOrMod {
		return v1dto.ConfirmDeliveryResponseDTO{}, utils.NewError(utils.ErrCodeForbidden, "Access denied")
	}

	if order.OrderStatus == OrderStatusDelivered {
		return v1dto.ConfirmDeliveryResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Order is already marked as delivered")
	}

	if order.OrderStatus != OrderStatusShipped {
		var errMsg string
		switch order.OrderStatus {
		case OrderStatusPending:
			errMsg = "Order is pending and has not been processed yet"
		case OrderStatusConfirmed:
			errMsg = "Order is confirmed but has not been shipped yet. Delivery can only be confirmed once shipped"
		case OrderStatusCancelled:
			errMsg = "Cannot confirm delivery for a cancelled order"
		case OrderStatusReturned:
			errMsg = "Cannot confirm delivery for a returned order"
		default:
			errMsg = fmt.Sprintf("Cannot confirm delivery for an order with status: %s", v1dto.MapOrderStatusToString(order.OrderStatus))
		}
		return v1dto.ConfirmDeliveryResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest, errMsg)
	}

	var updatedOrder sqlc.Order
	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		tx, _ := db.GetTx(txCtx)
		var err error
		updatedOrder, err = s.orderRepo.MarkDeliveredByUUID(txCtx, tx, orderUUID)
		if err != nil {
			return err
		}

		if s.trackingRepo != nil {
			eventNote := "Customer confirmed order receipt."
			_, err = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
				EventOrderID: order.OrderID,
				EventStatus:  "delivered",
				EventNote:    &eventNote,
				EventSource:  "user",
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return v1dto.ConfirmDeliveryResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update order to delivered", err)
	}

	if s.cache != nil {
		_ = s.cache.Clear(fmt.Sprintf("order:%s", orderUUID.String()))
		_ = s.cache.Clear("orders:*")
	}

	if s.rabbitmq != nil {
		evt := rabbitmq.NewOrderStatusChangedEvent(
			uuid.New().String(),
			updatedOrder.OrderUuid.String(),
			updatedOrder.OrderNumber,
			order.UserUuid.String(),
			updatedOrder.OrderID,
			updatedOrder.OrderUserID,
			order.OrderStatus,
			OrderStatusDelivered,
			"",
		)
		_ = s.rabbitmq.Publish(ctx, rabbitmq.QueueOrderStatusChanged, evt)
	}

	var deliveredAt *time.Time
	if updatedOrder.OrderDeliveredAt.Valid {
		deliveredAt = &updatedOrder.OrderDeliveredAt.Time
	}

	return v1dto.ConfirmDeliveryResponseDTO{
		OrderUUID:         updatedOrder.OrderUuid.String(),
		OrderNumber:       updatedOrder.OrderNumber,
		OrderStatus:       updatedOrder.OrderStatus,
		StatusName:        v1dto.MapOrderStatusToString(updatedOrder.OrderStatus),
		StatusText:        v1dto.MapOrderStatusToVietnamese(updatedOrder.OrderStatus),
		PaymentMethod:     v1dto.MapPaymentMethodToString(updatedOrder.OrderPaymentMethod),
		PaymentStatus:     updatedOrder.OrderPaymentStatus,
		PaymentStatusName: v1dto.MapPaymentStatusToString(updatedOrder.OrderPaymentStatus),
		DeliveredAt:       deliveredAt,
		Message:           "Order delivery confirmed successfully. You can now leave a product review or request a return within 7 days.",
	}, nil
}

