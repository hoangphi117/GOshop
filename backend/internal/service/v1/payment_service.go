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
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/payment"
	"goshop/pkg/rabbitmq"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sony/gobreaker/v2"
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
	orderRepo           repository.OrderRepository
	userRepo            repository.UserRepository
	orderItemRepo       repository.OrderItemRepository
	trackingRepo        repository.TrackingRepository
	inventoryService    InventoryService
	webhookRepo         repository.WebhookEventRepository
	providers           map[string]payment.PaymentProvider
	notificationService NotificationService
	cache               cache.RedisCacheService
	rabbitmq            rabbitmq.RabbitMQService
	returnURL           string
	webhookBaseURL      string
	refundService       PaymentRefundService
}

type PaymentOrderDependencies struct {
	InventoryService InventoryService
	OrderItemRepo    repository.OrderItemRepository
	TrackingRepo     repository.TrackingRepository
}

// NewPaymentService creates a new instance of PaymentService.
func NewPaymentService(
	paymentRepo repository.PaymentRepository,
	orderRepo repository.OrderRepository,
	userRepo repository.UserRepository,
	webhookRepo repository.WebhookEventRepository,
	providers map[string]payment.PaymentProvider,
	notificationService NotificationService,
	cache cache.RedisCacheService,
	rabbitmq rabbitmq.RabbitMQService,
	returnURL string,
	webhookBaseURL string,
	orderDeps ...PaymentOrderDependencies,
) PaymentService {
	refundService := NewPaymentRefundService(paymentRepo, orderRepo, providers, notificationService, cache)
	svc := &paymentService{
		paymentRepo:         paymentRepo,
		orderRepo:           orderRepo,
		userRepo:            userRepo,
		webhookRepo:         webhookRepo,
		providers:           providers,
		notificationService: notificationService,
		cache:               cache,
		rabbitmq:            rabbitmq,
		returnURL:           returnURL,
		webhookBaseURL:      webhookBaseURL,
		refundService:       refundService,
	}
	if len(orderDeps) > 0 {
		svc.inventoryService = orderDeps[0].InventoryService
		svc.orderItemRepo = orderDeps[0].OrderItemRepo
		svc.trackingRepo = orderDeps[0].TrackingRepo
	}
	return svc
}

// WithRefundService allows overriding the PaymentRefundService instance (useful for testing).
func (s *paymentService) WithRefundService(refundService PaymentRefundService) *paymentService {
	s.refundService = refundService
	return s
}

// ── Service Implementation ────────────────────────────────────────

// 1. TẠO PAYMENT INTENT
func (s *paymentService) CreatePaymentIntent(ctx context.Context, userUUID uuid.UUID, clientIP, idempotencyKey string, input v1dto.CreatePaymentIntentInput) (v1dto.PaymentIntentDTO, error) {
	startTime := time.Now()

	user, err := s.getUser(ctx, userUUID)
	if err != nil {
		return v1dto.PaymentIntentDTO{}, err
	}

	order, err := s.orderRepo.GetByUUID(ctx, input.OrderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return v1dto.PaymentIntentDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to get order", err)
	}

	if err := s.validateOrderForPayment(order, user); err != nil {
		return v1dto.PaymentIntentDTO{}, err
	}

	orderAmount := utils.NumericToFloat(order.OrderTotalAmount)

	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Missing X-Idempotency-Key header")
	}

	cached, err := s.checkIdempotency(ctx, idempotencyKey, order)
	if err != nil {
		return v1dto.PaymentIntentDTO{}, err
	}
	if cached != nil {
		return *cached, nil
	}

	providerName := strings.ToLower(strings.TrimSpace(input.Provider))
	provider, exists := s.providers[providerName]
	if !exists {
		return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Unsupported payment provider: %s", providerName))
	}

	txnRef := fmt.Sprintf("%s_%d", order.OrderNumber, time.Now().Unix())
	webhookURL := s.buildWebhookURL(providerName)

	paymentReq := payment.CreatePaymentRequest{
		OrderNumber: txnRef,
		Amount:      orderAmount,
		Currency:    "VND",
		Description: fmt.Sprintf("Payment for order %s", order.OrderNumber),
		ReturnURL:   s.returnURL,
		WebhookURL:  webhookURL,
		ClientIP:    clientIP,
	}
	providerResp, err := provider.CreatePaymentURL(ctx, paymentReq)
	if err != nil {
		logger.Log.Error().
			Err(err).
			Str("trace_id", logger.GetTraceId(ctx)).
			Str("module", "payment").
			Str("action", "create_intent").
			Str("provider", providerName).
			Str("order_uuid", order.OrderUuid.String()).
			Msg("Failed to generate payment URL from provider")
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return v1dto.PaymentIntentDTO{}, utils.WrapError(utils.ErrCodeServiceUnavailable, "Payment gateway is temporarily unavailable. Please try again later.", err)
		}
		return v1dto.PaymentIntentDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to connect to payment gateway. Please try again later.", err)
	}

	var providerURL *string
	if providerResp.PaymentURL != "" {
		providerURL = &providerResp.PaymentURL
	}

	ptParams := sqlc.CreatePaymentTransactionParams{
		PtOrderID:        order.OrderID,
		PtIdempotencyKey: idempotencyKey,
		PtProvider:       providerName,
		PtTxnRef:         &txnRef,
		PtProviderTxnID:  nil,
		PtProviderUrl:    providerURL,
		PtAmount:         order.OrderTotalAmount,
		PtCurrency:       "VND",
		PtStatus:         PaymentStatusPending,
	}

	paymentTxn, err := s.paymentRepo.Create(ctx, nil, ptParams)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			logger.Log.Warn().Str("key", idempotencyKey).Msg("Concurrent duplicate payment intent request detected, fetching existing transaction")
			existingTxn, getErr := s.paymentRepo.GetByIdempotencyKey(ctx, nil, idempotencyKey)
			if getErr == nil {
				return v1dto.MapToPaymentIntentDTO(existingTxn, order.OrderUuid), nil
			}
		}
		return v1dto.PaymentIntentDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to create payment transaction", err)
	}

	responseDTO := v1dto.MapToPaymentIntentDTO(paymentTxn, order.OrderUuid)
	redisKey := fmt.Sprintf("idem:payment:%s", idempotencyKey)
	if s.cache != nil {
		_ = s.cache.Set(redisKey, responseDTO, 24*time.Hour)
	}

	logger.Log.Info().
		Str("trace_id", logger.GetTraceId(ctx)).
		Str("module", "payment").
		Str("action", "create_intent").
		Str("order_number", order.OrderNumber).
		Str("order_uuid", order.OrderUuid.String()).
		Str("payment_uuid", paymentTxn.PtUuid.String()).
		Str("provider", providerName).
		Str("status", "Pending").
		Float64("amount", orderAmount).
		Int64("duration_ms", time.Since(startTime).Milliseconds()).
		Msg("Payment intent created successfully")

	return responseDTO, nil
}

// 2. NHẬN WEBHOOK TỪ CỔNG (VERIFY + DEDUP + PUSH RABBITMQ)
func (s *paymentService) HandleWebhook(ctx context.Context, providerName string, rawPayload []byte, headers map[string]string) error {
	startTime := time.Now()

	providerName = strings.ToLower(strings.TrimSpace(providerName))
	provider, exists := s.providers[providerName]
	if !exists {
		return utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Unsupported payment provider: %s", providerName))
	}

	if err := provider.VerifyWebhook(rawPayload, headers); err != nil {
		logger.Log.Warn().
			Err(err).
			Str("module", "payment").
			Str("action", "webhook_received").
			Str("provider", providerName).
			Msg("Invalid webhook signature")
		return utils.NewError(utils.ErrCodeUnauthorized, "Invalid webhook signature")
	}

	result, err := provider.ParseWebhookPayload(rawPayload)
	if err != nil {
		return utils.WrapError(utils.ErrCodeBadRequest, "Failed to parse webhook payload", err)
	}

	eventParams := sqlc.InsertWebhookEventParams{
		WeProvider:        providerName,
		WeExternalEventID: result.EventID,
		WeEventType:       result.EventType,
		WeRawBody:         string(rawPayload),
	}

	event, err := s.webhookRepo.InsertEvent(ctx, nil, eventParams)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Log.Info().
				Str("provider", providerName).
				Str("event_id", result.EventID).
				Msg("Duplicate webhook event received (ON CONFLICT), acknowledging gateway")
			return nil
		}
		logger.Log.Error().
			Err(err).
			Str("provider", providerName).
			Str("event_id", result.EventID).
			Msg("Database failure while inserting webhook event")
		return utils.WrapError(utils.ErrCodeInternal, "failed to persist webhook event", err)
	}

	if event.WeID == 0 {
		logger.Log.Info().
			Str("provider", providerName).
			Str("event_id", result.EventID).
			Msg("Duplicate webhook event received (WeID=0), acknowledging gateway")
		return nil
	}

	realOrderNumber, _, _ := strings.Cut(result.OrderNumber, "_")

	var orderUUID string
	order, err := s.orderRepo.GetByOrderNumber(ctx, realOrderNumber)
	if err == nil {
		orderUUID = order.OrderUuid.String()
	}

	msgPayload := v1dto.PaymentWebhookMessage{
		WebhookEventID: event.WeID,
		OrderUUID:      orderUUID,
		OrderNumber:    result.OrderNumber,
		Provider:       providerName,
		EventType:      result.EventType,
		ProviderTxnID:  result.ProviderTxnID,
		Amount:         result.Amount,
		Status:         string(result.Status),
		Metadata:       result.Metadata,
	}

	if err := s.rabbitmq.Publish(ctx, "payment.webhook", msgPayload); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to publish payment webhook to RabbitMQ")
		return utils.WrapError(utils.ErrCodeInternal, "failed to enqueue webhook event", err)
	}

	logger.Log.Info().
		Str("module", "payment").
		Str("action", "webhook_received").
		Str("provider", providerName).
		Str("event_id", result.EventID).
		Str("order_number", realOrderNumber).
		Int64("duration_ms", time.Since(startTime).Milliseconds()).
		Msg("Payment webhook accepted and enqueued")

	return nil
}

// 3. CONSUMER XỬ LÝ MESSAGE CẬP NHẬT TRẠNG THÁI TRONG DB
func (s *paymentService) ProcessWebhookMessage(headers amqp.Table, body []byte) error {
	var msg v1dto.PaymentWebhookMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to unmarshal payment webhook message")
		return err
	}
	return s.ProcessWebhook(context.Background(), msg)
}

func (s *paymentService) ProcessWebhook(ctx context.Context, msg v1dto.PaymentWebhookMessage) error {
	if ctx == nil {
		ctx = context.Background()
	}
	startTime := time.Now()

	var newPaymentStatus int32 = PaymentStatusFailed
	if msg.Status == "success" {
		newPaymentStatus = PaymentStatusPaid
	}

	tx, err := db.BeginTx(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to begin database transaction")
		return err
	}
	defer tx.Rollback(ctx)

	pt, err := s.paymentRepo.GetByTxnRef(ctx, tx, msg.Provider, msg.OrderNumber)
	if err != nil {
		pt, err = s.paymentRepo.GetByProviderTxnID(ctx, tx, msg.Provider, msg.OrderNumber)
	}
	if err != nil {
		logger.Log.Error().
			Err(err).
			Str("provider", msg.Provider).
			Str("txn_ref", msg.OrderNumber).
			Msg("Payment transaction not found by txn_ref")
		return err
	}

	if pt.PtStatus == PaymentStatusPaid {
		logger.Log.Info().
			Str("module", "payment").
			Str("action", "webhook_idempotent_skip").
			Str("pt_uuid", pt.PtUuid.String()).
			Str("order_number", msg.OrderNumber).
			Msg("Payment transaction is already Paid, acknowledging redelivered message as idempotent success")

		_ = s.webhookRepo.MarkProcessed(ctx, tx, msg.WebhookEventID)
		_ = tx.Commit(ctx)
		return nil
	}

	metaBytes, _ := json.Marshal(map[string]interface{}{
		"txn_ref":  msg.OrderNumber,
		"trans_id": msg.ProviderTxnID,
	})
	_, err = s.paymentRepo.UpdateStatusAtomically(ctx, tx, sqlc.UpdatePaymentStatusAtomicallyParams{
		PtUuid:              pt.PtUuid,
		NewStatus:           newPaymentStatus,
		ProviderTxnID:       &msg.ProviderTxnID,
		Metadata:            metaBytes,
		AllowedFromStatuses: []int32{PaymentStatusPending},
	})
	if err != nil {
		logger.Log.Error().Err(err).Str("pt_uuid", pt.PtUuid.String()).Msg("Failed to update payment transaction")
		return err
	}

	var paidOrder *sqlc.GetOrderByOrderNumberRow
	if msg.Status == "success" {
		realOrderNumber, _, _ := strings.Cut(msg.OrderNumber, "_")

		order, err := s.orderRepo.GetByOrderNumber(ctx, realOrderNumber)
		if err == nil {
			paidOrder = &order
			_, err = s.orderRepo.UpdateOrderPaymentStatus(ctx, tx, sqlc.UpdateOrderPaymentStatusParams{
				OrderUuid:           order.OrderUuid,
				PaymentStatus:       OrderPaymentStatusPaid,
				RefundedAmount:      utils.FloatToNumeric(0),
				AllowedFromStatuses: []int32{OrderPaymentStatusUnpaid},
			})
			if err != nil {
				logger.Log.Error().Err(err).Str("order_number", realOrderNumber).Msg("Failed to update order payment status")
				return err
			}
		}
	}

	if err := s.webhookRepo.MarkProcessed(ctx, tx, msg.WebhookEventID); err != nil {
		logger.Log.Error().
			Err(err).
			Int32("webhook_event_id", msg.WebhookEventID).
			Msg("Failed to mark webhook event as processed")
		return utils.WrapError(utils.ErrCodeInternal, "failed to mark webhook event as processed", err)
	}

	if err := tx.Commit(ctx); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to commit database transaction")
		return err
	}

	if s.cache != nil {
		_ = s.cache.Clear("orders:*")
	}

	if msg.Status == "success" && paidOrder != nil {
		s.dispatchSuccessNotification(*paidOrder, msg.ProviderTxnID)
	}

	logger.Log.Info().
		Str("module", "payment").
		Str("action", "webhook_processed").
		Str("order_number", msg.OrderNumber).
		Str("status", msg.Status).
		Int64("duration_ms", time.Since(startTime).Milliseconds()).
		Msg("Payment webhook processed successfully")

	return nil
}

func (s *paymentService) GetPaymentHistory(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) ([]v1dto.PaymentTransactionDTO, error) {
	user, err := s.getUser(ctx, userUUID)
	if err != nil {
		return nil, err
	}

	order, err := s.orderRepo.GetByUUID(ctx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get order", err)
	}

	if order.OrderUserID != user.UserID && user.UserLevel != RoleAdmin && user.UserLevel != RoleModerator {
		return nil, utils.NewError(utils.ErrCodeForbidden, "You are not authorized to view payment history for this order")
	}

	paymentTxns, err := s.paymentRepo.GetByOrderID(ctx, nil, order.OrderID)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get payment transactions", err)
	}

	history := make([]v1dto.PaymentTransactionDTO, 0, len(paymentTxns))
	for _, txn := range paymentTxns {
		history = append(history, v1dto.MapToPaymentTransactionDTO(txn, order.OrderUuid))
	}

	return history, nil
}

func (s *paymentService) ConfirmCODPayment(ctx context.Context, orderUUID uuid.UUID) error {
	startTime := time.Now()

	tx, err := db.BeginTx(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to begin database transaction for COD confirmation")
		return utils.WrapError(utils.ErrCodeInternal, "failed to begin database transaction", err)
	}
	defer tx.Rollback(ctx)

	order, err := s.orderRepo.GetByUUID(ctx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return utils.WrapError(utils.ErrCodeInternal, "failed to get order", err)
	}

	if order.OrderPaymentMethod != OrderPaymentMethodCOD {
		return utils.NewError(utils.ErrCodeBadRequest, "Order is not a COD order")
	}

	if order.OrderPaymentStatus == OrderPaymentStatusPaid {
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
		PtAmount:         order.OrderTotalAmount,
		PtCurrency:       "VND",
		PtStatus:         PaymentStatusPaid,
	}

	paymentTxn, err := s.paymentRepo.Create(ctx, tx, ptParams)
	if err != nil {
		logger.Log.Error().Err(err).Str("order_number", order.OrderNumber).Msg("Failed to create COD payment transaction")
		return utils.WrapError(utils.ErrCodeInternal, "failed to create payment transaction", err)
	}

	_, err = s.orderRepo.UpdateOrderPaymentStatus(ctx, tx, sqlc.UpdateOrderPaymentStatusParams{
		OrderUuid:           order.OrderUuid,
		PaymentStatus:       OrderPaymentStatusPaid,
		RefundedAmount:      utils.FloatToNumeric(0),
		AllowedFromStatuses: []int32{OrderPaymentStatusUnpaid},
	})
	if err != nil {
		logger.Log.Error().Err(err).Str("order_number", order.OrderNumber).Msg("Failed to update order payment status for COD confirmation")
		return utils.WrapError(utils.ErrCodeInternal, "failed to update order payment status", err)
	}

	if err := tx.Commit(ctx); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to commit database transaction for COD confirmation")
		return utils.WrapError(utils.ErrCodeInternal, "failed to commit database transaction", err)
	}

	if s.cache != nil {
		_ = s.cache.Clear("orders:*")
	}

	logger.Log.Info().
		Str("trace_id", logger.GetTraceId(ctx)).
		Str("module", "payment").
		Str("action", "confirm_cod").
		Str("order_number", order.OrderNumber).
		Str("order_uuid", order.OrderUuid.String()).
		Str("payment_uuid", paymentTxn.PtUuid.String()).
		Float64("amount", utils.NumericToFloat(order.OrderTotalAmount)).
		Int64("duration_ms", time.Since(startTime).Milliseconds()).
		Msg("COD payment confirmed successfully")
	return nil
}

func (s *paymentService) RefundPayment(ctx context.Context, idempotencyKey string, input v1dto.RefundPaymentInput) (v1dto.PaymentTransactionDTO, error) {
	if s.refundService != nil {
		return s.refundService.RefundPayment(ctx, idempotencyKey, input)
	}
	return v1dto.PaymentTransactionDTO{}, utils.NewError(utils.ErrCodeInternal, "refund service not initialized")
}

func (s *paymentService) ReconcilePendingPayments(ctx context.Context) error {
	startTime := time.Now()

	lockKey := "lock:payment:reconciliation"
	if s.cache != nil {
		acquired, err := s.cache.SetNX(lockKey, "locked", 4*time.Minute)
		if err != nil {
			logger.Log.Error().Err(err).Msg("Reconciliation: Failed to acquire distributed lock in Redis")
			return err
		}
		if !acquired {
			logger.Log.Info().Msg("Reconciliation: Another instance is currently running reconciliation, skipping this cycle")
			return nil
		}
		defer func() {
			_ = s.cache.Del(lockKey)
		}()
	}

	olderThan := time.Now().Add(-15 * time.Minute)

	pendingTxns, err := s.paymentRepo.GetPendingTransactionsOlderThan(ctx, nil, sqlc.GetPendingTransactionsOlderThanParams{
		OlderThan: olderThan,
		Lim:       50,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Reconciliation: Failed to fetch pending payment transactions")
		return utils.WrapError(utils.ErrCodeInternal, "failed to fetch pending transactions for reconciliation", err)
	}

	if len(pendingTxns) == 0 {
		return nil
	}

	logger.Log.Info().
		Int("count", len(pendingTxns)).
		Msg("Reconciliation: Processing expired/pending payment transactions")

	successCount := 0
	expiredCount := 0

	for _, txn := range pendingTxns {
		provider, exists := s.providers[txn.PtProvider]
		if !exists {
			logger.Log.Warn().
				Str("provider", txn.PtProvider).
				Str("pt_uuid", txn.PtUuid.String()).
				Msg("Reconciliation: Provider not supported for stale pending transaction, marking as Expired")

			tx, err := db.BeginTx(ctx)
			if err != nil {
				logger.Log.Error().Err(err).Msg("Reconciliation: Failed to begin db transaction for unsupported provider")
				continue
			}

			_, err = s.paymentRepo.UpdateStatusAtomically(ctx, tx, sqlc.UpdatePaymentStatusAtomicallyParams{
				PtUuid:              txn.PtUuid,
				NewStatus:           PaymentStatusExpired,
				AllowedFromStatuses: []int32{PaymentStatusPending},
			})
			if err != nil {
				_ = tx.Rollback(ctx)
				if !errors.Is(err, pgx.ErrNoRows) {
					logger.Log.Error().Err(err).Str("pt_uuid", txn.PtUuid.String()).Msg("Reconciliation: Failed to mark unsupported provider transaction as Expired")
				}
				continue
			}

			if err := s.cancelExpiredOrder(ctx, tx, txn); err != nil {
				logger.Log.Error().Err(err).Str("order_number", txn.OrderNumber).Msg("Reconciliation: Failed to cancel expired order for unsupported provider")
			}

			if err := tx.Commit(ctx); err != nil {
				logger.Log.Error().Err(err).Msg("Reconciliation: Failed to commit Expired update for unsupported provider")
				continue
			}

			expiredCount++
			s.dispatchExpirationSideEffects(txn)
			continue
		}

		queryOrderID := txn.OrderNumber
		if txn.PtTxnRef != nil && *txn.PtTxnRef != "" {
			queryOrderID = *txn.PtTxnRef
		} else if txn.PtProviderTxnID != nil && *txn.PtProviderTxnID != "" {
			queryOrderID = *txn.PtProviderTxnID
		}

		status, err := provider.QueryTransactionStatus(ctx, queryOrderID)
		if err != nil {
			logger.Log.Warn().
				Err(err).
				Str("provider", txn.PtProvider).
				Str("query_order_id", queryOrderID).
				Msg("Reconciliation: Failed to query transaction status from gateway")
			continue
		}

		tx, err := db.BeginTx(ctx)
		if err != nil {
			logger.Log.Error().Err(err).Msg("Reconciliation: Failed to begin db transaction")
			continue
		}

		switch status {
		case payment.TxnStatusSuccess:
			_, err = s.paymentRepo.UpdateStatusAtomically(ctx, tx, sqlc.UpdatePaymentStatusAtomicallyParams{
				PtUuid:              txn.PtUuid,
				NewStatus:           PaymentStatusPaid,
				AllowedFromStatuses: []int32{PaymentStatusPending},
			})
			if err != nil {
				_ = tx.Rollback(ctx)
				if errors.Is(err, pgx.ErrNoRows) {
					logger.Log.Info().
						Str("pt_uuid", txn.PtUuid.String()).
						Msg("Reconciliation: Transaction was already updated concurrently (idempotent skip)")
				} else {
					logger.Log.Error().
						Err(err).
						Str("pt_uuid", txn.PtUuid.String()).
						Msg("Reconciliation: DB error updating payment status to Paid")
				}
				continue
			}

			_, err = s.orderRepo.UpdateOrderPaymentStatus(ctx, tx, sqlc.UpdateOrderPaymentStatusParams{
				OrderUuid:           txn.OrderUuid,
				PaymentStatus:       OrderPaymentStatusPaid,
				RefundedAmount:      utils.FloatToNumeric(0),
				AllowedFromStatuses: []int32{OrderPaymentStatusUnpaid, OrderPaymentStatusPaid},
			})
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					logger.Log.Info().
						Str("order_number", txn.OrderNumber).
						Msg("Reconciliation: Order was already updated concurrently (idempotent skip)")
				} else {
					_ = tx.Rollback(ctx)
					logger.Log.Error().
						Err(err).
						Str("order_number", txn.OrderNumber).
						Msg("Reconciliation: DB error updating order payment status to Paid")
					continue
				}
			}

			if err := tx.Commit(ctx); err != nil {
				logger.Log.Error().Err(err).Msg("Reconciliation: Failed to commit Paid update")
				continue
			}

			successCount++
			logger.Log.Info().
				Str("order_number", txn.OrderNumber).
				Str("pt_uuid", txn.PtUuid.String()).
				Msg("Reconciliation: Successfully recovered missed paid transaction")

		case payment.TxnStatusFailed, payment.TxnStatusExpired, payment.TxnStatusPending:
			_, err = s.paymentRepo.UpdateStatusAtomically(ctx, tx, sqlc.UpdatePaymentStatusAtomicallyParams{
				PtUuid:              txn.PtUuid,
				NewStatus:           PaymentStatusExpired,
				AllowedFromStatuses: []int32{PaymentStatusPending},
			})
			if err != nil {
				_ = tx.Rollback(ctx)
				if errors.Is(err, pgx.ErrNoRows) {
					logger.Log.Info().
						Str("pt_uuid", txn.PtUuid.String()).
						Msg("Reconciliation: Transaction was already updated concurrently before expiring (idempotent skip)")
				} else {
					logger.Log.Error().
						Err(err).
						Str("pt_uuid", txn.PtUuid.String()).
						Msg("Reconciliation: DB error expiring pending transaction")
				}
				continue
			}

			if err := s.cancelExpiredOrder(ctx, tx, txn); err != nil {
				logger.Log.Error().Err(err).Str("order_number", txn.OrderNumber).Msg("Reconciliation: Failed to cancel expired order")
			}

			if err := tx.Commit(ctx); err != nil {
				logger.Log.Error().Err(err).Msg("Reconciliation: Failed to commit Expired update")
				continue
			}

			expiredCount++
			s.dispatchExpirationSideEffects(txn)
			logger.Log.Info().
				Str("pt_uuid", txn.PtUuid.String()).
				Str("order_number", txn.OrderNumber).
				Msg("Reconciliation: Marked abandoned payment transaction as Expired and cancelled order")
		}
	}

	if successCount > 0 && s.cache != nil {
		_ = s.cache.Clear("orders:*")
	}

	logger.Log.Info().
		Int("recovered_paid", successCount).
		Int("marked_expired", expiredCount).
		Int64("duration_ms", time.Since(startTime).Milliseconds()).
		Msg("Reconciliation: Job finished successfully")

	return nil
}

// ── Private SRP Helper Methods ────────────────────────────────────

func (s *paymentService) cancelExpiredOrder(ctx context.Context, tx pgx.Tx, txn sqlc.GetPendingTransactionsOlderThanRow) error {
	order, err := s.orderRepo.GetByUUID(ctx, txn.OrderUuid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}

	// Only cancel if order is still Pending and Unpaid
	if order.OrderStatus != OrderStatusPending || order.OrderPaymentStatus == OrderPaymentStatusPaid {
		return nil
	}

	// 1. Atomically cancel order (updates order_status = 5 Cancelled, order_cancelled_at = NOW())
	_, err = s.orderRepo.CancelOrder(ctx, tx, order.OrderUuid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	// 2. Restore inventory if inventoryService and orderItemRepo are available
	if s.inventoryService != nil && s.orderItemRepo != nil {
		orderItems, err := s.orderItemRepo.GetByOrderID(ctx, order.OrderID)
		if err == nil && len(orderItems) > 0 {
			cancelReason := "Payment expired after 15 minutes"
			txCtx := db.InjectTx(ctx, tx)
			if err := s.inventoryService.RestoreStockWithRef(txCtx, toRestoreItems(orderItems), order.OrderNumber, "order_cancel", nil, &cancelReason); err != nil {
				return fmt.Errorf("failed to restore stock for expired order: %w", err)
			}
		}
	}

	// 4. Record tracking event
	if s.trackingRepo != nil {
		cancelNote := "Order was automatically cancelled due to payment expiration (15 minutes timeout)."
		_, _ = s.trackingRepo.InsertEvent(ctx, tx, sqlc.InsertTrackingEventParams{
			EventOrderID: order.OrderID,
			EventStatus:  "Cancelled",
			EventNote:    &cancelNote,
			EventSource:  "system",
		})
	}

	return nil
}

func (s *paymentService) dispatchExpirationSideEffects(txn sqlc.GetPendingTransactionsOlderThanRow) {
	if s.cache != nil {
		_ = s.cache.Clear("orders:*")
	}

	if s.notificationService != nil && s.userRepo != nil {
		go func() {
			user, err := s.userRepo.GetByUUID(context.Background(), txn.UserUuid)
			if err == nil {
				_ = s.notificationService.Send(context.Background(), v1dto.SendNotificationInput{
					UserID: user.UserID,
					Type:   "order_cancelled",
					Title:  "Order Cancelled",
					Body:   fmt.Sprintf("Your order #%s was automatically cancelled due to payment expiration (15 minutes).", txn.OrderNumber),
					Data: map[string]any{
						"order_uuid":   txn.OrderUuid.String(),
						"order_number": txn.OrderNumber,
						"reason":       "Payment timeout",
					},
				})
			}
		}()
	}
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

func (s *paymentService) validateOrderForPayment(order sqlc.GetOrderByUUIDRow, user sqlc.User) error {
	if order.OrderUserID != user.UserID {
		return utils.NewError(utils.ErrCodeForbidden, "You are not authorized to pay for this order")
	}

	if order.OrderPaymentStatus == OrderPaymentStatusPaid {
		return utils.NewError(utils.ErrCodeBadRequest, "Order has already been paid")
	}

	if order.OrderStatus != OrderStatusPending && order.OrderStatus != OrderStatusConfirmed {
		return utils.NewError(utils.ErrCodeBadRequest, "Order status does not allow payment")
	}

	if order.OrderPaymentMethod == OrderPaymentMethodCOD {
		return utils.NewError(utils.ErrCodeBadRequest, "COD orders cannot create online payment intent")
	}

	orderAmount := utils.NumericToFloat(order.OrderTotalAmount)
	if orderAmount <= 0 {
		return utils.NewError(utils.ErrCodeBadRequest, "Order amount is 0, online payment is not required")
	}

	if !order.OrderCreatedAt.IsZero() && time.Since(order.OrderCreatedAt) > 15*time.Minute {
		return utils.NewError(utils.ErrCodeBadRequest, "Order payment window (15 minutes) has expired. Please place a new order.")
	}

	return nil
}

func (s *paymentService) checkIdempotency(ctx context.Context, idempotencyKey string, order sqlc.GetOrderByUUIDRow) (*v1dto.PaymentIntentDTO, error) {
	redisKey := fmt.Sprintf("idem:payment:%s", idempotencyKey)
	var cachedDTO v1dto.PaymentIntentDTO
	if s.cache != nil {
		if err := s.cache.Get(redisKey, &cachedDTO); err == nil {
			if cachedDTO.OrderUUID != order.OrderUuid.String() {
				return nil, utils.NewError(utils.ErrCodeConflict, "Idempotency key already used for a different order")
			}
			return &cachedDTO, nil
		}
	}

	existingTxn, err := s.paymentRepo.GetByIdempotencyKey(ctx, nil, idempotencyKey)
	if err == nil {
		if existingTxn.PtOrderID != order.OrderID {
			return nil, utils.NewError(utils.ErrCodeConflict, "Idempotency key already used for a different order")
		}
		dto := v1dto.MapToPaymentIntentDTO(existingTxn, order.OrderUuid)
		if s.cache != nil {
			_ = s.cache.Set(redisKey, dto, 24*time.Hour)
		}
		return &dto, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to check idempotency key", err)
	}

	return nil, nil
}

func (s *paymentService) buildWebhookURL(providerName string) string {
	webhookBase := strings.TrimRight(s.webhookBaseURL, "/")
	if !strings.Contains(webhookBase, "/api/v1/webhooks/payment") {
		webhookBase = fmt.Sprintf("%s/api/v1/webhooks/payment", webhookBase)
	}
	return fmt.Sprintf("%s/%s", webhookBase, providerName)
}

func (s *paymentService) dispatchSuccessNotification(paidOrder sqlc.GetOrderByOrderNumberRow, providerTxnID string) {
	if s.notificationService != nil {
		go func() {
			_ = s.notificationService.Send(context.Background(), v1dto.SendNotificationInput{
				UserID: paidOrder.OrderUserID,
				Type:   "payment_success",
				Title:  "Payment Received",
				Body:   fmt.Sprintf("Payment for order #%s was successfully received.", paidOrder.OrderNumber),
				Data: map[string]any{
					"order_uuid":      paidOrder.OrderUuid.String(),
					"provider_txn_id": providerTxnID,
				},
			})
		}()
	}
}
