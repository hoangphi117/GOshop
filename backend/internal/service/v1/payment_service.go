package v1service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/payment"
	"goshop/pkg/rabbitmq"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	PaymentStatusPending  int32 = 1
	PaymentStatusPaid     int32 = 2
	PaymentStatusFailed   int32 = 3
	PaymentStatusExpired  int32 = 4
	PaymentStatusRefunded int32 = 5
)

type paymentService struct {
	paymentRepo         repository.PaymentRepository
	orderReader         PaymentOrderReaderPort
	userRepo            repository.UserReader
	webhookRepo         repository.WebhookEventRepository
	providers           map[string]payment.PaymentProvider
	notificationService NotificationService
	cache               cache.RedisCacheService
	rabbitmq            rabbitmq.RabbitMQService
	returnURL           string
	webhookBaseURL      string
	refundService       PaymentRefundService
	outboxRepo          repository.OutboxRepository
}

// NewPaymentService creates a new instance of PaymentService.
func NewPaymentService(
	paymentRepo repository.PaymentRepository,
	orderReader PaymentOrderReaderPort,
	userRepo repository.UserReader,
	webhookRepo repository.WebhookEventRepository,
	providers map[string]payment.PaymentProvider,
	notificationService NotificationService,
	cache cache.RedisCacheService,
	rabbitmq rabbitmq.RabbitMQService,
	returnURL string,
	webhookBaseURL string,
	outboxRepo ...repository.OutboxRepository,
) PaymentService {
	var orderRepo repository.OrderRepository
	if repoProvider, ok := orderReader.(interface{ OrderRepository() repository.OrderRepository }); ok {
		orderRepo = repoProvider.OrderRepository()
	}
	refundService := NewPaymentRefundService(paymentRepo, orderRepo, providers, notificationService, cache)
	var obRepo repository.OutboxRepository
	if len(outboxRepo) > 0 {
		obRepo = outboxRepo[0]
	}
	return &paymentService{
		paymentRepo:         paymentRepo,
		orderReader:         orderReader,
		userRepo:            userRepo,
		webhookRepo:         webhookRepo,
		providers:           providers,
		notificationService: notificationService,
		cache:               cache,
		rabbitmq:            rabbitmq,
		returnURL:           returnURL,
		webhookBaseURL:      webhookBaseURL,
		refundService:       refundService,
		outboxRepo:          obRepo,
	}
}

// WithRefundService allows overriding the PaymentRefundService instance (useful for testing).
func (s *paymentService) WithRefundService(refundService PaymentRefundService) *paymentService {
	s.refundService = refundService
	return s
}

// GetPaymentHistory returns all payment transactions associated with an order.
func (s *paymentService) GetPaymentHistory(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) ([]v1dto.PaymentTransactionDTO, error) {
	user, err := s.getUser(ctx, userUUID)
	if err != nil {
		return nil, err
	}

	order, err := s.orderReader.GetByUUID(ctx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get order", err)
	}

	if order.UserID != user.UserID && user.UserLevel != RoleAdmin && user.UserLevel != RoleModerator {
		return nil, utils.NewError(utils.ErrCodeForbidden, "You are not authorized to view payment history for this order")
	}

	paymentTxns, err := s.paymentRepo.GetByOrderID(ctx, nil, order.OrderID)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get payment transactions", err)
	}

	history := make([]v1dto.PaymentTransactionDTO, 0, len(paymentTxns))
	for _, txn := range paymentTxns {
		history = append(history, v1dto.MapToPaymentTransactionDTO(txn, order.OrderUUID))
	}

	return history, nil
}

// ConfirmCODPayment confirms a cash-on-delivery payment, records the payment transaction, and emits an outbox event.
func (s *paymentService) ConfirmCODPayment(ctx context.Context, orderUUID uuid.UUID) error {
	startTime := time.Now()

	tx, err := db.BeginTx(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to begin database transaction for COD confirmation")
		return utils.WrapError(utils.ErrCodeInternal, "failed to begin database transaction", err)
	}
	defer tx.Rollback(ctx)

	order, err := s.orderReader.GetByUUID(ctx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return utils.WrapError(utils.ErrCodeInternal, "failed to get order", err)
	}

	if order.PaymentMethod != OrderPaymentMethodCOD {
		return utils.NewError(utils.ErrCodeBadRequest, "Order is not a COD order")
	}

	if order.PaymentStatus == OrderPaymentStatusPaid {
		return utils.NewError(utils.ErrCodeBadRequest, "Order has already been confirmed as paid")
	}

	if order.OrderStatus == OrderStatusCancelled {
		return utils.NewError(utils.ErrCodeBadRequest, "Order is cancelled, cannot confirm payment")
	}

	idempotencyKey := fmt.Sprintf("cod:%s", order.OrderNumber)
	providerTxnID := fmt.Sprintf("COD_%s", order.OrderNumber)

	ptParams := sqlc.CreatePaymentTransactionParams{
		PtOrderID:        order.OrderID,
		PtIdempotencyKey: idempotencyKey,
		PtProvider:       "cod",
		PtProviderTxnID:  &providerTxnID,
		PtAmount:         utils.FloatToNumeric(order.TotalAmount),
		PtCurrency:       "VND",
		PtStatus:         PaymentStatusPaid,
	}

	paymentTxn, err := s.paymentRepo.Create(ctx, tx, ptParams)
	if err != nil {
		logger.Log.Error().Err(err).Str("order_number", order.OrderNumber).Msg("Failed to create COD payment transaction")
		return utils.WrapError(utils.ErrCodeInternal, "failed to create payment transaction", err)
	}

	if s.outboxRepo != nil {
		event := rabbitmq.NewPaymentSucceededEvent(
			uuid.New().String(),
			paymentTxn.PtUuid.String(),
			order.OrderUUID.String(),
			order.OrderNumber,
			order.UserUUID.String(),
			"cod",
			providerTxnID,
			order.TotalAmount,
		)
		if _, err := s.outboxRepo.Insert(ctx, tx, "payment", paymentTxn.PtUuid.String(), rabbitmq.QueuePaymentSucceeded, event); err != nil {
			logger.Log.Error().Err(err).Str("order_number", order.OrderNumber).Msg("Failed to enqueue payment.succeeded outbox event for COD confirmation")
			return utils.WrapError(utils.ErrCodeInternal, "failed to enqueue payment event", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to commit database transaction for COD confirmation")
		return utils.WrapError(utils.ErrCodeInternal, "failed to commit database transaction", err)
	}

	if s.cache != nil {
		if clearErr := s.cache.Clear("orders:*"); clearErr != nil {
			logger.Log.Warn().Err(clearErr).Msg("Failed to clear orders cache after COD payment confirmation")
		}
	}

	logger.Log.Info().
		Str("trace_id", logger.GetTraceId(ctx)).
		Str("module", "payment").
		Str("action", "confirm_cod").
		Str("order_number", order.OrderNumber).
		Str("order_uuid", order.OrderUUID.String()).
		Str("payment_uuid", paymentTxn.PtUuid.String()).
		Float64("amount", order.TotalAmount).
		Int64("duration_ms", time.Since(startTime).Milliseconds()).
		Msg("COD payment confirmed successfully")
	return nil
}

// RefundPayment delegates refund processing to PaymentRefundService.
func (s *paymentService) RefundPayment(ctx context.Context, idempotencyKey string, input v1dto.RefundPaymentInput) (v1dto.PaymentTransactionDTO, error) {
	if s.refundService != nil {
		return s.refundService.RefundPayment(ctx, idempotencyKey, input)
	}
	return v1dto.PaymentTransactionDTO{}, utils.NewError(utils.ErrCodeInternal, "refund service not initialized")
}

func (s *paymentService) getUser(ctx context.Context, userUUID uuid.UUID) (sqlc.User, error) {
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.User{}, utils.NewError(utils.ErrCodeNotFound, "User not found")
		}
		return sqlc.User{}, utils.WrapError(utils.ErrCodeInternal, "failed to get user", err)
	}
	return user, nil
}
